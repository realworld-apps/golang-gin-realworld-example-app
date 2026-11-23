package users

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/stretchr/testify/assert"
)

// REV-GO-002: a valid token (middleware optional) whose user no longer exists (BD) must not authenticate.
func TestAuthMiddlewareDeletedUserNotAuthenticated(t *testing.T) {
	asserts := assert.New(t)

	t.Setenv("JWT_SECRET", "auth-middleware-test-secret-32-chars!!")
	asserts.NoError(common.InitJWTSecret())

	var ctxUserID uint
	var ctxUserModel UserModel

	r := gin.New()
	r.Use(AuthMiddleware(false))
	r.GET("/test", func(c *gin.Context) {
		ctxUserID = c.MustGet("my_user_id").(uint)
		ctxUserModel = c.MustGet("my_user_model").(UserModel)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	resetDBWithMock()

	doGet := func() *httptest.ResponseRecorder {
		req, _ := http.NewRequest("GET", "/test", nil)
		common.HeaderTokenMock(req, 1)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// Control: while the user exists, the context is populated
	w := doGet()
	asserts.Equal(http.StatusOK, w.Code)
	asserts.Equal(uint(1), ctxUserID, "existing user should be authenticated")

	// Delete the user; the token is still cryptographically valid
	asserts.NoError(test_db.Unscoped().Delete(&UserModel{}, 1).Error)

	w = doGet()
	asserts.Equal(http.StatusOK, w.Code, "auto401=false should proceed as anonymous")
	asserts.Equal(uint(0), ctxUserID, "my_user_id should be 0 for a deleted user")
	asserts.Equal(UserModel{}, ctxUserModel, "my_user_model should be empty for a deleted user")
}

// REV-GO-002: with AuthMiddleware(true), PUT /user for a deleted user must
// return 401 and must not recreate a ghost record with the token's ID.
func TestAuthMiddlewareStrictRejectsDeletedUserAndPreventsGhostSave(t *testing.T) {
	asserts := assert.New(t)

	t.Setenv("JWT_SECRET", "auth-middleware-test-secret-32-chars!!")
	asserts.NoError(common.InitJWTSecret())

	r := gin.New()
	r.Use(AuthMiddleware(true))
	UserRegister(r.Group("/user"))
	resetDBWithMock()

	asserts.NoError(test_db.Unscoped().Delete(&UserModel{}, 1).Error)

	req, _ := http.NewRequest("PUT", "/user", bytes.NewBufferString(
		`{"user":{"username":"ghostuser"}}`))
	req.Header.Set("Content-Type", "application/json")
	common.HeaderTokenMock(req, 1)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	asserts.Equal(http.StatusUnauthorized, w.Code, "token of a deleted user should return 401")

	var count int64
	test_db.Model(&UserModel{}).Where("id = ?", 1).Count(&count)
	asserts.Zero(count, "no ghost record should be recreated with the token's ID")
}

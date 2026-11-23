package common

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Secreto que estaba hardcodeado en common/utils.go (REV-GO-001).
// Se usa solo para comprobar que ya no es aceptado.
const legacySecret = "A String Very Very Very Strong!!@##$!@#$" // #nosec G101

// Secreto de prueba válido (exactamente 32 caracteres).
const validTestSecret = "k9Vq2mX7pLr4TzB8nYc1WdE6hJf3UaGs" // #nosec G101

// signToken firma un token HS256 con el secreto indicado.
func signToken(t *testing.T, secret string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"id":  uint(7),
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return token
}

// 1) Validación al arrancar: JWT_SECRET debe existir y tener al menos 32 caracteres.
func TestInitJWTSecretValidation(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		unset   bool
		wantErr bool
	}{
		{name: "no definida", unset: true, wantErr: true},
		{name: "vacía", secret: "", wantErr: true},
		{name: "demasiado corta (31)", secret: strings.Repeat("a", 31), wantErr: true},
		{name: "válida (32)", secret: validTestSecret, wantErr: false},
		{name: "válida (64)", secret: strings.Repeat("b", 64), wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", tc.secret) // restaura el valor original al terminar
			if tc.unset {
				require.NoError(t, os.Unsetenv("JWT_SECRET"))
			}

			err := InitJWTSecret()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// 2) Firma y verificación exitosa con el secreto configurado en el entorno.
func TestGenTokenAndVerifyWithConfiguredSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", validTestSecret)
	require.NoError(t, InitJWTSecret())

	token := GenToken(42)
	require.NotEmpty(t, token)

	claims, err := VerifyTokenClaims(token)
	require.NoError(t, err)
	assert.Equal(t, float64(42), claims["id"])

	// La firma debe corresponder a JWT_SECRET, no a una constante interna.
	_, err = jwt.Parse(token, func(*jwt.Token) (interface{}, error) {
		return []byte(validTestSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	assert.NoError(t, err)
}

// 3) Rechazo del secreto hardcodeado anterior y de claves distintas.
func TestVerifyTokenClaimsRejectsInvalidSecrets(t *testing.T) {
	t.Setenv("JWT_SECRET", validTestSecret)
	require.NoError(t, InitJWTSecret())

	// Control positivo: el rechazo posterior no puede deberse a otro motivo.
	_, err := VerifyTokenClaims(signToken(t, validTestSecret))
	require.NoError(t, err)

	tests := []struct {
		name   string
		secret string
	}{
		{name: "secreto hardcodeado anterior", secret: legacySecret},
		{name: "clave distinta", secret: "otra-clave-completamente-diferente-123"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := VerifyTokenClaims(signToken(t, tc.secret))
			assert.Error(t, err)
		})
	}
}

# Hallazgos de auditoría — `golang-gin-realworld-example-app`

**Repositorio:** [github.com/gothinkster/golang-gin-realworld-example-app](https://github.com/gothinkster/golang-gin-realworld-example-app)
**SHA revisado:** `a41289b2a4951e7cc09066f86742a27f85121c9b`

## Resumen

| ID | Hallazgo | Severidad | Archivo | Esfuerzo | ¿Bloquea el merge? |
|---|---|---|---|---|---|
| [REV-GO-001](#rev-go-001--secreto-jwt-hardcodeado-en-el-código-fuente) | Secreto JWT hardcodeado en el código fuente | 🔴 Crítica | `common/utils.go` | S | Sí |
| [REV-GO-002](#rev-go-002--token-válido-de-un-usuario-inexistente-queda-autenticado) | Token válido de un usuario inexistente queda autenticado | 🔴 Crítica | `users/middlewares.go` | S | Sí |
| [REV-GO-003](#rev-go-003--pánico-por-nil-en-init-y-testdbinit) | Pánico por nil en `Init()` y `TestDBInit()` | 🔴 Crítica | `common/database.go` | S | Sí |

---

## REV-GO-001 — Secreto JWT hardcodeado en el código fuente

| Campo | Contenido |
|---|---|
| **ID** | REV-GO-001 |
| **Severidad** | Crítica |
| **Archivo** | `common/utils.go` |
| **Línea** | 43 (es la constante `JWTSecret` en `common/utils.go`, y también se usa al firmar en `GenToken` alrededor de la línea 51) |
| **Esfuerzo (S/M/L)** | S |
| **¿Bloquea el merge?** | Sí |

### Evidencia

```go
const JWTSecret = "A String Very Very Very Strong!!@##$!@#$" // #nosec G101
```

Además, el secreto se usa directamente en:

- `GenToken` de `common/utils.go`
- `AuthMiddleware` de `users/middlewares.go`
- `VerifyTokenClaims` de `common/test_helpers.go`

### Impacto en producción

Es como dejar la llave maestra pegada en la puerta y usar la misma en todos lados: cualquiera que vea el código puede hacerse pasar por cualquier usuario, acceder a datos privados y causar un incidente grave de seguridad, pérdida de confianza y problemas legales.

### Recomendación

- Quitar el secreto hardcodeado y el `#nosec`.
- Leer `JWT_SECRET` desde variable de entorno al arrancar y abortar el inicio si falta o tiene menos de 32 caracteres.
- Generar una clave aleatoria nueva en el entorno/secret manager y rotar la clave.

### ¿Por qué bloquea el merge?

Es aceptable como atajo de arranque rápido en un repo de ejemplo local, pero inaceptable en producción.

---

## REV-GO-002 — Token válido de un usuario inexistente queda autenticado

| Campo | Contenido |
|---|---|
| **ID** | REV-GO-002 |
| **Severidad** | Crítica |
| **Archivo** | `users/middlewares.go` |
| **Línea** | 26–33 (`UpdateContextUserModel`) y 68–72 (`AuthMiddleware`) |
| **Esfuerzo (S/M/L)** | S |
| **¿Bloquea el merge?** | Sí |

### Evidencia

`db.First(&myUserModel, my_user_id)` se ejecuta sin comprobar el error en `UpdateContextUserModel`, dejando `my_user_id` asignado en el contexto con un `UserModel{}` vacío si el usuario ya no existe.

### Impacto en producción

Una cuenta ya eliminada o revocada podría seguir operando en la plataforma o incluso reaparecer en la base de datos si el token aún no ha caducado. Esto compromete la confianza en la gestión de cuentas, puede generar datos fantasma o accesos indebidos y afectar el cumplimiento de bajas de usuario.

### Recomendación

1. Comprobar el error devuelto por `db.First(&myUserModel, my_user_id)` dentro de `UpdateContextUserModel` (y distinguir `gorm.ErrRecordNotFound` de un error de base de datos).
2. Si el usuario no existe (o falla la consulta), resetear el contexto a usuario anónimo (`my_user_id = 0` y `UserModel{}`) en lugar de dejar un `my_user_id` huérfano con un modelo vacío.
3. En `AuthMiddleware`, si no se logra cargar un usuario válido y `auto401` es `true`, abortar la petición con `401 Unauthorized` y evitar que `UserUpdate` ejecute `Save` sobre un usuario inexistente.

### ¿Por qué bloquea el merge?

Permite que peticiones con un token de un usuario borrado avancen autenticadas o disparen escrituras sobre registros inexistentes, lo que es inaceptable en producción.

---

## REV-GO-003 — Pánico por nil en `Init()` y `TestDBInit()`

| Campo | Contenido |
|---|---|
| **ID** | REV-GO-003 |
| **Severidad** | Crítica |
| **Archivo** | `common/database.go` |
| **Línea** | 54–71 (`Init`) y 74–92 (`TestDBInit`) |
| **Esfuerzo (S/M/L)** | S |
| **¿Bloquea el merge?** | Sí |

### Evidencia

`db, err := gorm.Open(...)` imprime el error con `fmt.Println` pero no retorna error, y continúa llamando `db.DB()` sobre un `*gorm.DB` potencialmente nil.

### Impacto en producción

Si la base de datos falla al abrirse o no está disponible, la aplicación puede desplomarse de golpe en lugar de fallar de forma controlada o reportar el problema limpiamente. En producción esto significa caídas del servicio, interrupciones para los usuarios y diagnósticos más difíciles ante incidentes de infraestructura.

### Recomendación

1. Cambiar la firma de `Init()` y `TestDBInit()` para que retornen `(*gorm.DB, error)` y devolver el error de inmediato si `gorm.Open` (o `db.DB()`) falla, sin continuar usando `db` en nil.
2. En `hello.go` (y en los callers/pruebas correspondientes), comprobar el error devuelto por `common.Init()` y abortar el arranque con un mensaje de error claro si la conexión inicial falla.
3. Actualizar `TestDBInit()` y sus pruebas asociadas para manejar el error explícitamente y evitar pánicos en entornos de test.

### ¿Por qué bloquea el merge?

Si la base de datos falla al abrirse, el proceso sigue y provoca un nil pointer dereference (caída abrupta) en lugar de fallar limpiamente; en un repo de ejemplo local puede tolerarse, pero en producción es inaceptable.

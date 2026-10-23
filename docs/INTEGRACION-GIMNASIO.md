# Alta de empleados en el gimnasio — 22 de octubre de 2026

## Contrato y alcance implementado

Car Cruds consume el POST publicado por Socios y Membresías del gimnasio. El contrato 1.0.1 se obtuvo del repositorio proporcionado por el usuario, revisión `eb77f9536c28fcef587ad0c83d84102cf485e1fc` de `main` (mismo árbol que `f74f41b` de `entrega-2`). Se conserva una [copia normalizada a JSON](contracts/providers/gym-openapi-1.0.1.json) del [OpenAPI del proveedor](https://github.com/PabloMarquezCapilla/Gestion-gimnasio-arquitectura-software/blob/eb77f9536c28fcef587ad0c83d84102cf485e1fc/docs/contracts/openapi-empleados-v1.yaml). No se modificó el repositorio del gimnasio.

La dirección efectiva es **Car Cruds envía empleados al gimnasio**, diferente de la propuesta inicial donde el gimnasio consultaba nuestra lista. `GET /v1/employees` se conserva compatible, sin afirmar que el gimnasio lo consuma. El POST crea un socio y una solicitud pendiente: no activa una membresía, no confirma un pago ni devuelve uso o gastos. El hospital y esos informes siguen pendientes de sus contratos.

```mermaid
sequenceDiagram
    participant A as Administrador
    participant E as Empleados (por gateway propio)
    participant DB as PostgreSQL de Empleados
    participant G as Gateway del gimnasio
    A->>E: POST empleado/{id}/gym-enrollments + clave local + DNI/email
    E->>DB: Guardar intención y snapshot; reclamar envío
    E->>G: POST /api/v1/integraciones/empleados + clave estable
    G-->>E: 201, socioId, solicitudId, PENDIENTE, false
    E->>DB: Guardar resultado, sin transacción abierta durante HTTP
    E-->>A: ACCEPTED (solicitud pendiente, membresía no habilitada)
```

## Configuración

Configurar únicamente en `.env` de Car Cruds:

```dotenv
GYM_BASE_URL=https://<host-del-gimnasio>
GYM_API_KEY=<credencial-privada-entregada-por-el-gimnasio>
GYM_ALLOW_HTTP=false
```

Ambas variables vacías deshabilitan los envíos; configurar solo una o un origen inválido impide iniciar Empleados. La base es el origen de su gateway, sin ruta ni parámetros. HTTPS es obligatorio fuera de loopback salvo `GYM_ALLOW_HTTP=true`, reservado para un laboratorio en LAN/Docker. Para alcanzar un servidor en el host desde Docker Desktop se puede usar `http://host.docker.internal:<puerto>` con esa excepción explícita. No usar `localhost` del contenedor para dirigirse al host.

La clave externa viaja únicamente en `X-API-Key`. No se reutilizan ni envían nuestros Bearer tokens al proveedor. La URL y la clave reales no fueron suministradas; permanecen sin configurar. Con cambios en `.env`, recrear Empleados mediante `docker compose up -d --build employees`.

## Operaciones propias

Solo `admin` y `service` pueden ejecutar o consultar estos trámites. `gym` y `client` reciben 403.

- `POST /v1/employees/{id}/gym-enrollments`, con Bearer propio e `Idempotency-Key` de 1 a 128 caracteres. Cuerpo: `{"dni":"99900001","email":"persona.ficticia@example.invalid"}`. Los valores del ejemplo son ficticios. El nombre y el identificador se toman del empleado existente y activo; ambos deben caber en los límites de 100 caracteres del proveedor. DNI y email se normalizan y se conservan en el trámite privado, no en la lista compartida ni en logs.
- `GET /v1/employees/{id}/gym-enrollments/{enrollmentId}` recupera el estado durable sin llamar al proveedor. El POST devuelve esta dirección en `Location`.

| Estado local | HTTP del POST | Interpretación y acción |
| --- | --- | --- |
| `PENDING` | 202 | Intención durable; un envío está en curso o su lease aún no venció. Consultar estado/repetir con la misma clave. |
| `UNKNOWN` | 503 | Resultado no confirmado por timeout, desconexión, autenticación externa o respuesta inválida. No equivale a rechazo del alta. Reintentar el mismo cuerpo y clave con la misma aplicación. |
| `ACCEPTED` | 201 | Proveedor confirmó socio y solicitud `PENDIENTE`; `membershipEnabled=false`. Repetir devuelve el resultado guardado, sin reenviar. |
| `REJECTED` | 409 | Rechazo explícito y procesable del proveedor; revisar `code`. Corregir datos y usar una nueva clave solo después de resolver el conflicto. |

Reutilizar una clave local con otro empleado/DNI/email devuelve 409. Una intención nueva para un empleado con trámite pendiente, desconocido o aceptado también devuelve 409. No se intenta eludir conflictos del proveedor con claves nuevas.

Ejemplo desde PowerShell 7, con nuestro entorno iniciado y la conexión al gimnasio configurada para una prueba acordada con datos ficticios:

```powershell
$admin = (Select-String -LiteralPath .env -Pattern '^AUTH_ADMIN_TOKEN=').Line.Split('=', 2)[1]
$headers = @{ Authorization = "Bearer $admin"; 'Idempotency-Key' = 'gym-alta-demo-001' }
$body = @{ dni = '99900001'; email = 'persona.ficticia@example.invalid' } | ConvertTo-Json -Compress
$response = Invoke-WebRequest -Method Post `
  -Uri 'http://127.0.0.1:8080/v1/employees/employee-001/gym-enrollments' `
  -Headers $headers -ContentType 'application/json' -Body $body -SkipHttpErrorCheck
$response.StatusCode
$response.Content
```

`employee-001` es el empleado ficticio que genera `SEED_DEMO`; usar el identificador real autorizado cuando corresponda y el puerto propio configurado. Con `UNKNOWN`, conservar cuerpo y clave del ejemplo para repetir la misma intención; el GET de la dirección `Location` permite consultar sin enviar nuevamente. El token del ejemplo se captura en memoria y no se muestra; no compartir `.env` con el gimnasio.

## Persistencia, reintentos y límites

La migración `002_gym.sql` conserva clave local por aplicación, huella, snapshot de datos, estado y referencias externas. El nombre queda congelado al crear el trámite: cambios de perfil o una baja posterior no alteran un reintento. Nuevas intenciones solo admiten empleados activos; los candidatos no se envían.

Antes de HTTP se confirma la intención. Un lease de 15 segundos permite un solo envío local simultáneo y se recupera tras caída del proceso. Un token de lease impide que una finalización atrasada sobrescriba otro intento. HTTP tiene un presupuesto de 2 segundos y no conserva conexiones/transacciones PostgreSQL. La finalización usa un contexto acotado independiente de la desconexión del llamador. Si no pudo persistirse el resultado, consultar/repetir el trámite original tras vencer el lease.

El encabezado externo `Idempotency-Key` se deriva del identificador durable del trámite (`carcruds-<enrollmentId>`). Se reutiliza exactamente el mismo JSON y clave al reintentar, incluso después de reiniciar Empleados. No hay un proceso que envíe automáticamente todos los empleados ni un bucle de reintentos: cada intento es una acción administrativa explícita. No cambiar origen/identidad del proveedor para recuperar una operación desconocida: debe resolverse contra la misma instalación y grupo del alta original.

Se rechazan redirecciones para no reenviar datos o credenciales a otro destino, respuestas fuera de contrato, cuerpos de más de 64 KiB y respuestas marcadas `X-Mock`. Solo una respuesta 201 válida con `estadoSolicitud=PENDIENTE` y `membresiaHabilitada=false` confirma el alta local. Los mensajes y cuerpos de error externos no se registran ni se devuelven; se conservan códigos controlados. La retención/limpieza de estos datos privados debe definirse operativamente antes de una publicación con datos reales; borrar una intención desconocida pierde su referencia de recuperación.

## Verificación y estado real

Las pruebas de consumidor verifican ruta, encabezados, mapeo de cuatro campos, contrato versionado, errores, timeout, redirecciones y respuestas inválidas. Las pruebas PostgreSQL cubren intención anterior a HTTP, pérdida de respuesta y replay tras recrear el servicio, snapshot estable, permisos, proyección sin DNI/email, claves concurrentes, leases y recuperación del emisor caído. El proveedor HTTP de estas pruebas es simulado; no representa una prueba conjunta con el gimnasio real.

```powershell
go test -race -count=1 ./services/employees/... ./cmd/employees
go vet ./services/employees/... ./cmd/employees
```

Las pruebas PostgreSQL necesitan `TEST_DATABASE_URL`; una omisión no acredita integración. La CI existente configura la dependencia real. Para cerrar la comunicación entre grupos faltan URL accesible, credencial/identidad asignadas por el gimnasio y prueba conjunta de alta/repetición/conflicto. El código implementado permite realizarla sin modificar el sistema del otro grupo.

### Resultado ejecutado el 22 de octubre

La [CI 39879790835](https://github.com/Gerpeirone/Car-Cruds/actions/runs/39879790835), sobre `aae1f8e9806358075191f1db1bba4395f7330c11`, aprobó ambos trabajos: **114 pruebas/subpruebas con dependencias reales, cero omitidas**, análisis estático, validación de configuración/Caddy y verificador de la lista. Las nuevas regresiones `TestPostgresGymLostResponseDurableReplayAndPrivateHTTPProjection` y `TestPostgresGymSerializesKeysLeasesAndEmployeeIntentions` aprobaron contra PostgreSQL. El recorrido `smoke -history` aprobó 76 solicitudes y su limpieza (`ef255325b3704cc7978979449d802ad3`).

La ejecución local previa con detector de carreras aprobó 99 pruebas/subpruebas y omitió 15 integraciones por ausencia de `TEST_*`; esos resultados se conservan separados de la CI. Los tests del gimnasio simulan HTTP y verifican el contrato fijado: **no se realizó un alta en el servidor del otro grupo ni se validaron credenciales reales**. [Resumen reproducible](evidence/gym-integration-2026-10-22.json).

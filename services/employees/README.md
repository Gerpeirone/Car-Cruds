# Servicio Empleados

| Campo | Valor |
| --- | --- |
| Fecha de alcance | 23 de octubre de 2026 |
| Estado | Implementación propia con PostgreSQL; integración externa pendiente de aceptación |
| Entrada ejecutable | [cmd/employees](../../cmd/employees/main.go) |
| Contrato de personal y gimnasio | [OpenAPI 3.0.3](../../docs/contracts/employees-openapi.json), versión 0.3.0 |
| Contrato administrativo de turnos | [OpenAPI 3.0.3](../../docs/contracts/clinic-integration-openapi.json), versión 0.3.0 |
| Decisiones vigentes | [ADR-018 — Empleados sin candidatos](../../docs/adr/ADR-018-empleados-sin-candidatos.md) y [ADR-019 — Turnos para socios](../../docs/adr/ADR-019-consumo-turnos-socios.md) |

Empleados administra el registro laboral de la concesionaria, la evidencia de membresías del gimnasio y el seguimiento administrativo de turnos médicos. El dominio activo contiene únicamente empleados; la gestión de candidatos fue retirada. La lista compartida con el gimnasio mantiene exclusivamente `employeeId` y `displayName` de empleados activos, sin cambios en su formato ni en el intercambio de membresías.

## Arquitectura y persistencia

- `domain/`: modelos y validaciones, sin HTTP, SQL ni bibliotecas de infraestructura.
- `application/`: casos de uso y puertos de repositorio, clínica y evidencia de membresías.
- `adapters/httpapi/`: autorización por operación, entrada JSON y proyecciones de respuesta.
- `adapters/postgres/`: persistencia propia, migraciones SQL embebidas y datos ficticios optativos.
- `adapters/clinic/`: cliente HTTP de Turnos para socios 1.0.0, revisión `c0b65f4`; autenticación, clave idempotente, errores operativos y consultas de evidencia.
- `cmd/employees/`: ensamblado por inyección explícita y servidor de la plataforma compartida.

El servicio conserva base y credenciales PostgreSQL propias; no consulta bases ajenas. Sus datos activos incluyen `employees`, evidencia de membresías, vínculos de pacientes, intenciones de turno e historia. La migración de inicio usa una transacción, registro de versión y bloqueo para coordinar arranques concurrentes.

Las migraciones históricas y la tabla `candidates` permanecen para conservar instalaciones y referencias previas. Sus filas, vínculos y solicitudes no se borran ni se convierten automáticamente en empleados. La API activa no permite acceder a esos registros ni operarlos; un paciente ya vinculado a un registro histórico conserva esa vinculación y requiere revisión administrativa antes de cualquier reasignación futura. No se agregan candidatos a la siembra.

Cada página utiliza `READ ONLY REPEATABLE READ`: conteo y registros pertenecen a la misma instantánea, ordenados por identificador. `page` comienza en 1 (predeterminado 1); `pageSize` admite 1–100 (predeterminado 20). Se rechaza un desplazamiento que no quepa en 64 bits. Entre páginas los datos pueden cambiar; una ausencia no acredita una baja laboral.

Los nombres administrativos admiten de 1 a 150 caracteres Unicode, se normalizan quitando espacios de los extremos y rechazan controles. Las altas y actualizaciones requieren `active` explícito. La baja es lógica e idempotente: conserva historia y no cancela turnos ni membresías automáticamente.

## Operaciones y permisos

Las operaciones usan autenticación Bearer de aplicación. El rol procede del middleware; un encabezado del consumidor no concede privilegios.

| Ruta | Métodos | Roles y comportamiento |
| --- | --- | --- |
| `/v1/employees` | GET | `gym`, `service`, `admin`: lista activa mínima |
| `/v1/employees` | POST | `admin`: alta con `displayName` y `active` |
| `/v1/admin/employees` | GET | `admin`: lista con activos e inactivos |
| `/v1/employees/{id}` | GET / PUT / DELETE | `admin`: consulta, reemplazo y baja lógica |
| `/v1/employees/membership-evidence` | POST | `gym`: comunicar evidencia de membresía, uso y gastos |
| `/v1/employees/reports/memberships` | GET | `admin`, `service`: informe por período y moneda |
| `/v1/employees/clinic/patient-links` | POST | `admin`: vínculo explícito con paciente verificado |
| `/v1/employees/clinic/requests` | POST | `admin`: registrar intención durable |
| `/v1/employees/clinic/requests/{requestId}` | GET | `admin`: consultar intención local |
| `/v1/employees/clinic/requests/{requestId}/submit` | POST | `admin`: envío o recuperación explícita de la misma intención, con espera durable |
| `/v1/employees/clinic/requests/{requestId}/refresh` | POST | `admin`: consultar o conciliar evidencia del hospital |
| `/v1/employees/reports/clinic` | GET | `admin`, `service`: informe de empleados |
| `/v1/employees/integrations` | GET | `admin`, `service`: estado general `503 NOT_CONFIGURED`, sin acreditar aceptación externa |

Las rutas `/v1/candidates` y `/v1/candidates/{id}` se retiraron; con autenticación válida devuelven `404`, también para Administración. El middleware conserva el rechazo `401` de solicitudes sin autenticación. El rol `client` no accede a personal. El gimnasio no administra empleados ni consulta detalles administrativos o clínicos.

Los cuerpos requieren `Content-Type: application/json`; campos desconocidos, múltiples objetos y cuerpos de más de 1 MiB se rechazan. Los contratos describen API propias y no acreditan acuerdo bilateral ni despliegue público.

## Elegibilidad para turnos

Nuevos vínculos, intenciones y cualquier envío que pueda crear turno, incluido el reintento incierto, exigen empleado existente y activo. Un empleado inexistente devuelve `404`; un inactivo, `409 EMPLOYEE_INACTIVE`. La validación y la escritura local se coordinan en PostgreSQL para no admitir una baja concurrente entre ambos pasos. No se mantiene una transacción durante HTTP.

Se conservan `personKind` y `personId` en el formato clínico por compatibilidad, pero el único tipo admitido es `employee`. El informe puede consultarse sin selector; `personKind=employee` sigue siendo válido y `candidate` se rechaza. No incluye registros históricos de candidatos.

Una baja permite consultar/actualizar evidencia previa mediante `refresh`, sin cancelar automáticamente ni permitir otro POST incierto que pueda crear. Repetir la referencia administrativa recupera la intención original. Antes de HTTP se persisten procedencia `turnos-partner-v1` y clave igual a `requestId`. Cada acción explícita hace como máximo un intento; al vencer `nextAttemptAt` puede repetirse la misma clave/datos, sin trabajador automático. El reclamo y el enfriamiento mínimo duran diez segundos; 429 amplía la espera con `Retry-After`. Conflicto idempotente requiere revisión. Solicitudes terminales/con turno y solicitudes antiguas ya enviadas sin procedencia idempotente nunca se reenvían como POST.

## Configuración y arranque

| Variable | Uso |
| --- | --- |
| `DATABASE_URL` | Base exclusiva de Empleados; obligatoria |
| `AUTH_ADMIN_TOKEN` | Aplicación administrativa; obligatoria |
| `AUTH_SERVICE_TOKEN` | Aplicación interna; obligatoria |
| `AUTH_GYM_TOKEN` | Aplicación del gimnasio; obligatoria para este servicio |
| `AUTH_CLIENT_TOKEN` | Identidad de cliente optativa; sin permisos sobre personal |
| `HTTP_ADDR` | Dirección de escucha; predeterminado `:8080` |
| `SEED_DEMO` | `true` carga empleados ficticios; predeterminado `false` |
| `CLINIC_BASE_URL` | Base pública del hospital terminada en `/v1`; optativa |
| `CLINIC_API_KEY` | Clave privada de consumidor; requerida cuando se configura URL |
| `CLINIC_ALLOW_HTTP` | Permiso explícito de HTTP para laboratorio; predeterminado `false` |
| `CLINIC_TIMEOUT` | Entre 100 ms y 5 s; predeterminado 2 s |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Exportación de trazas de la plataforma compartida |

Las credenciales se suministran fuera de Git, deben ser diferentes y tener al menos 24 caracteres. El servidor requiere base accesible, migración válida y credenciales obligatorias. Sin URL clínica permite registrar vínculos/intenciones y leer informes, pero enviar o actualizar devuelve `503 NOT_CONFIGURED`.

Desde la raíz, con el entorno configurado:

```powershell
go run ./cmd/employees
```

La siembra incluye `employee-001` y `employee-002`, personas ficticias, sin modificar nombres ni estados existentes. Readiness, logs, métricas y trazas usan la plataforma compartida, con endpoints de infraestructura protegidos.

```powershell
$employeesBase = 'http://127.0.0.1:8080'
$gymHeaders = @{ Authorization = 'Bearer ' + $env:AUTH_GYM_TOKEN }
Invoke-RestMethod -Headers $gymHeaders -Uri "$employeesBase/v1/employees?page=1&pageSize=20"
```

En la distribución completa se accede mediante el gateway. Las guías de [gimnasio](../../docs/COMUNICACION-GIMNASIO.md) y [hospital](../../docs/COMUNICACION-HOSPITAL.md) documentan cuerpos, permisos, correcciones, incertidumbre y límites. Los acuerdos operativos se siguen en [INTEGRACIONES](../../docs/INTEGRACIONES.md).

## Verificación

```powershell
go test ./services/employees/... ./cmd/employees
```

Las pruebas del servicio cubren entradas, permisos, bajas y contratos propios. Las pruebas PostgreSQL requieren `TEST_DATABASE_URL`; sin ella se omiten explícitamente. Usan un esquema aislado para verificar migraciones, persistencia y paginación. Las verificaciones de esta revisión deben cubrir rechazo de candidatos, elegibilidad activa, conservación de registros históricos y seguimiento de inactivos.

```powershell
$env:TEST_DATABASE_URL = $env:DATABASE_URL
go test ./services/employees/adapters/postgres -count=1 -v
```

Los resultados anteriores conservan sus fechas y no acreditan esta revisión. La suite debe comprobar recuperación idempotente, elegibilidad, espera, historia y credenciales contra el gateway original fijado a `c0b65f4`; sus resultados se registran al ejecutarla. Una omisión no acredita integración real. Prueba conjunta, pacientes reales, URL/clave compartidas y condiciones económicas continúan pendientes. Esta adaptación conserva solicitud/consulta e informe, sin agregar catálogo, cancelación saliente, facturación, frontend ni despliegue.

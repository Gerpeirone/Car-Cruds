# Servicio Empleados

Desde el 22 de octubre, el servicio incorpora el [alta administrativa en el gimnasio](../../docs/INTEGRACION-GIMNASIO.md): `POST /v1/employees/{id}/gym-enrollments` con DNI/email y clave idempotente, y `GET /v1/employees/{id}/gym-enrollments/{enrollmentId}` para recuperar el estado. Solo admin/service; intención durable, snapshot privado y reintentos con la misma clave externa. Configurar `GYM_BASE_URL` y `GYM_API_KEY` para habilitarlo. Una aceptación significa solicitud pendiente, nunca membresía habilitada. La lista compartida mantiene sus dos campos originales. Las referencias posteriores a ausencia de adaptador de gimnasio describen el estado anterior; uso/gastos y hospital siguen pendientes.

| Campo | Valor |
| --- | --- |
| Fecha | 13 de octubre de 2026 |
| Estado | Implementación propia con PostgreSQL; integración externa pendiente |
| Entrada ejecutable | [cmd/employees](../../cmd/employees/main.go) |
| Contrato implementado | [OpenAPI 3.0.3](../../docs/contracts/employees-openapi.json), versión 0.1.0 |
| Acuerdo con el consumidor | Pendiente; población activa y operaciones son decisiones propias provisionales |

Empleados administra empleados y candidatos en poblaciones separadas. La capacidad compartida permite consultar **solo empleados activos**, mediante `employeeId` y `displayName`. Excluye candidatos, clientes, credenciales, costos e información clínica. Registrar un candidato no crea un empleado: el alta administrativa es explícita.

## Arquitectura y persistencia

- `domain/`: modelos y validaciones, sin HTTP, SQL ni bibliotecas de infraestructura.
- `application/`: casos de uso y puertos propios de repositorio, clínica y evidencia de membresías.
- `adapters/httpapi/`: autorización por operación, entrada JSON y proyecciones de respuesta.
- `adapters/postgres/`: repositorio, migraciones SQL embebidas y datos ficticios optativos.
- `cmd/employees/`: ensamblado por inyección explícita y servidor de la plataforma compartida.

Empleados es dueño exclusivo de las tablas `employees` y `candidates`, dentro de su propia base y credenciales PostgreSQL; no consulta bases ajenas. La migración de inicio usa una transacción, registro de versión y bloqueo para coordinar arranques concurrentes.

Cada página utiliza `READ ONLY REPEATABLE READ`: conteo y registros pertenecen al mismo snapshot, ordenados por identificador. `page` comienza en 1 (predeterminado 1); `pageSize` admite 1–100 (predeterminado 20). Se rechaza un desplazamiento que no quepa en 64 bits. Entre solicitudes de páginas diferentes pueden cambiar los datos; no debe inferirse una baja por ausencia en una página.

Los nombres administrativos admiten de 1 a 150 caracteres Unicode, se normalizan quitando espacios de los extremos y rechazan controles. Las altas y actualizaciones de empleados requieren `active` explícito. La baja del empleado es lógica e idempotente. La eliminación de candidatos borra solo su registro local, sin operaciones externas.

## Operaciones y permisos

Todas las operaciones usan autenticación Bearer de aplicación. La identidad y el rol proceden del middleware; un encabezado del consumidor no concede privilegios.

| Ruta | Métodos | Roles y comportamiento |
| --- | --- | --- |
| `/v1/employees` | GET | `gym`, `service`, `admin`: lista de activos con dos campos mínimos |
| `/v1/employees` | POST | `admin`: alta con `displayName` y `active` |
| `/v1/admin/employees` | GET | `admin`: lista con activos e inactivos |
| `/v1/employees/{id}` | GET / PUT / DELETE | `admin`: consulta, reemplazo y baja lógica |
| `/v1/candidates` | GET / POST | `admin`: lista y alta de candidatos |
| `/v1/candidates/{id}` | GET / PUT / DELETE | `admin`: consulta, actualización y eliminación |
| `/v1/employees/integrations` | GET | `admin`, `service`: `503 NOT_CONFIGURED` |

El rol `client` no accede a personal. El gimnasio no escribe registros ni consulta candidatos, detalles administrativos o integración clínica. Los cuerpos requieren `Content-Type: application/json`; campos desconocidos, múltiples objetos y cuerpos de más de 1 MiB se rechazan. El contrato describe la API propia implementada, sin afirmar un acuerdo externo ni una URL pública desplegada.

## Configuración y arranque

| Variable | Uso |
| --- | --- |
| `DATABASE_URL` | Base exclusiva de Empleados; obligatoria |
| `AUTH_ADMIN_TOKEN` | Aplicación administrativa; obligatoria |
| `AUTH_SERVICE_TOKEN` | Aplicación interna; obligatoria |
| `AUTH_GYM_TOKEN` | Aplicación lectora del gimnasio; obligatoria para este servicio |
| `AUTH_CLIENT_TOKEN` | Identidad de cliente optativa; sin permisos sobre personal |
| `HTTP_ADDR` | Dirección de escucha; predeterminado `:8080` |
| `SEED_DEMO` | `true` carga datos ficticios; predeterminado `false` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Exportación de trazas de la plataforma compartida |

Las credenciales se suministran fuera de Git, deben ser diferentes y tener al menos 24 caracteres. No hay token débil predeterminado. El servidor requiere base accesible, migración válida y credenciales obligatorias.

Desde la raíz, con el entorno configurado:

```powershell
go run ./cmd/employees
```

La siembra incluye `employee-001`, `employee-002` y `candidate-001`, este último excluido de la lista compartida. Son personas ficticias. La siembra no modifica nombres ni estados de filas existentes. Readiness, logs, métricas y trazas usan la plataforma compartida, con endpoints de infraestructura protegidos.

Con el proceso local escuchando en `:8080`, la consulta del consumidor usa su credencial de entorno:

```powershell
$employeesBase = 'http://127.0.0.1:8080'
$gymHeaders = @{ Authorization = 'Bearer ' + $env:AUTH_GYM_TOKEN }
Invoke-RestMethod -Headers $gymHeaders -Uri "$employeesBase/v1/employees?page=1&pageSize=20"
```

El contrato contiene ejemplos ficticios de solicitud y respuesta. En la distribución completa, el acceso externo se realiza mediante el gateway y su dirección documentada en la raíz del proyecto.

## Contratos externos y límites

El gimnasio del grupo de Pablo publicó su contrato POST 1.0.1 de recepción de empleados, ya implementado por el adaptador `adapters/gym`. Faltan URL/credencial y prueba conjunta; no se conoce el contrato de devolución de membresías/uso/costos. El hospital de Octi debe publicar su contrato de consultas y turnos. **No existen adaptadores del hospital ni de los informes de uso/gastos**. Sus puertos conservan la distinción entre evidencia desconocida y valores confirmados.

`GET /v1/employees/integrations` informa que ambas integraciones siguen pendientes. No fabrica turnos, afiliación negativa, importes cero ni diagnósticos. Los acuerdos se siguen en [INTEGRACIONES.md](../../docs/INTEGRACIONES.md). El [borrador anterior](../../docs/contracts/employees-openapi-draft.json) se conserva como antecedente; el contrato de reservas sigue separado.

## Verificación

```powershell
go test ./services/employees/... ./cmd/employees
```

Las pruebas unitarias verifican separación de poblaciones, permisos, bajas, entradas inválidas y la proyección frente al contrato propio. Las pruebas PostgreSQL requieren `TEST_DATABASE_URL`; sin ella se omiten explícitamente. Usan un esquema aislado y comprueban persistencia tras reconexión, migraciones repetidas, siembra sin sobrescritura y paginación. Una escritura concurrente entre conteo y consulta comprueba el snapshot.

```powershell
$env:TEST_DATABASE_URL = $env:DATABASE_URL
go test ./services/employees/adapters/postgres -count=1 -v
```

Una prueba omitida no acredita integración real. El consumo por el gimnasio, la disponibilidad en nube y los intercambios externos se verificarán cuando existan entorno y contratos acordados.

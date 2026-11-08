# Referencia de endpoints de Car Cruds

**Fecha de revisión:** 28 de octubre de 2026. **Alcance:** 34 operaciones HTTP implementadas: 32 de negocio y 2 técnicas comunes. Esta referencia atiende el feedback posterior a la segunda entrega y describe el comportamiento vigente del código.

## 1. Consultar la documentación con Swagger

Desde la raíz del repositorio, con Go 1.26.1:

```powershell
go run ./cmd/apidocs
```

Abrir [Swagger UI local](http://127.0.0.1:8090/api/). El menú superior permite seleccionar Clientes, Flota, Alquileres, Empleados/gimnasio, integración clínica u operación técnica. Al desplegar una operación se muestran su finalidad, los roles autorizados, parámetros de ruta/consulta/encabezado, cuerpo JSON, campos obligatorios, modelos de respuesta, ejemplos y significado de los errores. El filtro permite localizar operaciones dentro de la API seleccionada.

El visor no necesita Docker, bases de datos, `.env` ni servicios activos. Los recursos de Swagger UI 5.34.0 están incluidos localmente, con licencia y procedencia en `docs/api/vendor/`; tampoco requiere una CDN ni envía los contratos a un validador remoto. `Ctrl+C` detiene el visor. Para cambiar el puerto: `go run ./cmd/apidocs -addr 127.0.0.1:8091`.

El visor es de lectura: no envía solicitudes de negocio ni requiere ingresar credenciales. Los contratos se pueden descargar desde las rutas `/contracts/<archivo>` del mismo visor e importar en herramientas compatibles con OpenAPI 3.0.3. Para ejecutar ejemplos se debe iniciar el backend según el [README](../README.md#ejecución-local) y utilizar el gateway en `http://127.0.0.1:8080`, con la credencial del rol requerido. Los servidores indicados en los contratos son direcciones locales de ejemplo, no una publicación externa.

La instalación y configuración del visor siguen la documentación oficial de [Swagger UI](https://swagger.io/docs/open-source-tools/swagger-ui/usage/installation/) y sus [opciones de configuración](https://swagger.io/docs/open-source-tools/swagger-ui/usage/configuration/).

## 2. Convenciones comunes

- **Autenticación:** `Authorization: Bearer <token>` en todas las operaciones del backend. Los tokens opacos identifican aplicaciones de laboratorio, no usuarios personales; no se presupone JWT. Se generan fuera de Git con `go run ./cmd/devsetup`.
- **Roles:** `admin` usa `AUTH_ADMIN_TOKEN`; `client`, `AUTH_CLIENT_TOKEN`; `gym`, `AUTH_GYM_TOKEN`; `service`, `AUTH_SERVICE_TOKEN`. El rol se resuelve desde el token. La credencial interna no se entrega al consumidor externo.
- **Propiedad:** `client` queda ligado a `AUTH_CLIENT_CUSTOMER_ID` para consultar su perfil y operar sobre sus reservas. Conocer un identificador no otorga acceso a un recurso ajeno.
- **JSON:** cuerpos con un solo objeto, sin campos desconocidos ni duplicados y hasta 1 MiB. Empleados y la creación de reservas verifican `Content-Type: application/json`; los otros comandos también documentan JSON, aunque no todos verifican explícitamente ese encabezado.
- **Fechas y dinero:** alquileres usan días calendario en `America/Buenos_Aires`, intervalo `[startsOn, endsOn)`, duración de 1 a 30 días e inicio desde hoy. Los importes `*Minor` son enteros de unidades menores; alquileres y flota usan centavos de ARS. Los importes externos dependen de la moneda y reglas acordadas con el productor.
- **Correlación:** se admite `X-Request-ID` de 1 a 80 caracteres de letras, números, punto, guion o guion bajo; si falta o es inválido se genera uno. La respuesta incluye `X-Request-ID` y `X-Service-Instance`. Se propaga contexto de traza HTTP.
- **Errores:** los handlers de negocio y autenticación devuelven JSON `{ "code": "CODIGO", "message": "Descripción" }`. Se decide por código HTTP y `code`, no por el texto de `message`. `401` indica token ausente/inválido; `403`, rol sin permiso; `404` puede ocultar un recurso ajeno. `503` expresa indisponibilidad o admisión agotada, no un resultado de negocio negativo. Algunas rutas o métodos inexistentes resueltos directamente por `net/http` devuelven `404/405` de texto plano. Go también admite HEAD en las rutas registradas como GET; el inventario cuenta las operaciones declaradas explícitamente.
- **Reintentos:** solo la creación de reservas exige `Idempotency-Key`. No se debe suponer idempotencia de cualquier POST. Las correcciones de gimnasio usan referencia/versión, y clínica conserva una intención local y su procedencia idempotente. Un timeout no permite afirmar que una escritura no ocurrió.

Todos los ejemplos usan personas, vehículos e identificadores ficticios. Sus valores monetarios ilustran el formato, no tarifas externas acordadas. Ajustar fechas y obtener identificadores reales del entorno antes de ejecutar una solicitud.

## 3. Clientes — 3 operaciones

Contrato: [customers-openapi.json](contracts/customers-openapi.json). Implementación: `services/customers/presentation/http.go`.

| Método y ruta | Finalidad y entradas | Resultado y permisos |
| --- | --- | --- |
| `GET /v1/customers` | Lista administrativa sin filtros ni paginación | `200`, objeto `items` con hasta 200 perfiles ordenados por `id`, incluidos inactivos. `admin`, `service` |
| `POST /v1/customers` | Alta con `id`, `name`, `active` explícito y `licenseValidUntil` | `201`, perfil registrado. `admin`. `400` por datos inválidos; `409 CONFLICT` por identificador existente |
| `GET /v1/customers/{id}` | Consulta de perfil por identificador | `200`, perfil con actividad y licencia. `admin`/`service` para cualquiera; `client` solo su perfil. Ajeno o inexistente: `404` |

El alta registra la licencia, sin garantizar que esté vigente: Alquileres valida la elegibilidad durante la reserva y el retiro. No hay endpoints implementados para modificar/eliminar clientes ni para leer su historial; la proyección de alquileres completados se actualiza por mensajería.

## 4. Flota — 7 operaciones

Contrato: [fleet-openapi.json](contracts/fleet-openapi.json). Implementación: `services/fleet/http.go`.

| Método y ruta | Finalidad y entradas | Resultado y permisos |
| --- | --- | --- |
| `GET /v1/vehicles` | Catálogo de MongoDB; `page`, `pageSize`, `brand`, `category`, `transmission`, `seats`, `sort`, `includeInactive` | `200`, página de vehículos. `admin`, `client`, `service`; solo `admin` incluye inactivos con el literal `includeInactive=true` |
| `GET /v1/vehicles/search` | Búsqueda Solr con `q`, paginación, filtros de dominio y orden por tarifa | `200`, página de activos; `503 SEARCH_UNAVAILABLE` si falla el índice. `admin`, `client`, `service` |
| `GET /v1/vehicles/{id}` | Ficha por identificador; consulta caché con fallback a MongoDB | `200`, vehículo y versión, incluso si está inactivo. `admin`, `client`, `service`; inexistente: `404 VEHICLE_NOT_FOUND` |
| `GET /v1/vehicles/{id}/quote` | Tarifa y actividad autoritativas, sin caché ni índice | `200`, `vehicleId`, `active`, `dailyRateMinor`, `currency`, `version`. Solo `admin`, `service` |
| `POST /v1/vehicles` | Alta con identidad, marca, modelo, categoría, transmisión, plazas, tarifa y moneda; `active` opcional | `201`, vehículo en versión 1. `admin`, `service`. Duplicado: `409 VERSION_CONFLICT` |
| `PUT /v1/vehicles/{id}` | Sustitución del catálogo con `version` vigente; `active=false` produce baja lógica | `200`, vehículo con versión incrementada. `admin`, `service`. Versión desactualizada: `409 VERSION_CONFLICT` |
| `POST /v1/vehicles/reindex` | Sin cuerpo; reencola el catálogo completo para el indexador | `202`, `status=pending` y `enqueued`; no significa índice actualizado. `admin`, `service` |

La paginación comienza en 1, usa 20 ítems por defecto y admite hasta 100. `page` admite hasta 100000. `seats` filtra por igualdad; 0 desactiva el filtro. El orden es `price_asc` o `price_desc`, con identificador ascendente para desempatar. `q` solo filtra en `/search`: el lector común lo acepta en el catálogo, pero MongoDB no lo aplica. La búsqueda siempre excluye inactivos y puede retrasarse respecto de la fuente.

En el alta `version` se omite o vale 0, y `active` omitido/null se interpreta como true. En PUT `vehicleId` puede omitirse y, si aparece, debe coincidir con la ruta; `active` omitido/null conserva el estado actual. Las escrituras mantienen evento outbox e intentan invalidar caché. `503 OUTBOX_FULL` indica que se alcanzó el límite de eventos pendientes; no corresponde anunciar éxito ni reconstrucción completa. No hay DELETE de vehículos.

## 5. Alquileres — 7 operaciones

Contrato: [rentals-openapi.json](contracts/rentals-openapi.json). Implementación: `services/rentals/adapters/httpapi/handler.go`.

| Método y ruta | Finalidad y entradas | Resultado y permisos |
| --- | --- | --- |
| `GET /v1/availability` | `vehicleId`, `startsOn`, `endsOn`, obligatorios y no repetidos; calcula disponibilidad y precio | `200`, disponibilidad, días, tarifa y total. `admin`, `client`, `service`. No bloquea el vehículo |
| `POST /v1/reservations` | JSON de vehículo, cliente, fechas y `expectedTotalMinor`; encabezado `Idempotency-Key` obligatorio | `201`, reserva `RESERVED`; misma clave/cuerpo recupera la respuesta persistida. `admin`, `client`, `service` con control de cliente |
| `GET /v1/reservations/{id}` | Consulta de reserva por identificador | `200`, reserva y precio congelado. `admin`/`service`; `client` para su recurso. Ajena para client: `404` |
| `DELETE /v1/reservations/{id}` | Cancela una reserva sin retiro; sin cuerpo | `200`, `CANCELLED`; repetir devuelve el mismo estado. Permisos del recurso; estado incompatible: `409 INVALID_STATE` |
| `POST /v1/reservations/{id}/pickup` | Registra retiro; vuelve a comprobar conductor y alquiler activo del auto | `200`, `ACTIVE`; repetición válida no duplica retiro. `admin`, `service` |
| `POST /v1/reservations/{id}/return` | Registra devolución y evento de alquiler completado; sin cuerpo | `200`, `COMPLETED`; repetir no duplica devolución ni evento. `admin`, `service` |
| `POST /v1/maintenance` | Bloqueo con `vehicleId`, `startsOn`, `endsOn`; comparte exclusión de agenda | `201`, bloqueo creado. `admin`, `service`. No admite encabezado de idempotencia ni consulta/eliminación HTTP del bloqueo |

La creación exige una clave de 1 a 128 caracteres ASCII visibles, sin espacios y en una sola aparición. Reutilizarla con otra solicitud devuelve `409 IDEMPOTENCY_KEY_REUSED`. El total esperado debe coincidir con la tarifa fuente; si cambió, `409 PRICE_CHANGED`. Un período ocupado produce `409 VEHICLE_UNAVAILABLE`; un conductor no habilitado produce `422 CUSTOMER_NOT_ELIGIBLE`. La disponibilidad puede devolver `available=false` con razón de negocio; una dependencia caída produce `503`.

El precio queda congelado al reservar. Llegar a `endsOn` no devuelve el auto: un alquiler `ACTIVE` sigue impidiendo otro retiro hasta registrar su devolución. La cancelación solo aplica antes del retiro. Consultar primero no garantiza confirmar después; la transacción vuelve a validar agenda y condiciones.

## 6. Empleados y gimnasio — 9 operaciones

Contrato: [employees-openapi.json](contracts/employees-openapi.json). Implementación: `services/employees/adapters/httpapi/handler.go` y `memberships.go`.

| Método y ruta | Finalidad y entradas | Resultado y permisos |
| --- | --- | --- |
| `GET /v1/employees` | Lista compartida de activos con `page` y `pageSize` | `200`, exclusivamente `employeeId`, `displayName` y metadatos de página. `admin`, `gym`, `service` |
| `POST /v1/employees` | Alta laboral explícita con `displayName` y `active` | `201`, registro completo y encabezado `Location`. Solo `admin` |
| `GET /v1/admin/employees` | Lista administrativa de activos e inactivos; paginada | `200`, ítems con `employeeId`, `displayName`, `active`. Solo `admin` |
| `GET /v1/employees/{id}` | Consulta administrativa individual | `200`, registro completo; `404` si no existe. Solo `admin` |
| `PUT /v1/employees/{id}` | Sustituye `displayName` y `active`, ambos obligatorios | `200`, registro actualizado. Solo `admin` |
| `DELETE /v1/employees/{id}` | Baja lógica repetible; conserva historia | `200`, registro con `active=false`. Solo `admin`; no cancela turnos ni membresías automáticamente |
| `GET /v1/employees/integrations` | Marcador general de acuerdos/acceso externos pendientes | Siempre `503 NOT_CONFIGURED` en esta versión. `admin`, `service`; no sustituye las rutas específicas implementadas |
| `POST /v1/employees/membership-evidence` | Recibe instantánea de membresía, uso y gastos identificada por empleado, período, referencia y versión | `200`, `{ "applied": true/false }`. Solo `gym`; conflicto de procedencia/versión: `409 EVIDENCE_CONFLICT` |
| `GET /v1/employees/reports/memberships` | Informe administrativo con `period` obligatorio, `page`, `pageSize` | `200`, evidencia por empleado, faltantes y totales por moneda para todo el período. `admin`, `service` |

Las listas admiten `page` desde 1 y `pageSize` de 1 a 100, con valores predeterminados 1/20; rechazan parámetros desconocidos o repetidos. Conteo e ítems pertenecen a la misma instantánea de una página, sin prometer una instantánea entre páginas. El nombre se normaliza y admite hasta 150 caracteres, sin controles.

La evidencia del gimnasio identifica al emisor desde su token. `employeeId`, `period`, `sourceReference`, `version` y `observedAt` son obligatorios. `member`, `used`, `costMinor` y `spendingMinor` omitidos/null significan desconocido; una corrección es una nueva instantánea completa con mayor versión, no un incremento monetario. Los importes conocidos requieren moneda. Misma versión/contenido no duplica gastos; una versión atrasada no reemplaza la vigente.

El informe incluye inactivos y suma cantidades conocidas por moneda, sin conversión. Sus totales y contadores abarcan el registro completo del período, no solo la página. Ausencia de evidencia no significa no uso ni costo cero. El contrato es propio y provisional; su existencia no acredita aceptación del gimnasio. Ver [COMUNICACION-GIMNASIO](COMUNICACION-GIMNASIO.md).

## 7. Integración clínica de Empleados — 6 operaciones

Contrato: [clinic-integration-openapi.json](contracts/clinic-integration-openapi.json). Implementación: `services/employees/adapters/httpapi/clinic.go`. Estas rutas son de Car Cruds: Empleados llama al hospital desde su adaptador; la API no publica la clave privada del proveedor.

| Método y ruta | Finalidad y entradas | Resultado y permisos |
| --- | --- | --- |
| `POST /v1/employees/clinic/patient-links` | Vincula `personKind=employee`, `personId` y `patientId` verificado | `204`, sin cuerpo. Solo `admin`; requiere empleado activo y no crea pacientes en el hospital |
| `POST /v1/employees/clinic/requests` | Guarda intención con empleado, `localReference`, `doctorId` y `startAt` con precisión de segundos | `200`, intención local y `Location`; no envía todavía. Solo `admin`; nueva intención requiere empleado activo |
| `GET /v1/employees/clinic/requests/{requestId}` | Lee estado local, referencia, fechas y última evidencia | `200`, intención; no llama al hospital. Solo `admin`; admite seguimiento de inactivos |
| `POST /v1/employees/clinic/requests/{requestId}/submit` | Sin cuerpo; envía o recupera la intención persistida respetando `nextAttemptAt` | `200`, estado administrativo resultante; puede seguir desconocido. Solo `admin`; toda acción que pueda crear exige empleado activo |
| `POST /v1/employees/clinic/requests/{requestId}/refresh` | Consulta turno conocido; cuerpo opcional con `appointmentId` para conciliación verificada | `200`, última evidencia local, que puede conservar incertidumbre. Solo `admin`; admite empleados inactivos |
| `GET /v1/employees/reports/clinic` | Informe paginado con `personKind=employee` opcional, `page`, `pageSize` | `200`, última intención por empleado, incluidos inactivos; no enumera todos sus turnos externos. `admin`, `service` |

Las cinco operaciones de administración individual rechazan parámetros de consulta. El informe admite únicamente su tipo y paginación, sin repeticiones. No existe API activa de candidatos.

El flujo es **vincular paciente → guardar intención → enviar → consultar/conciliar → informar**. Una referencia local estable recupera la intención correspondiente; reutilizarla con datos contradictorios produce conflicto. Para crear una nueva intención se requiere vínculo previo (`409 PATIENT_LINK_REQUIRED`); empleado inactivo produce `409 EMPLOYEE_INACTIVE` en las nuevas acciones correspondientes.

`200` no equivale a turno confirmado: se debe leer `state`, `classification`, `failureCode`, `lastKnownAppointment` y, si aparece, `nextAttemptAt`. Un fallo del proveedor que se logra persistir puede responder `200` con `state=unknown`. `confirmed` requiere evidencia válida; `pending_verification` conserva incertidumbre; `without_confirmed_appointment` refleja una cancelación/rechazo registrado de esa intención, sin certificar ausencia de cualquier turno del empleado ni aptitud/asistencia médica.

Cada acción explícita hace como máximo una llamada externa; no hay reintentos automáticos. El envío conserva clave y datos internos. Una solicitud terminal, con turno ya vinculado, conflicto idempotente o enviada con el contrato antiguo sin procedencia idempotente no genera otro POST. Una consulta externa `404` tampoco prueba ausencia: el proveedor puede ocultar turnos de otra aplicación. Ver [COMUNICACION-HOSPITAL](COMUNICACION-HOSPITAL.md) y [ADR-019](adr/ADR-019-consumo-turnos-socios.md).

## 8. Rutas técnicas — 2 operaciones comunes

Contrato: [operations-openapi.json](contracts/operations-openapi.json). Implementación: `internal/platform/runtime.go`.

| Método y ruta | Finalidad y entradas | Resultado y permisos |
| --- | --- | --- |
| `GET /internal/ready` | Comprobación de disponibilidad de la instancia consultada | `200`, `status`, `service`, `instance`; `503` si no verifica la dependencia. `admin`, `service` |
| `GET /internal/metrics` | Métricas Prometheus de la instancia; `Accept: text/plain` permite solicitar el formato textual | `200`, texto de métricas, no JSON. `admin`, `service` |

Estas rutas existen en gateway, Clientes, Alquileres, Empleados, cada réplica de Flota e indexador. En el gateway `/internal/ready` consulta en paralelo los cuatro servicios de negocio; `/internal/metrics` solo muestra métricas del gateway. Los servicios privados se consultan desde la red interna, sin publicar nuevos puertos. Caddy bloquea `/internal/*` en el despliegue público. El estado técnico no acredita acuerdos ni disponibilidad funcional de los proveedores externos.

## 9. Ejemplo de recorrido por el gateway

Con el entorno iniciado, una credencial `client` y un vehículo activo existente:

```powershell
$headers = @{ Authorization = "Bearer $env:AUTH_CLIENT_TOKEN" }
$gateway = 'http://127.0.0.1:8080'
$disponibilidad = Invoke-RestMethod "$gateway/v1/availability?vehicleId=auto-001&startsOn=2030-01-15&endsOn=2030-01-18" -Headers $headers
if (-not $disponibilidad.available) { throw 'El vehículo no está disponible para ese período' }
$solicitud = @{
  vehicleId = 'auto-001'
  customerId = 'cli-001'
  startsOn = '2030-01-15'
  endsOn = '2030-01-18'
  expectedTotalMinor = $disponibilidad.totalMinor
} | ConvertTo-Json
$headers['Idempotency-Key'] = 'reserva-demo-cli-001-2030-01-15-01'
$reserva = Invoke-RestMethod "$gateway/v1/reservations" -Method Post -Headers $headers -ContentType 'application/json' -Body $solicitud
Invoke-RestMethod "$gateway/v1/reservations/$($reserva.id)" -Headers $headers
```

Configurar la variable de entorno del token desde la credencial local autorizada; el ejemplo no carga ni imprime `.env`. El cliente debe coincidir con el configurado en el token. Los registros de ejemplo no se crean automáticamente salvo el seed expresamente habilitado. Si la respuesta se pierde, recuperar con la misma clave y cuerpo; una intención diferente requiere otra clave. Este ejemplo modifica datos al ejecutarse y no constituye evidencia de una prueba realizada.

## 10. Mantenimiento y verificación

Al cambiar una ruta, actualizar su contrato, esta referencia y el catálogo del visor si corresponde. Los contratos vigentes conservan su versión funcional; la ampliación documental no anuncia una nueva API ni un acuerdo externo. No se modifican `openapi-v1.json`, los borradores históricos ni las copias del proveedor.

Verificación formal y de ejemplos, en un entorno Python separado:

```powershell
python -m venv tmp/api-docs-venv
./tmp/api-docs-venv/Scripts/python.exe -m pip install -r docs/api/requirements.txt
./tmp/api-docs-venv/Scripts/python.exe scripts/check-api-docs.py
go test ./docs ./cmd/apidocs
```

En Linux: `python3 -m venv tmp/api-docs-venv` y utilizar `tmp/api-docs-venv/bin/python`. CI ejecuta la misma validación: estructura OpenAPI 3.0, referencias locales, ejemplos compatibles con esquemas, identificadores de operación únicos, correspondencia de métodos/rutas con los handlers y con esta guía, y hashes de los recursos de Swagger. La validación documental no sustituye pruebas con almacenamiento, integración externa ni aceptación de los consumidores.

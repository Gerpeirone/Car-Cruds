# Comunicación con el hospital de Octi

Implementación inicial de Car Cruds del 22 de octubre de 2026; alcance actualizado el 23 de octubre según [ADR-018](adr/ADR-018-empleados-sin-candidatos.md). Empleados solicita y consulta turnos únicamente de empleados; el gimnasio no recibe esta información. No se modifica el repositorio del hospital.

**Corte de compatibilidad:** el hospital publicó después el [contrato Turnos para socios 1.0.0](https://github.com/OctiDelFabro/portal-paciente-clinica/tree/c0b65f45867f884e0bdc8f7c23eb0f344f646bb2/docs/contracts). Esta tarea retira candidatos; no migra el cliente saliente. Las rutas, configuración, falta de autenticación e idempotencia descritas abajo corresponden exclusivamente a la revisión antigua fijada en 5e4c210. La siguiente etapa deberá adaptar URL, credencial, clave idempotente y recuperación, y actualizar la prueba de contrato; esta guía no acredita compatibilidad con socios 1.0.0.

## Referencia comprobada del cliente actual

Repositorio: [portal-paciente-clinica](https://github.com/OctiDelFabro/portal-paciente-clinica). La API de Turnos está en **`feature/entrega-2`**, commit **`5e4c2103e8c3b0ffaccd1e02a94d1dcf98f2f96d`**; `main` conserva una versión anterior con esqueletos de servicios. Se revisaron README, gateway, handler HTTP, aplicación, dominio y directorio de pacientes/médicos.

Operaciones consumidas, observadas en su código:

| Operación | Gateway del hospital | Resultado |
| --- | --- | --- |
| Solicitar turno | `POST /api/turnos/appointments` | 201 y turno con ID, paciente, médico, inicio/fin, estado y versión |
| Consultar turno | `GET /api/turnos/appointments/{id}` | 200 y turno actual; permite reflejar cancelaciones |

El cuerpo del POST contiene exclusivamente `patientId`, `doctorId` y `startAt` RFC3339. El hospital exige al menos 24 horas de anticipación, paciente habilitado y ausencia de superposiciones. Sus estados implementados son `booked` y `cancelled`. No existe una operación implementada de reprogramación en la revisión inspeccionada; si el proveedor cambia datos con una versión superior, Car Cruds conserva la nueva evidencia del mismo turno y paciente.

En esa revisión antigua, su OpenAPI público describía **Notificaciones**, no Turnos. Car Cruds **no sustituye una reserva por un correo** ni reutiliza `X-API-Key` o `Idempotency-Key` de Notificaciones. La API de Turnos de esa revisión no implementa autenticación del portal ni idempotencia de creación. La [especificación observada del consumidor](contracts/providers/octi-turnos-observed.openapi.json) documenta lo comprobado; no se presenta como contrato oficial aceptado por Octi. Deben confirmar exposición y autenticación antes de un despliegue compartido.

## Responsabilidades y dirección

1. Personal de Car Cruds identifica administrativamente que un empleado registrado y activo necesita consulta. No se almacena diagnóstico ni se infiere necesidad médica. La API de Octi inspeccionada todavía no ofrece una consulta/notificación de empleados que requieren atención.
2. Personal registra el vínculo entre el empleado activo y un `patientId` verificado del hospital. No se equiparan IDs ni se comparan nombres automáticamente.
3. Empleados guarda una intención durable con referencia local, persona, paciente, médico y horario solicitado.
4. Una acción explícita de envío reclama esa intención en PostgreSQL y confirma la marca **antes** de llamar directamente al hospital. No mantiene una transacción SQL abierta durante HTTP.
5. Una respuesta válida confirma el turno. La consulta posterior recupera cambios administrados por el hospital y alimenta el informe de personal.

La llamada sale del backend de Empleados hacia el proveedor; no del frontend ni a través de nuestro gateway. Las acciones de personal sí entran por nuestro gateway y exigen rol `admin`. Solo `admin` y `service` pueden leer el informe. El gimnasio y los clientes no pueden consultar datos de turnos.

## Configuración

En el `.env` de **Car Cruds**, fuera de Git:

```dotenv
CLINIC_BASE_URL=https://host-acordado-del-hospital/api/turnos
CLINIC_ALLOW_HTTP=false
CLINIC_TIMEOUT=2s
```

La base incluye el prefijo que corresponda al despliegue: se agregan `/appointments` y `/appointments/{id}`. Para acceso directo al servicio de Turnos del laboratorio, la base puede ser `http://host-de-pruebas:8081` sin prefijo. HTTP requiere `CLINIC_ALLOW_HTTP=true` explícito y se reserva al entorno de pruebas. En Docker, `localhost` refiere al contenedor de Empleados: usar una dirección alcanzable desde él, por ejemplo `host.docker.internal` si Turnos se ejecuta en la PC anfitriona. Los dos proyectos no deben intentar publicar el mismo puerto del host.

Sin URL el servicio propio inicia normalmente: permite vincular personas, registrar intenciones y consultar informes; enviar o actualizar devuelve 503 `NOT_CONFIGURED`. Una configuración de URL/timeout inválida impide iniciar Empleados. Timeout configurable entre 100 ms y 5 s; valor inicial 2 s. Máximo ocho llamadas clínicas simultáneas, sin cola ilimitada y sin seguir redirecciones. Se propagan trazas y `X-Correlation-ID`, sin reenviar el Bearer administrativo de Car Cruds.

## Uso de nuestra API

Contrato propio: [clinic-integration-openapi.json](contracts/clinic-integration-openapi.json), versión 0.2.0. Todas estas acciones usan `Authorization: Bearer <AUTH_ADMIN_TOKEN>`; no es una credencial del hospital.

### 1. Vincular un empleado activo con un paciente

`POST /v1/employees/clinic/patient-links`, `Content-Type: application/json`:

```json
{"personKind":"employee","personId":"employee-001","patientId":"pat-001"}
```

`personKind` admite únicamente `employee`; se conserva junto con `personId` para compatibilidad del formato. El empleado debe existir y estar activo; un inactivo devuelve `409 EMPLOYEE_INACTIVE`. El mismo vínculo puede repetirse mientras sea elegible; asociar el paciente a otra persona o cambiar silenciosamente un vínculo devuelve 409. Los vínculos históricos de candidatos se conservan sin acceso por la API, sin reasignar pacientes ni promover registros automáticamente.

**Los IDs de ejemplo son ficticios.** En el hospital inspeccionado solo existen pacientes semilla `pat-001`, `pat-002` (no habilitado) y `pat-003`, y médicos `doc-001`/`doc-002`. Para personas reales Octi debe confirmar/implementar su registro y habilitación; este enlace no crea pacientes ni los habilita en su sistema.

### 2. Registrar la necesidad/intención administrativa

`POST /v1/employees/clinic/requests`:

```json
{
  "personKind":"employee",
  "personId":"employee-001",
  "localReference":"revision-personal-001",
  "doctorId":"doc-001",
  "startAt":"2026-12-01T10:00:00Z"
}
```

La nueva intención exige empleado registrado y activo, además del vínculo previo; un inactivo devuelve `409 EMPLOYEE_INACTIVE`. Elegir un horario real acordado con el hospital, al menos 24 h en el futuro. No se inventa un catálogo de disponibilidad inexistente en su API. `startAt` usa precisión de segundos, conforme al formato observado del proveedor. La respuesta 200 incluye `requestId`, `state: ready` y `classification: pending_verification`. No se envía nada al hospital en este paso.

`localReference` es una referencia estable de la consulta administrativa. Repetirla con los mismos datos recupera la intención original; cambiar persona, médico o inicio con esa referencia devuelve 409. Una referencia distinta representa otra consulta: no debe utilizarse para eludir un resultado desconocido. La intención fija el paciente vinculado en ese momento y conserva quién la registró.

### 3. Enviar la intención una sola vez

El reclamo del primer envío vuelve a comprobar que el empleado esté activo. Si fue dado de baja después de registrar la intención, se rechaza con 409 EMPLOYEE_INACTIVE sin llamar al proveedor. Una solicitud ya reclamada conserva su resultado al repetirse, también si luego se dio de baja al empleado.

`POST /v1/employees/clinic/requests/{requestId}/submit`, **sin cuerpo**.

Si el hospital devuelve 201 válido para el paciente, médico y horario solicitados, se guarda `state: booked`, la referencia externa y la fecha de observación. Un rechazo explícito compatible —horario ocupado, anticipación insuficiente, paciente no habilitado o IDs desconocidos— se guarda como `rejected` con un código administrativo acotado.

Timeout, conexión perdida, 5xx o respuesta inválida dejan `state: unknown`. La respuesta propia 200 informa el estado de la intención; **no significa necesariamente turno confirmado**. Repetir `submit` devuelve el registro y no vuelve a ejecutar POST remoto, incluso después de reiniciar. Esto evita duplicar una operación para la cual el hospital no garantiza idempotencia.

Si el proceso cae después de reclamar y antes de enviar, también queda desconocido. Es una limitación deliberada: sin idempotencia o consulta por referencia local del proveedor, no es seguro distinguir ese caso de un turno creado cuya respuesta se perdió.

### 4. Consultar y conciliar

- `GET /v1/employees/clinic/requests/{requestId}` lee la intención local, sin llamar al hospital.
- `POST /v1/employees/clinic/requests/{requestId}/refresh`, sin cuerpo, consulta el turno ya vinculado por ID externo.
- Si se perdió la respuesta de creación, solicitar a Octi el ID del turno y enviar `{"appointmentId":"id-verificado-del-hospital"}` en `refresh`. Se verifica contra el proveedor que paciente, médico e inicio coincidan antes de vincularlo; no se permite “confirmar” manualmente un turno solo por escribir su ID.

La baja laboral conserva el historial y permite leer y actualizar evidencia anterior del empleado mediante `refresh`; no cancela turnos ni crea reservas automáticamente. La consulta remota no modifica turnos. Una cancelación válida se registra como `cancelled`; no crea una nueva reserva. Se conserva historia de versiones. Una versión atrasada no reemplaza otra nueva; misma versión con distinto contenido obtiene 409. Un error de consulta o 404 deja la verificación pendiente y conserva `lastKnownAppointment` y la fecha anterior; no prueba que la persona no tenga turno. Un fallo atrasado no sobrescribe una observación posterior.

## Informe administrativo

`GET /v1/employees/reports/clinic?page=1&pageSize=20`. Se admite `personKind=employee` por compatibilidad; `candidate` se rechaza con 400. El rol de servicio también puede leerlo.

Incluye todos los empleados, activos e inactivos, incluso aquellos sin vínculo ni intención, que aparecen pendientes. Excluye registros históricos de candidatos. Presenta la última intención administrativa de cada persona, con referencias, fecha y evidencia anterior disponible. No es una búsqueda global de todos los turnos de una persona en el hospital.

| Clasificación | Alcance |
| --- | --- |
| `confirmed` | El hospital confirmó el turno de la última intención consultada, con fecha de verificación |
| `without_confirmed_appointment` | La última intención fue rechazada explícitamente o su turno fue cancelado, según evidencia guardada |
| `pending_verification` | Falta intención/evidencia, no se envió todavía, hubo una respuesta perdida o falló la actualización |

La clasificación se limita a esa intención, no certifica que no existan otros turnos en el hospital. Una confirmación no acredita asistencia, aptitud médica ni contratación. La lectura del informe no efectúa consultas masivas externas; para actualizar se utiliza `refresh`. No se inventa un SLA de vigencia: siempre se muestra la fecha de la última comprobación.

## Pruebas y acuerdos pendientes

Pruebas propias: contrato HTTP observado, validación de respuestas, rechazo de redirecciones, timeouts, permisos, persistencia PostgreSQL, reclamo concurrente, reinicio, historia y precedencia. El escenario `integration/cliniccontract` ejecuta el código original de Turnos fijado por commit, con MySQL/RabbitMQ aislados, y Car Cruds con PostgreSQL real. Comprueba reserva, repetición local, consulta, cancelación administrada en el hospital y pérdida de una respuesta exitosa seguida de conciliación. No modifica archivos del proveedor ni usa su entorno real; no acredita aceptación bilateral, correo entregado ni despliegue público.

La prueba indicada utiliza la revisión antigua `5e4c210`; sus resultados no acreditan la retirada de candidatos ni el contrato de socios posterior. La migración a ese contrato publicado queda para la siguiente etapa, con autenticación, idempotencia y recuperación acordes a su semántica. Para integración compartida siguen pendientes URL, credencial, procedimiento de alta/vinculación/habilitación de pacientes reales, médicos de prueba, condiciones económicas y validación conjunta. La indicación de quién necesita consulta sigue pendiente de una capacidad real del hospital. Notificaciones es una capacidad distinta y no sustituye una reserva.

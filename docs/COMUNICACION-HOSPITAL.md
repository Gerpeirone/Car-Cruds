# Comunicación con el hospital de Octi

> **Verificación sin clave — 11/11/2026:** `https://api-turnos.srv1779429.hstgr.cloud/healthz` responde 200. `GET /v1/doctors`, `/v1/usage` y `/v1/appointments/{id}` responden `401 UNAUTHORIZED` sin `X-API-Key`. La raíz sin `/v1` (`/doctors`, `/openapi.json`, `/docs`) devuelve el portal web del hospital y no la API: `CLINIC_BASE_URL` debe terminar en `/v1`. El hospital no publica disponibilidad para socios; las operaciones son médicos, turnos, cancelación y facturación. El `main` del proveedor (`73e9e28`) mantiene la versión 1.0.0 y agrega `422 OUTSIDE_SCHEDULE` para horarios fuera de la agenda semanal del médico, en hora de Argentina (UTC-03:00). Empleados lo trata como rechazo de negocio, igual que `LEAD_TIME_TOO_SHORT`. En el código del proveedor, la agenda es de lunes a viernes: `doc-001` de 9 a 13, `doc-002` de 14 a 18, `doc-003` de 8 a 12 y `doc-004` de 15 a 19. Esa agenda se confirma con el catálogo real y no se fija en Car Cruds.

> **Prueba real — 10/11/2026:** con la clave de Car Cruds, Empleados reservó un turno en el hospital publicado (employee-001 con `pat-001`, `doc-001`, 20/11 a las 10:00 de Argentina): `booked`/`confirmed`. El reenvío no creó otro turno, la consulta posterior lo confirmó y la facturación muestra exactamente un cargo de ARS 25.000. Ver la [evidencia](evidence/hospital-live-2026-11-10.json) y la [captura](evidence/img/hospital-turno-confirmado-2026-11-10.png).

> **Acceso compartido — 10/11/2026:** el grupo de Octi publicó Turnos para socios 1.0.0 en `https://api-turnos.srv1779429.hstgr.cloud/v1` y entregará la clave de Car Cruds por un canal privado. El contrato recibido es idéntico al integrado (solo agrega la URL pública en `servers`). Se verificó `/healthz` (200), el rechazo sin clave (401) y una latencia de unos 200 ms. Empleados agrega la lectura de médicos (`GET /v1/employees/clinic/doctors`) y de la facturación mensual (`GET /v1/employees/clinic/usage`); el panel elige el médico de ese catálogo y muestra los cargos. Precio informado: ARS 25.000 por reserva creada, sin reintegro al cancelar. Pacientes de prueba habilitados: `pat-001` y `pat-003` (`pat-002` registrado sin habilitar). Car Cruds no cancela turnos: la operación de cancelación del proveedor queda sin usar, como estaba decidido.
> **Verificación — 08/11/2026:** adaptación integrada en `main` por PR #5. Prueba del contrato contra gateway y Turnos originales aprobada con carreras en laboratorio; [evidencia](REVISION-E2-2026-11-08.md). URL compartida, clave y pacientes/datos operativos siguen pendientes; [aportes necesarios](PENDIENTES-E2.md).

Adaptación del 23 de octubre de 2026 a **Turnos para socios 1.0.0**, limitada a solicitar y consultar turnos para empleados. [ADR-019](adr/ADR-019-consumo-turnos-socios.md) sustituye D9 anterior y complementa [ADR-018](adr/ADR-018-empleados-sin-candidatos.md). No se modifica el repositorio del hospital.

## Contrato consumido y alcance de entrega 2

Referencia: [OpenAPI oficial publicado](https://github.com/OctiDelFabro/portal-paciente-clinica/blob/c0b65f45867f884e0bdc8f7c23eb0f344f646bb2/docs/contracts/turnos-partner-v1.openapi.json), rama `feature/entrega-2`, commit `c0b65f45867f884e0bdc8f7c23eb0f344f646bb2`. La [copia local](contracts/providers/octi-turnos-partner-v1.openapi.json) conserva el contenido del proveedor. El [contrato observado anterior](contracts/providers/octi-turnos-observed.openapi.json), fijado a `5e4c210`, permanece como antecedente.

| Operación consumida | Ruta pública del hospital | Resultado |
| --- | --- | --- |
| Solicitar turno | `POST /v1/appointments` | 201 con turno nuevo o repetición idempotente del mismo turno |
| Consultar turno | `GET /v1/appointments/{id}` | 200 con evidencia actual del turno perteneciente al consumidor |

Ambas operaciones requieren `X-API-Key`; el POST requiere además `Idempotency-Key`. El cuerpo contiene exclusivamente `patientId`, `doctorId` y `startAt` RFC3339. El proveedor exige paciente habilitado, al menos 24 horas de anticipación y ausencia de superposiciones. Sus estados son `booked` y `cancelled`; la versión aumenta con los cambios. Una repetición puede devolver un turno ya cancelado: 201 no implica necesariamente un turno reservado actualmente.

Se conserva vínculo → intención → envío → consulta → informe administrativo, conforme al punto 6.3 del enunciado. No se incorporan al producto catálogo de médicos, cancelación saliente, estado de cuenta, frontend, despliegue ni procesamiento automático. Una cancelación administrada en el hospital se refleja al consultar. La llamada sale directamente de Empleados, sin frontend ni gateway propios.

## Configuración

En el `.env` de Car Cruds, fuera de Git:

```dotenv
CLINIC_BASE_URL=https://api-turnos.srv1779429.hstgr.cloud/v1
CLINIC_API_KEY=credencial-entregada-por-el-hospital
CLINIC_ALLOW_HTTP=false
CLINIC_TIMEOUT=2s
```

La base incluye `/v1`; el cliente agrega `/appointments` o `/appointments/{id}`. En laboratorio se utiliza el gateway, por ejemplo `http://host.docker.internal:8088/v1`, con HTTP expresamente permitido. No usar `/api/turnos` ni el puerto interno de Turnos como API de socios. En Docker, `localhost` refiere al propio contenedor.

Sin URL, Empleados inicia y permite vínculos, intenciones y consultas locales; enviar/actualizar devuelve `503 NOT_CONFIGURED`. Con URL configurada se requiere clave; URL, clave o timeout inválidos impiden iniciar. Timeout configurable 100 ms–5 s, predeterminado 2 s; máximo ocho llamadas simultáneas, sin cola ilimitada ni redirecciones. Cada acción explícita ejecuta como máximo una llamada HTTP: no hay bucles ni reintentos en segundo plano. Se propagan trazas y `X-Correlation-ID`, sin reenviar el Bearer administrativo ni publicar la clave clínica.

## Uso de nuestra API

Contrato propio [clinic-integration-openapi.json](contracts/clinic-integration-openapi.json), versión **0.3.0**. Las acciones requieren `Authorization: Bearer <AUTH_ADMIN_TOKEN>`, diferente de la credencial del hospital. `personKind` conserva únicamente `employee`.

### 1. Vincular empleado activo y paciente

`POST /v1/employees/clinic/patient-links`, `Content-Type: application/json`:

```json
{"personKind":"employee","personId":"employee-001","patientId":"pat-001"}
```

Requiere empleado existente y activo; un inactivo devuelve `409 EMPLOYEE_INACTIVE`. La identidad del paciente se verifica administrativamente, sin equiparar IDs ni comparar nombres automáticamente. Repetir el vínculo es posible mientras el empleado sea elegible; reasignarlo silenciosamente devuelve conflicto. Los ejemplos son ficticios: esta operación no crea ni habilita pacientes en el hospital. El procedimiento real continúa pendiente de acuerdo.

### 2. Registrar intención administrativa

`POST /v1/employees/clinic/requests`:

```json
{"personKind":"employee","personId":"employee-001","localReference":"revision-personal-001","doctorId":"doc-001","startAt":"2026-12-01T10:00:00Z"}
```

Exige vínculo previo y empleado activo; médico y horario deben acordarse, con al menos 24 horas de anticipación. El 200 devuelve intención `ready` y clasificación `pending_verification`, sin HTTP externo. Repetir `localReference` con iguales datos recupera la intención; cambiar persona, médico o inicio devuelve conflicto. Otra referencia representa otra consulta y no debe usarse para recuperar una operación incierta.

### 3. Enviar y recuperar resultados desconocidos

`POST /v1/employees/clinic/requests/{requestId}/submit`, **sin cuerpo**.

Antes de HTTP se guardan procedencia del contrato `turnos-partner-v1` y clave idempotente igual al `requestId`. Paciente, médico e inicio permanecen iguales en todos los intentos. No se usa `localReference` como clave de encabezado. La transacción SQL termina antes de la red.

El reclamo durable limita envíos concurrentes mediante una espera de diez segundos. Los errores recuperables conservan enfriamiento mínimo de diez segundos, ampliado por `Retry-After` ante 429. `nextAttemptAt` indica cuándo puede intentarse nuevamente, sin disparar acciones al vencer. Tras timeout, reinicio o 503, Administración puede repetir explícitamente `submit` con la misma clave y datos una vez transcurrida la espera. No hay reintento automático.

Todo envío que pueda crear, incluido el reintento incierto, vuelve a exigir empleado activo. Una baja bloquea ese POST, porque el primer intento podría no haber llegado; conserva consulta y actualización de evidencia anterior. Una solicitud confirmada, cancelada o rechazada devuelve su registro sin crear otro turno. Las claves y la procedencia del contrato permanecen internas.

| Respuesta/falla del hospital | Estado y tratamiento local |
| --- | --- |
| 201 válido | `booked` o `cancelled`, según evidencia; guarda referencia, versión y fecha |
| Rechazo de negocio documentado: 400 `INVALID_REQUEST`/`INVALID_JSON`, 403 `PATIENT_NOT_ENABLED`, 404 `UNKNOWN_DOCTOR`/`UNKNOWN_PATIENT`, 409 `SLOT_UNAVAILABLE`, 422 `LEAD_TIME_TOO_SHORT`/`OUTSIDE_SCHEDULE` | `rejected`, con el código como motivo; definitivo: no vuelve a enviarse ni tiene `nextAttemptAt`. Exige coincidencia exacta de estado y código: otra combinación, como 409 `OUTSIDE_SCHEDULE`, sigue siendo `CLINIC_UNAVAILABLE` |
| 401 | `unknown`, `CLINIC_UNAUTHORIZED`; corregir credencial conservando el mismo consumidor antes de reintentar |
| 429 | `unknown`, `CLINIC_RATE_LIMITED`; respetar `Retry-After` y `nextAttemptAt` |
| 409 `IDEMPOTENCY_CONFLICT` | `unknown`, `CLINIC_IDEMPOTENCY_CONFLICT`; requiere revisión, sin cambiar clave ni reenvío automático |
| Timeout, conexión perdida, 5xx (incluido 503 `DIRECTORY_UNAVAILABLE`), código no documentado o JSON inválido | `unknown`, `CLINIC_UNAVAILABLE`; espera de 10 s y recuperación explícita con la misma intención y la misma clave |
| GET 404 | `unknown`, `CLINIC_NOT_FOUND`; inexistencia o falta de pertenencia no prueban ausencia de turno |

El hospital rechaza `OUTSIDE_SCHEDULE` dentro de su transacción de reserva, antes de insertar el turno: no crea turno, ni cargo, ni registro de idempotencia. Por eso se trata como rechazo definitivo. Antes de esta corrección quedaba como `CLINIC_UNAVAILABLE` y el panel invitaba a reintentar.

El 200 propio informa el estado administrativo; no acredita reserva exitosa. Una respuesta estructuralmente válida que no coincide con la intención conserva `INVALID_RESPONSE`; otros fallos de verificación conservan `LOOKUP_UNCONFIRMED` cuando corresponde. No se propagan como rechazos de negocio la falta de autenticación, saturación ni incertidumbre operativa.

### 4. Consultar, conciliar y conservar historia

- `GET /v1/employees/clinic/requests/{requestId}` lee la intención local sin HTTP externo.
- `POST /v1/employees/clinic/requests/{requestId}/refresh`, sin cuerpo, consulta el turno vinculado por ID.
- Para conciliar una respuesta perdida se admite `{"appointmentId":"id-verificado-del-hospital"}` en `refresh`: el turno debe ser legible con la credencial de Car Cruds y deben coincidir paciente, médico e inicio.

Una versión atrasada no reemplaza otra nueva; igual versión con contenido diferente devuelve conflicto. Un fallo conserva `lastKnownAppointment` y su fecha, dejando pendiente la verificación; un fallo atrasado no sobrescribe una observación posterior. Las bajas permiten este seguimiento y no cancelan turnos.

**Solicitudes antiguas:** una intención ya enviada sin procedencia y clave persistidas nunca vuelve a ejecutar POST. Agregar una clave retrospectiva no prueba que el proveedor la recibiera antes. La lectura/conciliación por ID requiere que la clínica asocie el turno al consumidor; un 404 no autoriza recrearlo. Se conservan migraciones y evidencia previa.

### 5. Registro de cada llamada al hospital

Empleados escribe un registro JSON por cada llamada HTTP al hospital (`event=clinic.call`): nivel `INFO` si salió bien y `WARN` en otro caso. Si la entrada se rechaza antes de llamar, no se registra nada.

| Campo | Contenido |
| --- | --- |
| `operation` | `book`, `lookup`, `doctors` o `usage` |
| `request_id` / `appointment_id` / `period` | Identificador ya validado por Car Cruds, según la operación |
| `http_status` | Estado HTTP del hospital; `0` si no llegó respuesta |
| `provider_code` | Código de error del hospital, solo si tiene forma de código (`[A-Z][A-Z0-9_]*`); otro valor se registra como `UNRECOGNIZED` |
| `outcome` | `ok`, `rejected`, `failure` (401, 429, 409 de idempotencia, 404 de consulta), `invalid_input` o `unavailable` |
| `result_code` | Código guardado en la solicitud, por ejemplo `CLINIC_UNAVAILABLE` o `CLINIC_UNAUTHORIZED` |
| `cause` | Solo en `unavailable`: `timeout`, `transport`, `canceled`, `unexpected_status`, `invalid_body` u `oversized_body` |
| `duration_ms`, `appointment_id`, `appointment_status` | Duración, y turno y estado cuando el hospital confirma |

No se registran la clave, los encabezados, los cuerpos, el mensaje libre del hospital ni datos del paciente (`patientId`, médico, horario). Las pruebas lo verifican, incluso con un proveedor que devuelve la clave en su mensaje o en su código. Para revisar una solicitud en la VM:

```bash
docker compose $F logs --since 24h employees | grep '"event":"clinic.call"' | grep '<requestId>'
```

**Caso del 11/11/2026 (`e89403e0-ba26-4fcb-b4cc-da062d0dc170`).** Se pidió `doc-001` para el sábado 12/12 a las 14:30 ART, fuera de su agenda de lunes a viernes de 9 a 13. Quedó `pending_verification` / `CLINIC_UNAVAILABLE`, sin turno externo. La causa más probable es un `422 OUTSIDE_SCHEDULE` que la versión desplegada no reconocía. La solicitud se envió antes de que existiera este registro, así que la confirmación sale de la traza saliente en Jaeger (`http.response.status_code`) o de los logs del hospital. Para descartar un cargo, la facturación de `2026-11` no debe tener otro turno. No se reenvía para conciliarla: se conserva como evidencia y, si corresponde, se registra otra intención con un horario de agenda.

## Informe administrativo

`GET /v1/employees/reports/clinic?page=1&pageSize=20`, roles `admin` o `service`. Incluye empleados activos e inactivos. Se admite `personKind=employee`; Se rechazan otros tipos de persona. Muestra la última intención por empleado, referencias, evidencia y fechas, sin consultar todos sus turnos en el hospital.

| Clasificación | Alcance |
| --- | --- |
| `confirmed` | Confirmación válida del turno de la última intención |
| `without_confirmed_appointment` | Esa intención fue rechazada explícitamente o su turno está cancelado |
| `pending_verification` | Falta evidencia, no se envió o el resultado permanece desconocido |

La confirmación no acredita asistencia ni aptitud médica. Leer el informe no genera consultas externas masivas. Se muestra la fecha sin inventar un SLA acordado.

## Condiciones y acuerdos pendientes

El proveedor documenta un cargo por reserva nueva, sin duplicarlo ante repetición idempotente, y cancelación sin reintegro. La tarifa publicada de **ARS 25.000 permanece pendiente de aceptación bilateral**: no se fija en código ni se implementa facturación propia o consumo de `/v1/usage`.

Faltan URL compartida, clave privada, procedimiento de alta/vinculación/habilitación de pacientes reales, correspondencia de médicos, datos controlados para prueba conjunta, tratamiento de turnos anteriores y confirmación económica. No hay consulta de disponibilidad ni indicación clínica de quién necesita atención; puede iniciarse por indicación administrativa local. Ver [INTEGRACIONES](INTEGRACIONES.md).

## Verificación y límites de evidencia

Se requiere probar encabezados, respuestas inválidas, errores, duplicados, respuesta perdida, reinicio, concurrencia SQL, espera `Retry-After`, elegibilidad laboral y conservación histórica. El escenario `integration/cliniccontract` utiliza el gateway del código original fijado a `c0b65f4`, credenciales de prueba y dependencias aisladas, sin modificarlo. La evidencia anterior contra `5e4c210` conserva su fecha y no acredita socios 1.0.0.

Los resultados efectivos de la rama se registran en [ESTADO-IMPLEMENTACION](ESTADO-IMPLEMENTACION.md) y su pull request. Prueba local, aceptación bilateral y conexión pública son estados diferentes; no se afirma integración real hasta verificarla con ambos equipos.

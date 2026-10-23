# Comunicación con el hospital de Octi

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
CLINIC_BASE_URL=https://host-acordado-del-hospital/v1
CLINIC_API_KEY=credencial-entregada-por-el-hospital
CLINIC_ALLOW_HTTP=false
CLINIC_TIMEOUT=2s
```

La base incluye `/v1`; el cliente agrega `/appointments` o `/appointments/{id}`. En laboratorio se utiliza el gateway, por ejemplo `http://host.docker.internal:8088/v1`, con HTTP expresamente permitido. No usar `/api/turnos` ni el puerto interno de Turnos como API de socios. En Docker, `localhost` refiere al propio contenedor.

Sin URL, Empleados inicia y permite vínculos, intenciones y consultas locales; enviar/actualizar devuelve `503 NOT_CONFIGURED`. Con URL configurada se requiere clave; URL, clave o timeout inválidos impiden iniciar. Timeout configurable 100 ms–5 s, predeterminado 2 s; máximo ocho llamadas simultáneas, sin cola ilimitada ni redirecciones. Cada acción explícita ejecuta como máximo una llamada HTTP: no hay bucles ni reintentos en segundo plano. Se propagan trazas y `X-Correlation-ID`, sin reenviar el Bearer administrativo ni publicar la clave clínica.

## Uso de nuestra API

Contrato propio [clinic-integration-openapi.json](contracts/clinic-integration-openapi.json), versión **0.3.0**. Las acciones requieren `Authorization: Bearer <AUTH_ADMIN_TOKEN>`, diferente de la credencial del hospital. `personKind` conserva únicamente `employee`; los candidatos históricos no tienen API activa.

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
| Rechazo de negocio documentado | `rejected`, con motivo administrativo acotado |
| 401 | `unknown`, `CLINIC_UNAUTHORIZED`; corregir credencial conservando el mismo consumidor antes de reintentar |
| 429 | `unknown`, `CLINIC_RATE_LIMITED`; respetar `Retry-After` y `nextAttemptAt` |
| 409 `IDEMPOTENCY_CONFLICT` | `unknown`, `CLINIC_IDEMPOTENCY_CONFLICT`; requiere revisión, sin cambiar clave ni reenvío automático |
| Timeout, conexión perdida, 5xx o JSON inválido | `unknown`, `CLINIC_UNAVAILABLE`; recuperación explícita con la misma intención |
| GET 404 | `unknown`, `CLINIC_NOT_FOUND`; inexistencia o falta de pertenencia no prueban ausencia de turno |

El 200 propio informa el estado administrativo; no acredita reserva exitosa. Una respuesta estructuralmente válida que no coincide con la intención conserva `INVALID_RESPONSE`; otros fallos de verificación conservan `LOOKUP_UNCONFIRMED` cuando corresponde. No se propagan como rechazos de negocio la falta de autenticación, saturación ni incertidumbre operativa.

### 4. Consultar, conciliar y conservar historia

- `GET /v1/employees/clinic/requests/{requestId}` lee la intención local sin HTTP externo.
- `POST /v1/employees/clinic/requests/{requestId}/refresh`, sin cuerpo, consulta el turno vinculado por ID.
- Para conciliar una respuesta perdida se admite `{"appointmentId":"id-verificado-del-hospital"}` en `refresh`: el turno debe ser legible con la credencial de Car Cruds y deben coincidir paciente, médico e inicio.

Una versión atrasada no reemplaza otra nueva; igual versión con contenido diferente devuelve conflicto. Un fallo conserva `lastKnownAppointment` y su fecha, dejando pendiente la verificación; un fallo atrasado no sobrescribe una observación posterior. Las bajas permiten este seguimiento y no cancelan turnos.

**Solicitudes antiguas:** una intención ya enviada sin procedencia y clave persistidas nunca vuelve a ejecutar POST. Agregar una clave retrospectiva no prueba que el proveedor la recibiera antes. La lectura/conciliación por ID requiere que la clínica asocie el turno al consumidor; un 404 no autoriza recrearlo. Migraciones y evidencia se conservan, sin eliminar ni promover datos de candidatos históricos.

## Informe administrativo

`GET /v1/employees/reports/clinic?page=1&pageSize=20`, roles `admin` o `service`. Incluye empleados activos e inactivos, no candidatos. Se admite `personKind=employee`; `candidate` se rechaza. Muestra la última intención por empleado, referencias, evidencia y fechas, sin consultar todos sus turnos en el hospital.

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

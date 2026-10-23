# ADR-019 — Consumo de Turnos para socios

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D9: consumo del proveedor |
| Decisiones complementadas | D5: comunicación; D10: resiliencia; D11: observabilidad |
| Fecha | 23 de octubre de 2026 |
| Estado | Implementado en rama de propuesta; pruebas locales parciales, dependencias reales/CI y revisión del equipo pendientes |
| Sustituye | [ADR-009](ADR-009-consumo-clinica.md) en el consumo de Turnos |
| Conserva | [ADR-018](ADR-018-empleados-sin-candidatos.md): empleados, conservación histórica y seguimiento de bajas |

## 1. Contexto y justificación

El hospital de Octi publicó Turnos para socios 1.0.0 en `c0b65f45867f884e0bdc8f7c23eb0f344f646bb2`. Reemplaza el uso de las rutas internas observadas en `5e4c210` por API pública `/v1`, clave por consumidor, idempotencia durable y aislamiento de turnos. El envío como máximo una vez elegido ante la API antigua evita duplicados, pero impide recuperar con seguridad respuestas perdidas. El contrato nuevo permite repetir una intención identificada sin crear ni facturar otra reserva.

La adaptación conserva el alcance de la entrega 2 (enunciado 6.3): flujo administrativo operativo, almacenamiento real, logs y trazabilidad inicial. No se agregan funcionalidades de presentación final. Se mantienen arquitectura hexagonal, PostgreSQL propio y llamada directa desde Empleados.

## 2. Decisión

### Contrato y responsabilidad

Consumir exclusivamente `POST /v1/appointments` y `GET /v1/appointments/{id}` del contrato oficial, mediante `X-API-Key` privada. Conservar una copia exacta del [OpenAPI oficial](../contracts/providers/octi-turnos-partner-v1.openapi.json), el commit y la especificación observada anterior. No modificar el proveedor ni asumir que la existencia del contrato acredita acuerdo bilateral.

Se conserva vínculo explícito entre empleado y paciente, intención durable, envío, consulta e informe. La clave de consumidor no acredita relación laboral: nuevo vínculo, intención y cualquier POST que pueda crear, incluso recuperación de resultado incierto, requieren empleado activo. Leer/actualizar evidencia anterior de inactivos permanece permitido, sin cancelaciones automáticas. Los candidatos históricos siguen sin acceso por la API.

### Idempotencia, concurrencia y recuperación

Guardar antes de la red `ProviderContract=turnos-partner-v1` e `IdempotencyKey=requestId`, con los datos originales de paciente, médico y horario. La referencia administrativa no reemplaza la clave saliente. No cambiar clave ni cuerpo para recuperar una operación desconocida.

Reclamar la intención bajo bloqueo PostgreSQL, con espera durable de diez segundos. Terminar la transacción antes de HTTP. Toda acción explícita realiza como máximo un intento; no hay trabajador en segundo plano ni bucle automático. Tras fallo recuperable se conserva un enfriamiento mínimo de diez segundos, ampliado por `Retry-After` en 429. `nextAttemptAt` comunica cuándo podría solicitarse otro intento, sin ejecutarlo por sí misma.

Una respuesta 201 válida puede contener `booked` o `cancelled`, incluyendo la repetición de un turno cancelado. Ambas deben traducirse según la evidencia observada. Una solicitud terminal o con turno ya vinculado no vuelve a reservar. Conflicto idempotente requiere revisión/conciliación, sin nuevo POST ni clave diferente.

Una intención enviada bajo la API antigua, sin procedencia y clave guardadas, jamás se reenvía como POST. Una intención todavía no enviada puede adoptar el contrato vigente al primer reclamo. La conciliación antigua por ID depende del acuerdo de pertenencia del proveedor; 404 no autoriza recrear el turno.

### Errores y protección

| Situación | Evidencia local y recuperación |
| --- | --- |
| Rechazo de negocio documentado | `rejected`, motivo administrativo acotado |
| 401 | `unknown / CLINIC_UNAUTHORIZED`; corregir credencial preservando consumidor |
| 429 | `unknown / CLINIC_RATE_LIMITED`; respetar espera indicada |
| 409 idempotente | `unknown / CLINIC_IDEMPOTENCY_CONFLICT`; revisión manual y conciliación |
| Timeout, transporte, 5xx o JSON inválido | `unknown / CLINIC_UNAVAILABLE`; recuperación explícita de la misma intención |
| GET 404 | `unknown / CLINIC_NOT_FOUND`; puede ocultar turno ajeno, no implica ausencia |

Se conservan además `INVALID_RESPONSE` para evidencia válida en estructura que no coincide con la intención y `LOOKUP_UNCONFIRMED` para fallos de verificación cuando corresponde. Se conserva evidencia previa y fecha; una observación o error atrasados no deben reemplazar evidencia posterior. Una respuesta propia 200 describe la intención, no necesariamente una reserva confirmada. Se mantiene timeout configurable 100 ms–5 s (inicial 2 s), límite de ocho llamadas simultáneas y redirecciones deshabilitadas. La llamada no mantiene una transacción SQL abierta.

Se conservan logs estructurados, métricas HTTP y propagación de trazas/correlación, sin registrar claves ni ampliar información clínica publicada. Tableros, alertas y una nueva evidencia de traza permanecen sujetos a verificación; esta decisión no inventa resultados.

## 3. Alternativas consideradas

- **Cambiar solo URL y encabezados:** descartado porque dejaría bloqueada la recuperación persistente y mezclaría solicitudes antiguas con nuevas.
- **Conservar un único envío para todo:** seguro ante la API antigua, pero pierde recuperación disponible bajo idempotencia documentada.
- **Reintentar automáticamente o usar otra clave:** descartado por amplificar fallas, ocultar costos y, con otra clave, permitir duplicados.
- **Incorporar todo el contrato nuevo:** diferido. Catálogo de médicos, cancelación saliente, consumos/facturación, frontend y despliegue exceden esta adaptación acotada.

## 4. Consecuencias, verificación y pendientes

Se añaden datos internos de procedencia/clave/espera sin exponer secretos en la API. El contrato administrativo clínico pasa a 0.3.0 e incorpora `nextAttemptAt` opcional. El contrato de personal/gimnasio permanece 0.3.0 sin cambio.

La suite local Windows obtuvo 127 pruebas/subpruebas aprobadas, 29 omitidas y ninguna fallida mediante `go test -count=1 -json -timeout 120s ./...`; `go vet ./...` y test/vet del mock histórico satisfactorios. Sin Docker/GCC y con CGO deshabilitado, las omisiones no acreditan dependencias reales ni detector de carreras. Compiló el escenario de contrato, pero su ejecución contra el gateway `c0b65f4`, autenticación, aislamiento, pérdida de respuesta, consulta/cancelación, y la persistencia/concurrencia SQL quedan para CI. [ESTADO-IMPLEMENTACION](../ESTADO-IMPLEMENTACION.md) y el PR registran el corte y sus límites; los ensayos antiguos no prueban esta revisión.

Pendientes externos: URL, credencial privada, alta/habilitación de pacientes reales, médicos y datos de prueba, pertenencia de turnos antiguos, confirmación económica y ensayo conjunto. El proveedor publica ARS 25.000 como tarifa inicial pendiente de confirmación; no se codifica como precio aceptado ni se incorpora facturación. La necesidad médica puede indicarse administrativamente; una indicación proveniente del hospital sigue sin capacidad pública.

## 5. Trazabilidad y material de cátedra

RF-15/HU-15: solicitud y recuperación de una intención; RF-16/HU-16: consulta, orden e historia; RF-17/HU-17: informe con incertidumbre explícita. Complementa D5 en timeouts/reintentos/idempotencia, D10 en aislamiento y esperas acotadas, D11 en correlación y protección de secretos.

Material consultado en `clases.zip`: `teoria/07-resiliencia.md` (resultado desconocido, deadline, reintentos y límites), `teoria/08-estilos-arquitectura-backend.md` (puertos/adaptadores) y `practica/README.md` (interfaces, Repository y concurrencia). Requisitos académicos: enunciado 2.5, 2.9, 4.1 y 6.3. Guía operativa: [COMUNICACION-HOSPITAL](../COMUNICACION-HOSPITAL.md).

# Contratos propios e integración

**Revisión vigente — 23/10/2026:** personal/gimnasio y la API administrativa clínica usan versión 0.3.0. [ADR-018](../adr/ADR-018-empleados-sin-candidatos.md) retira candidatos y [ADR-019](../adr/ADR-019-consumo-turnos-socios.md) adapta el flujo clínico a Turnos para socios 1.0.0 fijado a `c0b65f4`. Nuevos vínculos, intenciones y envíos que puedan crear exigen empleado activo; seguimiento previo de inactivos se conserva. La lista y membresías del gimnasio no cambian. Pruebas locales y aceptación externa se registran por separado.

Actualización del 22 de octubre: [API administrativa de hospital](clinic-integration-openapi.json), versión 0.1.0, y [contrato observado de Turnos de Octi](providers/octi-turnos-observed.openapi.json). El segundo documenta código publicado, no un acuerdo bilateral ni el OpenAPI oficial del proveedor. Guía de configuración, uso y recuperación: [COMUNICACION-HOSPITAL](../COMUNICACION-HOSPITAL.md). Se conserva la lista e intercambio propio del gimnasio.

| Campo | Valor |
| --- | --- |
| Versión documental | 0.4 |
| Fecha | 23 de octubre de 2026 |
| Estado | API propias implementadas; acuerdos externos pendientes |

Empleados concentra la integración con el gimnasio del grupo de Pablo y el hospital del grupo de Octi, denominado Clínica en el diseño. La implementación de una API propia no constituye aceptación del consumidor. Los contratos y accesos de esos proveedores siguen pendientes en [INTEGRACIONES](../INTEGRACIONES.md).

| Artefacto | Capacidad | Estado |
| --- | --- | --- |
| [employees-openapi.json](employees-openapi.json) | Lista de empleados activos, administración de empleados y evidencia/informe de membresías | Contrato OpenAPI 3.0.3 de la API propia implementada, versión 0.3.0 provisional |
| [employees-openapi-draft.json](employees-openapi-draft.json) y [EMPLOYEES.md](EMPLOYEES.md) | Propuesta inicial de lista compartida | Antecedente de diseño; no es un acuerdo externo ni la referencia completa del servicio actual |
| [rentals-openapi.json](rentals-openapi.json) | Disponibilidad, reservas, mantenimiento, retiro y devolución | Contrato del backend propio implementado; revisar junto con [su guía](../../services/rentals/README.md) |
| [openapi-v1.json](openapi-v1.json) y [RESERVAS.md](RESERVAS.md) | Disponibilidad/reservas de entrega 1 | Contrato histórico 1.0.0 del mock; conservado sin cambiar su significado |
| [clinic-integration-openapi.json](clinic-integration-openapi.json) | Vinculación, intenciones, seguimiento e informe de turnos de empleados | API administrativa propia 0.3.0, con `nextAttemptAt` opcional; no expone clave ni procedencia interna |
| [Turnos para socios](providers/octi-turnos-partner-v1.openapi.json) | Contrato oficial del hospital, versión 1.0.0 | Copia exacta de `c0b65f4`; consume POST/GET de turnos por gateway con credencial e idempotencia |
| [CLINIC.md](CLINIC.md) y [contrato observado](providers/octi-turnos-observed.openapi.json) | Antecedente y revisión antigua de Turnos | Referencia histórica fijada a `5e4c210`; sus evidencias no acreditan el contrato nuevo |

## API propia de Empleados

`GET /v1/employees` está disponible por gateway local para `gym`, `admin` y `service`. Devuelve empleados activos, exclusivamente `employeeId` y `displayName`. La paginación utiliza `page` desde 1, `pageSize` predeterminado 20 y máximo 100; conteo e ítems de una página provienen de la misma instantánea de PostgreSQL. No existe una instantánea compartida entre páginas, ni debe inferirse una baja laboral por una ausencia transitoria en el recorrido.

Las operaciones administrativas requieren `admin`; el gimnasio no puede modificar empleados ni consultar informes internos. Las rutas de candidatos fueron retiradas y devuelven 404 con autenticación válida. La API clínica mantiene `personKind`/`personId` con único tipo `employee`; el informe ya no selecciona candidatos. Los ejemplos son ficticios. Los tokens locales se generan fuera de Git; no se publican credenciales ni URL de producción.

[ADR-017](../adr/ADR-017-contrato-empleados.md) sustituye D8 histórico y registra esta capacidad propia. Población, bajas, identidad externa, compatibilidad, acceso y condiciones de consumo deben acordarse antes de presentar la integración como validada.

## Validación y publicación

La verificación reproducible propia es `go run ./cmd/checkshared -url http://127.0.0.1:8080 -env .env`. En una máquina consumidora con `AUTH_GYM_TOKEN` se utiliza `-url https://<host>` sin el archivo de credenciales del servidor. Comprueba `401` sin token/con token inválido, lista mínima y paginación, entrada inválida y `403` para lecturas administrativas/integraciones y `404` para rutas de candidatos retiradas; realiza únicamente GET y no imprime nombres ni credenciales. No sustituye al test de contrato que deberá acordarse con el consumidor.

Las pruebas locales verifican la API propia. La prueba conjunta con el gimnasio, su servidor consumidor y el intercambio económico continúan pendientes. El escenario de contrato clínico se adapta al gateway original de `c0b65f4`, con autenticación e idempotencia; sus resultados se registran al ejecutarlo. Los resultados previos de `5e4c210` no acreditan esta adaptación. La prueba conjunta y la operación pública siguen pendientes.

La capacidad compartida deberá permanecer accesible hasta finalizar la evaluación una vez publicada. La URL pública y fecha de publicación se registrarán al disponer de ellas; la forma de publicación (HTTPS mediante Caddy delante del gateway) está en [DESPLIEGUE](../DESPLIEGUE.md). La API de reservas del mock conserva su historial y no se utiliza como integración actual del gimnasio.

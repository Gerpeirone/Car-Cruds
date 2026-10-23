# Contratos propios e integración

Actualización del 22 de octubre: [API administrativa de hospital](clinic-integration-openapi.json), versión 0.1.0, y [contrato observado de Turnos de Octi](providers/octi-turnos-observed.openapi.json). El segundo documenta código publicado, no un acuerdo bilateral ni el OpenAPI oficial del proveedor. Guía de configuración, uso y recuperación: [COMUNICACION-HOSPITAL](../COMUNICACION-HOSPITAL.md). Se conserva la lista e intercambio propio del gimnasio.

| Campo | Valor |
| --- | --- |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | API propias implementadas; acuerdos externos pendientes |

Empleados concentra la integración con el gimnasio del grupo de Pablo y el hospital del grupo de Octi, denominado Clínica en el diseño. La implementación de una API propia no constituye aceptación del consumidor. Los contratos y accesos de esos proveedores siguen pendientes en [INTEGRACIONES](../INTEGRACIONES.md).

| Artefacto | Capacidad | Estado |
| --- | --- | --- |
| [employees-openapi.json](employees-openapi.json) | Lista de empleados activos y administración de empleados/candidatos | Contrato OpenAPI 3.0.3 de la API propia implementada, versión 0.1.0 provisional |
| [employees-openapi-draft.json](employees-openapi-draft.json) y [EMPLOYEES.md](EMPLOYEES.md) | Propuesta inicial de lista compartida | Antecedente de diseño; no es un acuerdo externo ni la referencia completa del servicio actual |
| [rentals-openapi.json](rentals-openapi.json) | Disponibilidad, reservas, mantenimiento, retiro y devolución | Contrato del backend propio implementado; revisar junto con [su guía](../../services/rentals/README.md) |
| [openapi-v1.json](openapi-v1.json) y [RESERVAS.md](RESERVAS.md) | Disponibilidad/reservas de entrega 1 | Contrato histórico 1.0.0 del mock; conservado sin cambiar su significado |
| [CLINIC.md](CLINIC.md) | Condiciones para consumir turnos reales | Contrato del proveedor no recibido; sin API externa inventada |
| Membresías, uso y gastos | Intercambio con gimnasio para informes administrativos | Mecanismo, contrato, períodos, moneda y datos pendientes |

## API propia de Empleados

`GET /v1/employees` está disponible por gateway local para `gym`, `admin` y `service`. Devuelve empleados activos, exclusivamente `employeeId` y `displayName`. La paginación utiliza `page` desde 1, `pageSize` predeterminado 20 y máximo 100; conteo e ítems de una página provienen de la misma instantánea de PostgreSQL. No existe una instantánea compartida entre páginas, ni debe inferirse una baja laboral por una ausencia transitoria en el recorrido.

Las operaciones administrativas y candidatos requieren `admin`; el gimnasio no puede modificarlos ni consultar candidatos. Los ejemplos son ficticios. Los tokens locales se generan fuera de Git; no se publican credenciales ni URL de producción.

[ADR-017](../adr/ADR-017-contrato-empleados.md) sustituye D8 histórico y registra esta capacidad propia. Población, bajas, identidad externa, compatibilidad, acceso y condiciones de consumo deben acordarse antes de presentar la integración como validada.

## Validación y publicación

La verificación reproducible propia es `go run ./cmd/checkshared -url http://127.0.0.1:8080 -env .env`. En una máquina consumidora con `AUTH_GYM_TOKEN` se utiliza `-url https://<host>` sin el archivo de credenciales del servidor. Comprueba `401` sin token/con token inválido, lista mínima y paginación, entrada inválida y `403` para lecturas administrativas/candidatos/integraciones; realiza únicamente GET y no imprime nombres ni credenciales. No sustituye al test de contrato que deberá acordarse con el consumidor.

Las pruebas locales de contrato y autorización verifican el servicio propio. La prueba conjunta con el gimnasio, su servidor consumidor y el intercambio económico continúan pendientes. También falta probar el contrato real del hospital; los puertos existentes no representan un adaptador operativo.

La capacidad compartida deberá permanecer accesible hasta finalizar la evaluación una vez publicada. La URL pública y fecha de publicación se registrarán al disponer de ellas; la forma de publicación (HTTPS mediante Caddy delante del gateway) está en [DESPLIEGUE](../DESPLIEGUE.md). La API de reservas del mock conserva su historial y no se utiliza como integración actual del gimnasio.

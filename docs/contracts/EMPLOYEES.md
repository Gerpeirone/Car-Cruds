# Capacidad propuesta de Empleados para el gimnasio

> **Antecedente histórico — 10/11 de octubre de 2026.** Este documento conserva la propuesta inicial de lista y su estado previo a la implementación. La API propia actual está descrita en [employees-openapi.json](employees-openapi.json), y su evidencia/límites en [ESTADO-IMPLEMENTACION](../ESTADO-IMPLEMENTACION.md). Las afirmaciones posteriores sobre ausencia de servidor, PostgreSQL, Docker Compose o balanceador corresponden a esa fecha histórica. El contrato externo, acceso público y validación del gimnasio continúan pendientes.

**Equipos externos:** gimnasio del grupo de Pablo (consume nuestra lista de empleados) y hospital del grupo de Octi (Car Cruds consume su API). Las referencias a “Clínica” en el diseño identifican al hospital de Octi. Contratos, entornos y credenciales siguen pendientes de acuerdo.

| Campo | Valor |
| --- | --- |
| Fecha | 10 de octubre de 2026 |
| Estado | Borrador propio; no acordado, implementado ni publicado |
| Contrato procesable propuesto | [employees-openapi-draft.json](employees-openapi-draft.json) |
| Versión del borrador | 0.1.0; independiente de la API histórica de reservas |

## 1. Alcance comunicado y propuesta

El gimnasio consumirá una lista de empleados de Car Cruds y devolverá información de uso de membresías. La lectura permite vincular esos beneficios a empleados de la empresa; Empleados utilizará la devolución para el informe administrativo.

Se propone una operación `GET /v1/employees`, con autenticación de aplicación y paginación. **La ruta, el formato, los campos, la paginación y la selección de población son propuestas de Car Cruds, no acuerdos externos.** No existe todavía un servidor ni URL pública para esta operación.

El borrador propone identificadores opacos `employeeId` y un nombre administrativo `displayName`, excluye candidatos y devuelve únicamente registros autorizados para esta integración. La selección de activos/inactivos y el alcance del nombre mostrado deben revisarse con el gimnasio. La relación con sus identificadores externos también está pendiente.

## 2. Condiciones propuestas

- Gateway valida identidad y límites de la aplicación; Empleados controla permiso de lectura y población autorizada.
- Comunicación pública mediante HTTPS; credenciales reales suministradas fuera del repositorio. El token de demostración del mock de reservas no es una credencial del gimnasio.
- Propuesta de paginación por `page` y `pageSize`, con máximo inicial de 100 registros por página. Los valores requieren acuerdo y validación; el borrador no garantiza una fotografía consistente mientras cambian datos entre páginas.
- Respuesta con `items`, `page`, `pageSize` y `total`. No contiene datos de clientes, médicos, contraseñas ni tarifas.
- Errores propios propuestos para entrada inválida, ausencia de autenticación, falta de permiso o indisponibilidad. No se atribuyen estos códigos a la clínica o al gimnasio.

La definición de una lista completa, snapshots o sincronización incremental deberá resolverse antes de que el consumidor dependa de su exhaustividad. Mientras se acuerda esa semántica, el borrador no garantiza que una persona ausente en una respuesta deba darse de baja en el gimnasio.

## 3. Devolución del uso

No hay operación de devolución definida en este borrador. El mecanismo, identificadores, período, significado de uso/no uso, deduplicación y precedencia de correcciones se acordarán en EXT-02 a EXT-05 de [INTEGRACIONES.md](../INTEGRACIONES.md). La falta de devolución se registra como información pendiente.

Tampoco hay precios o una fórmula de gastos acordados. El informe de uso precede a cualquier cálculo monetario.

## 4. Publicación y compatibilidad previstas

Antes de implementar y publicar: revisar el contrato conjuntamente, cerrar población/campos/errores, incorporar ejemplos acordados, definir entorno y credenciales, probar paginación y autorización y establecer el test de contrato del consumidor. Se propone conservar versiones incompatibles durante la transición acordada.

El contrato de reservas [openapi-v1.json](openapi-v1.json) sigue describiendo exclusivamente el mock de la entrega 1. No se cambia su significado ni se utiliza para representar empleados.


## Aclaración de alcance — 11 de octubre de 2026

El gimnasio de Pablo consume nuestra lista de empleados y administra membresías y costos. Empleados deberá recibir o consultar afiliación, costo por membresía, gastos por empleado y costos totales por período/moneda; el informe de uso es complementario y no reemplaza el económico. Se conservarán origen, referencias y correcciones, sin duplicar gastos ni asumir cero ante datos faltantes. El borrador de lista no define todavía el intercambio económico.

Car Cruds consume el hospital de Octi para empleados registrados: identificar quién necesita consulta mediante información recibida/consultada al hospital, solicitar el turno desde Empleados, conservar referencias y resultado, y reflejar cambios de estado, cancelaciones y reprogramaciones administrados por el hospital. La gestión de candidatos existente se mantiene como flujo adicional y separado. No se infieren diagnósticos ni asistencia a partir del turno.

Contratos, operaciones externas, mecanismos de actualización, identificadores y configuración operativa siguen pendientes. No se inventan endpoints externos ni valores de tarifas/timeouts. El gateway encamina el acceso entrante a Empleados; el adaptador saliente llama al hospital directamente. PostgreSQL propio, Docker Compose, configuración y pruebas de estos componentes aún no existen. El balanceador y las dos réplicas de Flota siguen como diseño pendiente de implementación y demostración.

La trazabilidad detallada está en SPEC (RN-16, RN-21 y RN-22), REQUERIMIENTOS (RF-13 a RF-17 y RF-19), BACKLOG (HU-13 a HU-17 y HU-23) e INTEGRACIONES (EXT-02, EXT-04 a EXT-10 y EXT-15). Esta aclaración registra el alcance solicitado por el grupo; no acredita aprobación externa ni implementación.

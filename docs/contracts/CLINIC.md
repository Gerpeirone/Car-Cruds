# Consumo de la clínica — acuerdos pendientes

**Equipos externos:** gimnasio del grupo de Pablo (consume nuestra lista de empleados) y hospital del grupo de Octi (Car Cruds consume su API). Las referencias a “Clínica” en el diseño identifican al hospital de Octi. Contratos, entornos y credenciales siguen pendientes de acuerdo.

| Campo | Valor |
| --- | --- |
| Fecha | 10 de octubre de 2026 |
| Estado | Capacidad identificada; contrato externo no recibido |
| Consumidor interno | Servicio Empleados |

La capacidad comunicada es el servicio de turnos para consultas médicas de empleados registrados y, de forma adicional, revisiones de candidatos. Todavía no se dispone de su formato procesable, versión, URLs, operaciones, autenticación, datos obligatorios, errores ni política de idempotencia.

No se incluye una especificación OpenAPI ficticia del proveedor. Una vez recibido su contrato se conservará aquí la referencia o copia autorizada, la versión consumida y el mapeo al modelo administrativo de Car Cruds.

## 1. Puerto interno propuesto

Empleados necesita solicitar una revisión y conocer si existe un turno confirmado para una intención/persona. Son necesidades del negocio local; no afirman que la clínica publique operaciones con esos nombres. El adaptador traduce el contrato real y conserva referencia externa, estado administrativo, fecha y correlación cuando estén disponibles.

El responsable puede ver un resultado confirmado, ausencia confirmada de turno o información pendiente de verificar. Una reserva de turno no acredita asistencia ni aptitud médica. La forma de registrar candidatos y su posterior alta como empleados pertenece a Car Cruds.

## 2. Condiciones que bloquean el adaptador real

- EXT-07: contrato/versiones y operaciones efectivamente disponibles.
- EXT-08: reserva directa frente a pasos completados por la persona.
- EXT-09: consulta de estado, cancelación/reprogramación e idempotencia.
- EXT-02 y EXT-10 a EXT-13: identidad, datos mínimos, permisos, entorno, límites y escenarios de prueba.

Los acuerdos se siguen en [INTEGRACIONES.md](../INTEGRACIONES.md). Un mock local futuro deberá declararse simulación y representar solo lo respaldado por un contrato; no prueba que la integración externa funcione.

## 3. Verificación prevista

Test de contrato sobre operaciones consumidas, solicitudes repetidas, respuestas tardías, cambios incompatibles y pérdida de respuesta después de confirmar. Se verificará que el informe conserve incertidumbre y que la falla externa no afecte alquileres.

No se ejecutaron estas verificaciones ni se recibieron tarifas o resultados clínicos. Ver las decisiones [D9](../adr/ADR-009-consumo-clinica.md) y [D10](../adr/ADR-010-resiliencia.md).


## Aclaración de alcance — 11 de octubre de 2026

El gimnasio de Pablo consume nuestra lista de empleados y administra membresías y costos. Empleados deberá recibir o consultar afiliación, costo por membresía, gastos por empleado y costos totales por período/moneda; el informe de uso es complementario y no reemplaza el económico. Se conservarán origen, referencias y correcciones, sin duplicar gastos ni asumir cero ante datos faltantes. El borrador de lista no define todavía el intercambio económico.

Car Cruds consume el hospital de Octi para empleados registrados: identificar quién necesita consulta mediante información recibida/consultada al hospital, solicitar el turno desde Empleados, conservar referencias y resultado, y reflejar cambios de estado, cancelaciones y reprogramaciones administrados por el hospital. La gestión de candidatos existente se mantiene como flujo adicional y separado. No se infieren diagnósticos ni asistencia a partir del turno.

Contratos, operaciones externas, mecanismos de actualización, identificadores y configuración operativa siguen pendientes. No se inventan endpoints externos ni valores de tarifas/timeouts. El gateway encamina el acceso entrante a Empleados; el adaptador saliente llama al hospital directamente. PostgreSQL propio, Docker Compose, configuración y pruebas de estos componentes aún no existen. El balanceador y las dos réplicas de Flota siguen como diseño pendiente de implementación y demostración.

La trazabilidad detallada está en SPEC (RN-16, RN-21 y RN-22), REQUERIMIENTOS (RF-13 a RF-17 y RF-19), BACKLOG (HU-13 a HU-17 y HU-23) e INTEGRACIONES (EXT-02, EXT-04 a EXT-10 y EXT-15). Esta aclaración registra el alcance solicitado por el grupo; no acredita aprobación externa ni implementación.

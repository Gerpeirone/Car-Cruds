# ADR-004 — Consistencia y concurrencia

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D4: consistencia y concurrencia |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Garantías implementadas y probadas en PostgreSQL |
| Hito | Entrega 2 — 23 de octubre de 2026 |

## 1. Contexto

La operación crítica del sistema es la confirmación de una reserva: dos solicitudes válidas que compiten por el mismo auto y período deben producir una sola reserva, y un reintento no debe duplicar ni perder información. El mock de la entrega 1 resuelve esto con un bloqueo en memoria de un solo proceso, lo que no sirve con varias instancias ni sobrevive a un reinicio. [D3](ADR-015-persistencia-v2.md) propone PostgreSQL para Alquileres; esta decisión determina qué garantía concreta se apoya en esa base. Las reglas asociadas son RN-03 a RN-11 de [SPEC.md](../../SPEC.md).

## 2. Decisión

La consistencia crítica se concentra dentro de Alquileres, en transacciones locales de su base. No se utilizan transacciones distribuidas entre servicios.

| Operación | Garantía propuesta |
| --- | --- |
| Confirmar reserva (RN-03, RN-04) | Restricción de exclusión sobre `(vehículo, rango de fechas [inicio, fin))` en la tabla de agenda, para entradas que ocupan agenda (reservas `RESERVED` y `ACTIVE` y bloqueos de mantenimiento). La base rechaza el solapamiento aunque haya varias instancias; el conflicto se traduce en `409 VEHICLE_UNAVAILABLE`. |
| Idempotencia (RN-05) | Clave única `(aplicación, operación, clave)` con huella de la carga. Se inserta en la misma transacción que la reserva y la respuesta; misma clave y huella recuperan la respuesta original, otra huella se rechaza. |
| Intención de publicar (D5) | Registro en la tabla outbox dentro de la misma transacción que el cambio de negocio. Un relay lo entrega al broker y lo marca publicado tras la confirmación del publicador. |
| Retiro exclusivo (RN-08, RN-10) | Índice único parcial que admite como máximo un alquiler `ACTIVE` por vehículo, independiente de la fecha de devolución prevista. |
| Cancelar y devolver (RN-07, RN-09) | Transición condicional (`UPDATE … WHERE estado = 'RESERVED'` / `'ACTIVE'`) y lectura del resultado. Repetir la operación no cambia el estado ni libera la agenda dos veces. |
| Bloqueo por mantenimiento (RN-11) | Misma restricción de exclusión que las reservas: un solapamiento con una reserva o bloqueo existente produce conflicto y no cancela nada. |
| Consumo de eventos (D5) | Registro de mensaje procesado con clave única en la misma transacción que el efecto. La entrega al menos una vez resulta en un efecto una vez. El indexador aplica una actualización solo si la versión es mayor. |
| Empleados: evidencia externa (RN-19) | Clave única por `(empleado, período, origen, identificador de origen)` y comparación de versión o fecha de origen para no retroceder un resultado más reciente. |

El rango se interpreta semiabierto, de modo que dos períodos que comparten solo su frontera no se solapan, tal como define SPEC.

Las validaciones de Clientes (habilitación) y Flota (tarifa) son lecturas HTTP previas a la transacción. Pueden quedar desactualizadas entre la lectura y la escritura; esa ventana se acepta y se compensa volviendo a validar la habilitación en el retiro y congelando el precio de la lectura autoritativa al confirmar.

### Fallas parciales

| Falla | Resultado |
| --- | --- |
| La instancia cae antes de confirmar la transacción | No hay reserva ni respuesta guardada; el reintento con la misma clave la crea una sola vez. |
| Cae después de confirmar y antes de responder | El reintento con la misma clave devuelve la respuesta original. |
| El broker no está disponible | La reserva queda confirmada y el evento permanece pendiente en outbox. |
| El relay publica y cae antes de marcar el registro | El evento se publica de nuevo; el consumidor lo descarta por idempotencia. |

## 3. Alternativas consideradas

| Alternativa | Evaluación |
| --- | --- |
| Verificar solapamiento en la aplicación y luego insertar | Admite una carrera entre la lectura y la escritura; no protege con varias instancias. |
| Bloqueo en memoria o mutex como en el mock | Válido para un proceso; falla con dos instancias y se pierde al reiniciar. |
| `SELECT … FOR UPDATE` sobre una fila del vehículo o bloqueo consultivo | Garantiza serialización, pero depende de que toda ruta de escritura lo respete; la restricción de exclusión protege a nivel de datos. Queda como alternativa si la restricción no resultara viable. |
| Nivel de aislamiento serializable | Funciona, con reintentos por conflictos de serialización y un costo mayor en lecturas; no se elige sin mediciones. |
| Bloqueo distribuido con Redis | No es durable ni transaccional respecto de la base; agrega una dependencia y modos de falla. |
| Control optimista con versión por vehículo | Serializa reservas de fechas independientes del mismo auto sin necesidad. |

## 4. Consecuencias y validación

Se acepta la dependencia de una característica concreta de PostgreSQL (restricciones de exclusión sobre rangos); un cambio de motor reabriría esta decisión. La restricción no reemplaza las validaciones de negocio ni la clasificación de errores. Flota utiliza un outbox embebido acotado, según D3.

Validar contra una base real y no un mock:
- N solicitudes concurrentes válidas por el mismo vehículo y período producen una sola reserva y N−1 conflictos, también con dos instancias del servicio.
- Reiniciar el servicio y la base conserva reservas y respuestas idempotentes.
- Repetir confirmación, cancelación y devolución no duplica efectos.
- Mensajes duplicados y fuera de orden no duplican historial ni retroceden el índice.
- La prueba de integración usa una dependencia real, según el enunciado 2.11.

## Avance de implementación — 14 de octubre de 2026

La agenda utiliza exclusión de rangos, el retiro un índice único parcial y la idempotencia un registro durable con huella/respuesta. El flujo real produjo una ganadora 201 y un conflicto 409 ante dos reservas concurrentes; cancelación, retiro y devolución fueron repetibles. La devolución COMPLETED y RentalCompleted se guardan en una transacción: no se emite un evento de reserva sin consumidor. Clientes escribe inbox e historial juntos. Una reserva completada sobrevivió al reinicio. La evidencia externa de Empleados sigue pendiente; la tabla describe su garantía requerida, sin atribuir una implementación clínica. Ver [evidencia](../ESTADO-IMPLEMENTACION.md).

## 5. Referencias e historial

### Revisión del 15 de octubre de 2026

La validación HTTP se ejecuta ahora después de una consulta breve de idempotencia y antes de abrir la transacción. La escritura vuelve a consultar la clave bajo bloqueo; se conservan respuesta original, huella, exclusión y atomicidad de agenda/reserva/respuesta. Se añaden regresiones de misma clave concurrente y de validación con el único recurso del pool disponible. La evidencia de ejecución se distingue de los antecedentes en [REVISION-E2-2026-10-15](../REVISION-E2-2026-10-15.md). No se introduce una transacción distribuida ni se modifica el contrato HTTP.

Material en `clases.zip`: `teoria/02-datos-en-microservicios.md`, `teoria/04-comunicacion-asincrona.md` y `practica/clase-6-concurrencia-go.md`. [D3](ADR-015-persistencia-v2.md), [D5](ADR-016-comunicacion-v2.md), [D10](ADR-010-resiliencia.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.1 | 11 de octubre de 2026 | Primera propuesta de garantías de consistencia y concurrencia |
| 0.3 | 14 de octubre de 2026 | Implementación propia, evidencia local y pendientes registrados |

# ADR-005 — Comunicación entre servicios

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D5: comunicación entre servicios |
| Versión | 0.1 — inicial |
| Fecha | 7 de octubre de 2026 |
| Estado | Propuesto |
| Hito | Entrega 1 — 9 de octubre de 2026 |

## 1. Contexto

La reserva necesita una respuesta inmediata basada en la habilitación del cliente y la tarifa vigente. El índice de búsqueda y el historial admiten actualización posterior. La comunicación puede perder respuestas, duplicar solicitudes y dejar mensajes pendientes; las políticas deben resolver esos casos sin duplicar reservas ni perder eventos.

## 2. Decisión

### Comunicación síncrona

Se propone HTTP para la interacción del frontend con el gateway y para las validaciones de Alquileres hacia Clientes y Flota. La capacidad propia se publica como API HTTP v1. El proveedor externo se consume desde el microservicio responsable mediante el mecanismo de su contrato.

| Parámetro inicial | Valor propuesto |
| --- | --- |
| Plazo global de confirmación | 5 segundos |
| Tiempo de espera por llamada interna | 1 segundo |
| Tiempo máximo asignado al proveedor dentro del flujo | 2 segundos, cuando su capacidad sea necesaria |
| Reintentos de lecturas ante fallas transitorias | Como máximo uno, si resta tiempo dentro del plazo global |
| Espera previa al reintento | Aproximadamente 100 ms, con variación para reducir reintentos simultáneos |

Las llamadas y los reintentos deben caber en el plazo global. Los valores se validarán mediante trazas y mediciones.

| Situación | Tratamiento |
| --- | --- |
| Error transitorio de lectura | Reintento limitado según el tiempo restante |
| Error de validación, autenticación o conflicto | Respuesta al solicitante sin reintento automático |
| Respuesta perdida al crear reserva | Repetición con la misma `Idempotency-Key` y el mismo cuerpo |
| Cancelación repetida | Operación idempotente por identificador |
| Respuesta tardía o resultado desconocido | Recuperación mediante la misma solicitud idempotente o consulta de estado, si se conoce el identificador |

La idempotencia se resuelve antes de revalidar precio y agenda. La identidad de aplicación, la operación y la clave determinan el ámbito; reutilizar una clave con otra carga produce conflicto. La respuesta original se almacena atómicamente con el resultado de negocio. La versión real conservará las claves durante el período admitido de reintentos y, como mínimo, durante la vida de la reserva. El mock las conserva hasta el reinicio.

Al confirmar, Alquileres obtiene la tarifa vigente de la fuente autoritativa de Flota mediante una lectura que omite la caché. El precio aceptado queda almacenado en la reserva.

### Comunicación asíncrona

Se propone RabbitMQ para eventos de dominio:

| Evento | Publicador | Consumidor y propósito |
| --- | --- | --- |
| `VehicleChanged.v1` | Flota | Indexador interno de Flota: actualizar ficha y tarifa en la búsqueda |
| `RentalCompleted.v1` | Alquileres | Clientes: actualizar el historial del cliente |
| `ReservationCreated.v1` / `ReservationCancelled.v1` | Alquileres | Consumidor por definir según una necesidad de negocio |

Los eventos incluirán `eventId`, versión, entidad, fecha, correlación y carga mínima. El outbox persistirá el evento junto con la modificación de negocio. La entrega será al menos una vez; los consumidores controlarán duplicados por `eventId` y los cambios de ficha por versión.

Los mensajes con fallas transitorias tendrán reintentos limitados: se proponen tres con espera creciente. Los no procesables pasarán a una cola separada con motivo de rechazo, alerta y procedimiento de reproceso. Una indisponibilidad del broker retrasa las proyecciones y mantiene los eventos pendientes en outbox.

## 3. Alternativas consideradas

| Alternativa | Evaluación |
| --- | --- |
| Solo HTTP | Acopla las actualizaciones secundarias a la disponibilidad del consumidor e incumple la comunicación asíncrona requerida |
| Solo eventos | Complica la confirmación inmediata e incumple la comunicación síncrona requerida |
| Publicar después de guardar sin outbox | Permite perder un evento ante una caída entre la persistencia y la publicación |
| Reintentar creación sin clave | Puede duplicar reservas cuando se pierde la primera respuesta |

## 4. Justificación y consecuencias

HTTP permite responder inmediatamente a las validaciones que condicionan una reserva. Si una dependencia obligatoria está indisponible, la confirmación se rechaza de forma controlada. Las consultas que puedan resolverse con otras dependencias conservarán su funcionamiento.

La mensajería desacopla índice e historial mediante consistencia eventual. Introduce operación del broker, publicación de outbox, control de duplicados y reproceso de mensajes fallidos. El mecanismo requiere pruebas específicas de pérdida de respuestas y fallas parciales.

## 5. Validación prevista

- Verificar en el mock repetición idempotente y colisión de claves con cargas distintas.
- Simular en el sistema real pérdida de respuesta, caída del broker, duplicación y mensajes inválidos.
- Comprobar conservación de la reserva, actualización posterior de proyecciones y gestión de la cola de fallidos.
- Validar los tiempos asignados mediante trazas y métricas de las operaciones.

El mock verifica el contrato en un proceso. Las políticas distribuidas se implementarán y demostrarán mediante fallas controladas.

## 6. Aspectos por resolver

- Protocolos, broker y valores definitivos a partir de la implementación y las mediciones.
- Capacidad externa y políticas específicas de tiempo de espera y degradación en D9 y D10.
- Política de archivado de claves de idempotencia acordada con consumidores.
- Consumidores justificados para eventos de creación y cancelación de reserva.

## 7. Historial

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.1 | 7 de octubre de 2026 | Políticas iniciales de HTTP, idempotencia, eventos y mensajes fallidos |

Una decisión sustitutiva deberá identificar este registro como reemplazado y conservar su historia.

# ADR-016 — Comunicación entre servicios, revisión 2

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D5: comunicación |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Hito | Preparación documental de la Entrega 2 — 23 de octubre de 2026 |
| Sustituye | [ADR-005](ADR-005-comunicacion.md) |

## 1. Contexto

Las validaciones de una reserva requieren respuesta inmediata. Índice e historial toleran actualización eventual. Gimnasio y Clínica introducen contratos de otros equipos, cuyo mecanismo, identificación, campos y políticas todavía deben acordarse. Se debe preservar la distinción entre falta de evidencia, resultado negativo confirmado y resultado desconocido.

## 2. Decisión

### Interacciones síncronas e integración externa

| Interacción | Propuesta y responsabilidad |
| --- | --- |
| Web → gateway → servicio | HTTPS/JSON en el borde; autenticación en gateway y autorización del recurso en el servicio |
| Alquileres → Clientes | HTTP/JSON para habilitación de conductor |
| Gateway o Alquileres → LB interno → réplica de Flota | HTTP/JSON; ficha y tarifa autoritativas al confirmar, sin caché |
| Gimnasio → gateway → Empleados | Consulta de lista de personal; contrato y protocolo externos por acordar |
| Gimnasio → entrada acordada de Empleados | Información de uso/no uso por período; mecanismo por acordar, sin suponer eventos en RabbitMQ |
| Empleados → Clínica | Adaptador del proveedor; protocolo y operaciones según contrato por publicar |

La llamada clínica sale directamente de Empleados; no atraviesa el frontend ni el gateway propio. Alquileres deja de consumir la capacidad externa prevista de manera genérica en la entrega 1.

Se mantienen como presupuestos internos iniciales una confirmación de reserva de hasta 5 segundos y llamadas internas de hasta 1 segundo, sujetos a medición. El LB y cada llamada deben respetar el tiempo restante. Las operaciones clínicas tendrán un presupuesto específico que se acordará después de conocer el contrato y sus consecuencias; no heredan automáticamente el presupuesto de reserva.

Las lecturas podrán tener como máximo un reintento ante falla transitoria si queda presupuesto. El dueño del reintento será único por operación; la configuración del proxy no debe multiplicarlo. Errores de negocio, permisos y datos inválidos no se reintentan. Comandos con resultado desconocido no se repiten automáticamente sin una garantía idempotente acordada.

En Alquileres, la identidad de aplicación, operación y clave delimitan la idempotencia. Misma clave y carga recuperan respuesta original antes de revalidar agenda/precio; otra carga produce conflicto. Resultado y respuesta se guardan juntos. Su retención cubrirá el período admitido de reintentos y, como mínimo, la vida de la reserva. Esto permanece pendiente de implementación durable.

### Comunicación asíncrona interna

Se propone RabbitMQ para `VehicleChanged.v1` de Flota hacia el indexador y `RentalCompleted.v1` de Alquileres hacia el historial de Clientes. La publicación utiliza outbox junto al cambio de negocio. No se agregan eventos sin consumidor y propósito definidos.

La propuesta utiliza entrega al menos una vez, ACK después del efecto durable y consumidores idempotentes. Se definirán colas durables, mensajes persistentes y confirmación del publicador antes de marcar un registro de outbox como publicado. Historial y registro de mensaje procesado se escriben en la misma transacción. El indexador compara versión y aplica cambios de manera atómica para que una entrega tardía no revierta el índice.

Se proponen tres reintentos de mensaje con espera creciente y variación, por validar. Mensajes incompatibles o que agoten la política se apartan en una cola de fallidos, con responsable, alerta y reproceso controlado. La caída del broker retrasa proyecciones y conserva la intención pendiente en outbox.

## 3. Alternativas consideradas

Solo HTTP acopla proyecciones a los consumidores; solo eventos dificulta la confirmación inmediata. Publicar luego de persistir sin outbox puede perder la intención. Usar el broker interno con los equipos externos presupone una infraestructura y contrato no acordados. Reintentar desde varias capas amplifica una falla.

## 4. Consecuencias y validación

Se aceptan consistencia eventual en lecturas derivadas y operación del broker/relay. Deben probarse respuestas perdidas, duplicados, desorden, DLQ y recuperación; trazar gateway, LB, servicio y dependencia. Con Clínica/Gimnasio, validar contratos antes de afirmar compatibilidad y separar información confirmada de pendiente o indeterminada.

## 5. Referencias e historial

Material en `clases.zip`: `teoria/04-comunicacion-asincrona.md`, `teoria/06-gateway-lb-discovery.md`, `teoria/07-resiliencia.md` y `practica/clase-4-rabbitmq.md`. [D9](ADR-009-consumo-clinica.md), [D10](ADR-010-resiliencia.md), [D12](ADR-012-balanceo-carga.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Actualiza integración administrativa, destino externo y ruta balanceada |

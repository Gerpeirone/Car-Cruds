# ADR-010 — Resiliencia y resultados desconocidos

> **Complemento clínico — 23/10/2026:** [ADR-019](ADR-019-consumo-turnos-socios.md) concreta timeout 100 ms–5 s (inicial 2 s), ocho llamadas simultáneas, un intento por acción explícita y espera durable ante incertidumbre/429. Conserva resultado desconocido sin reintentos automáticos. No modifica las evidencias históricas de este documento.

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D10: resiliencia |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Protecciones y fallas locales verificadas; Circuit Breaker y ampliaciones pendientes |
| Hito | Entrega 2 — 23 de octubre de 2026 |

## 1. Contexto

Una dependencia lenta retiene recursos y puede degradar operaciones independientes. Un timeout interrumpe la espera, pero no prueba que una reserva o solicitud externa haya fracasado. La clase 7 exige políticas por operación y degradación semánticamente honesta.

## 2. Decisión

Se proponen deadline extremo a extremo, timeout por intento, **Circuit Breaker** por dependencia/operación y límites de concurrencia o **Bulkhead** donde una dependencia pueda saturar recursos. El circuito tendrá estados cerrado, abierto y semiabierto; ventana, volumen mínimo y umbrales se fijarán con evidencia. Errores de validación no cuentan como caída.

Las lecturas transitorias podrán tener un reintento acotado con backoff y variación si resta presupuesto. Habrá un dueño único del reintento; gateway, LB y cliente no multiplicarán intentos independientemente. Los comandos externos no se repetirán sin garantía idempotente acordada.

| Dependencia o falla | Comportamiento previsto |
| --- | --- |
| Clientes, tarifa de Flota o base de Alquileres indisponible | No confirmar una nueva reserva sin validación y escritura durable |
| Respuesta de reserva perdida | Consultar estado o repetir misma intención con la clave idempotente, conservando resultado desconocido hasta resolver |
| Una réplica de Flota caída | LB dirige futuras solicitudes a réplica elegible; los intentos deben caber en el presupuesto |
| LB o base compartida de Flota caída | Catálogo/confirmación pueden fallar; dos réplicas no eliminan esta dependencia |
| Redis caída | Lectura autoritativa con capacidad acotada; si no alcanza, degradación controlada |
| Solr caída | Búsqueda no disponible; lectura por identificador y confirmación independiente del índice |
| RabbitMQ o indexador indisponible | Outbox conserva pendientes; proyecciones atrasadas observables y recuperación posterior |
| Clínica lenta, error o respuesta inválida | Empleados informa pendiente/indeterminado; no inventa confirmación ni ausencia de turno |
| Información de Gimnasio ausente o incompleta | Informe identifica período sin evidencia suficiente; no deduce no uso |
| Persistencia de Empleados caída | No acusar recepción durable ni guardar un informe ficticio |

Los informes administrativos podrán conservar evidencia previa con su origen, fecha y alcance visible cuando sea válido usarla. La ausencia de evidencia reciente no se interpreta como un resultado negativo confirmado. Clínica no forma parte del camino crítico de reserva.

Mensajería tendrá ACK tras efecto durable, reintentos limitados y cola de fallidos con dueño y reproceso. Un Circuit Breaker protege llamadas; no sustituye outbox, idempotencia ni DLQ.

## 3. Alternativas consideradas

Reintentos ilimitados amplifican saturación. Un circuito único para todas las operaciones bloquea capacidades sanas. Responder éxito vacío oculta una falla. Escalar réplicas ante una base o proveedor común saturado puede empeorar la carga.

## 4. Consecuencias y validación

Se agregan estados observables y configuración que requiere calibración. La sección de avance distingue las protecciones implementadas de las propuestas pendientes. Ensayar caída/lentitud de Clínica, una réplica de Flota, Redis y broker con hipótesis y condición de detención. Registrar impacto, detección, recuperación y acciones en `docs/POSTMORTEM.md`; comprobar ausencia de duplicación, límites de espera y preservación de operaciones independientes.

## Avance de implementación — 14 de octubre de 2026

Se implementaron deadlines HTTP/AMQP, límites de concurrencia y pools, fallback Redis hacia MongoDB, outbox y colas de fallidos. El gateway rechaza sobre su límite de 64 solicitudes; la lectura autoritativa no depende de caché ni índice. Circuit Breaker no está implementado. La suite verifica cancelación/validación del transporte y publicación bloqueada; el recorrido confirma idempotencia y efecto único. El [ensayo real](../evidence/resilience.json) verificó fallback Redis, retirada/recuperación de Flota B y recuperación de RabbitMQ/Solr; [POSTMORTEM](../POSTMORTEM.md) registra impacto y acciones. El [mensaje incompatible](../evidence/message-failure.json) llegó a DLQ con motivo rejected. Quedan reproceso de negocios, alerta, fallas de fuentes compartidas y escenarios externos. Clínica/gimnasio siguen sin contrato ni ensayos.

## 5. Referencias e historial

Material en `clases.zip`: `teoria/07-resiliencia.md`, `teoria/03-cache.md`, `teoria/04-comunicacion-asincrona.md` y `practica/clase-7-balanceo-nginx.md`. [D5](ADR-016-comunicacion-v2.md), [D9](ADR-009-consumo-clinica.md), [D12](ADR-012-balanceo-carga.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera propuesta de protección y degradación por dependencia |
| 0.3 | 14 de octubre de 2026 | Implementación propia, evidencia local y pendientes registrados |

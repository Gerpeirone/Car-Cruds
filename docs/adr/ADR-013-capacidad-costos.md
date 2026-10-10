# ADR-013 — Plan de medición de capacidad y costos

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D13: capacidad y costos |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Hito | Preparación documental de la Entrega 2 — 23 de octubre de 2026 |

## 1. Contexto

No hay sistema completo, carga medida, entorno cloud seleccionado ni costos reales disponibles. La consigna exige resultados de carga, identificación del límite principal, alternativa de escalado y estimación de costos. En esta versión se define cómo obtenerlos, sin declarar el requisito cumplido.

## 2. Decisión

Medir sobre una configuración reproducible y registrar fecha, versión, recursos, volumen de datos y herramientas. Se propone comenzar con una línea base, aumentar gradualmente concurrencia/tasa y detener ante degradación sostenida o riesgo para el entorno.

| Escenario previsto | Evidencia a obtener |
| --- | --- |
| Fichas sin caché, caché fría y caliente | Latencia, hit rate, llamadas y recursos de MongoDB |
| Flota con una y dos réplicas | Distribución, throughput, conexiones y costo adicional |
| Búsqueda y cambios simultáneos | Latencia de consulta, retraso del índice y backlog |
| Reserva válida y contención por mismo período | Tiempo, conflictos esperados, exclusión y ausencia de duplicación |
| Clínica lenta o caída | Esperas/reintentos de Empleados y preservación de reservas |
| Broker detenido y recuperado | Antigüedad de outbox, tasa de recuperación y efecto en proyecciones |

La carga del proveedor deberá acordarse; no se aplicará un ensayo de saturación a otro equipo sin coordinación. Podrán usarse simuladores locales claramente identificados para aislar el comportamiento de nuestro consumidor.

Se medirán solicitudes ofrecidas y completadas, errores de negocio y técnicos por separado, p50/p95/p99, CPU, memoria, red, conexiones y límites de pools. El límite principal se identificará mediante señales y repetición controlada, no por el primer componente que parezca lento.

### Estimación posterior de costos

Crear una tabla fechada de cómputo, almacenamiento, respaldos, transferencia y servicios administrados, con región, proveedor, moneda, unidad tarifaria y supuestos. Separar recursos gratuitos/locales, costo mensual estimado y costo del ensayo. Obtener precios vigentes de fuentes oficiales al seleccionar el entorno; no se asignan importes en esta propuesta.

Comparar la configuración base con una alternativa de escalado motivada por el cuello de botella: más réplicas si el límite es Flota, ajuste de almacenamiento/pools si es la fuente, o más consumidores con capacidad suficiente si es indexación. Agregar réplicas contra una dependencia común saturada puede empeorar el resultado.

## 3. Alternativas consideradas

Copiar cifras de clase no caracteriza este proyecto. Estimar solo el precio de una VM omite almacenamiento, red y operación. Escalar todo a la vez impide atribuir la mejora al cambio. Prometer capacidad a partir del mock de memoria no describe el sistema persistente.

## 4. Consecuencias y validación

Quedan pendientes dataset ficticio, mezcla de operaciones, tasas, duración, herramientas, entorno y resultados. Los informes deberán permitir repetir la prueba, vincularla con D6/D7/D11/D12 y explicar límites/deuda. No existen todavía números de capacidad, mejoras porcentuales ni costos aprobados.

## 5. Referencias e historial

Enunciado, D13 y pruebas de carga. Material en `clases.zip`: `teoria/03-cache.md`, `teoria/05-motores-de-busqueda.md`, `teoria/06-gateway-lb-discovery.md` y `practica/clase-7-balanceo-nginx.md`. [D11](ADR-011-observabilidad.md), [D12](ADR-012-balanceo-carga.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera propuesta de medición y estimación posterior de costos |

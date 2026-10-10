# ADR-006 — Búsqueda de vehículos e índice derivado

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D6: búsqueda |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Hito | Preparación documental de la Entrega 2 — 23 de octubre de 2026 |

## 1. Contexto

El catálogo requiere paginación, filtros por características y orden por tarifa. Un motor separado permite un modelo de lectura especializado; agrega infraestructura y desfase. El volumen y beneficio de rendimiento todavía no están medidos. La consigna requiere un motor de búsqueda y la clase 5 ofrece Solr como referencia.

## 2. Decisión

Se propone Solr con un documento derivado por vehículo: identificador estable, características necesarias para búsqueda, tarifa de referencia y versión. Los campos y tipos concretos se definirán con los casos de consulta; el índice no contiene perfiles de personas ni evidencia clínica.

Flota ofrece una abstracción de búsqueda y traduce filtros/orden hacia Solr en un adaptador. Proporciona paginación, filtros de marca, categoría, transmisión y plazas, y al menos orden por tarifa con un criterio estable de desempate. La disponibilidad por período permanece en Alquileres; un resultado de búsqueda no confirma precio ni reserva.

Las modificaciones de Flota generan `VehicleChanged.v1` mediante outbox. El indexador interno aplica upsert con identificador estable y comparación atómica de versión: duplicados o versiones anteriores no revierten la proyección. Se incluyen modificaciones y bajas, con tratamiento de eliminación por definir. El índice se actualiza aunque no haya búsquedas de usuarios.

Se conserva como **objetivo propuesto** un retraso máximo de 5 segundos en operación normal, desde el cambio durable en Flota hasta su visibilidad en consulta. No es una medición obtenida. Ante atrasos se observará backlog y antigüedad; se conserva la fuente para reconstrucción.

La reconstrucción prevé un índice nuevo, carga desde Flota, incorporación de cambios durante la carga, validación y cambio controlado. No debe eliminarse trabajo confirmado ni depender de datos del broker como única fuente.

## 3. Alternativas consideradas

Consultas directas a MongoDB reducen piezas, pero no satisfacen la elección de motor requerida. Elasticsearch/OpenSearch son alternativas; Solr se propone por la práctica de cátedra. Buscar únicamente después de sincronizar bajo demanda introduce retraso indefinido. Copiar todo el modelo operativo expone datos que la consulta no necesita.

## 4. Consecuencias y validación

Se acepta consistencia eventual del descubrimiento. Si Solr cae, se informa búsqueda no disponible; ficha por identificador y confirmación pueden continuar con sus fuentes. No se presenta un error como lista vacía exitosa.

Validar filtros, paginación y orden; duplicados, desorden, bajas y reconstrucción; medir latencia y retraso extremo a extremo bajo carga y recuperación. La evidencia y configuración faltan.

## 5. Referencias e historial

Material en `clases.zip`: `teoria/05-motores-de-busqueda.md` y `practica/clase-5-solr.md`. [D3](ADR-015-persistencia-v2.md), [D5](ADR-016-comunicacion-v2.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera propuesta de búsqueda y actualización del índice |

# ADR-007 — Caché de fichas de Flota

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D7: caché |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Caché/versiones y reducción de lecturas de ficha medidas; comparación completa pendiente |
| Hito | Entrega 2 — 23 de octubre de 2026 |

## 1. Contexto

Las fichas públicas de autos pueden consultarse repetidamente. La necesidad observable y la mejora aún deben medirse; incorporar caché no acredita por sí solo el requisito académico. La tarifa final y la agenda son datos críticos que deben validarse en sus fuentes.

## 2. Decisión

Se propone Redis como caché distribuida compartida por las dos réplicas de Flota, con patrón **cache-aside**. Ante hit vigente, la lectura exploratoria usa la ficha; ante miss, expiración o expulsión, Flota consulta MongoDB y puede poblar la caché. Se propone una abstracción/decorador que preserve la interfaz de lectura.

La clave incluirá identificador de vehículo, versión de formato y toda dimensión que cambie la respuesta. Se evitarán datos personalizados. El TTL configurado de 60 segundos acota la vigencia de las fichas; no acredita una mejora de rendimiento ni garantiza consistencia instantánea.

Al modificar, primero se confirma la fuente y luego se invalida la clave. Se requiere tratar la carrera entre un lector anterior y la invalidación, mediante una estrategia de versión y actualización condicional por definir y probar. El TTL limita vigencia temporal, pero no resuelve por sí solo todas las carreras.

La lectura autoritativa utilizada por Alquileres al confirmar accede a la fuente y omite Redis incluso si una entrada sigue vigente. La disponibilidad temporal pertenece a Alquileres y no se cachea para autorizar reservas. Los informes de personal tampoco dependen de esta caché.

Ante Redis lento o caído se limita la espera y se intenta la fuente con concurrencia acotada. Si la fuente no admite la carga, se informa degradación o indisponibilidad controlada; no se provoca una cascada por fallback ilimitado. Los errores de caché se miden separados de misses.

Redis tiene una capacidad máxima configurada de 64 MiB y expulsión volatile-lru. La defensa adicional de una clave popular mediante agrupación de lecturas y variación del TTL permanece prevista.

## 3. Alternativas consideradas

Caché local es simple, pero diverge entre réplicas y requiere invalidación múltiple. Memcached es una alternativa vista en clase; Redis se mantiene como propuesta del proyecto. Write-behind agrega riesgo de pérdida y no se acepta para el dato operativo. No cachear evita desfase, pero requiere comprobar la carga de lecturas.

## 4. Consecuencias y validación

Comparar la misma carga sin caché, con caché fría y caliente: p50/p95/p99, accesos a MongoDB, hit rate, errores, memoria y expulsiones. Probar cambio de tarifa, reinicio, caída y carreras de invalidación. Ninguna mejora ni cumplimiento se declara comprobado antes de obtener evidencia.

## Avance de implementación — 14 de octubre de 2026

Redis compartido implementa cache-aside con TTL de 60 s en fichas. Tras confirmar MongoDB se invalida mediante una marca de versión y operaciones Lua condicionales; un llenado anterior no puede reemplazar una versión más nueva si la invalidación fue exitosa y se conservan los datos de Redis. La política volatile-lru expulsa claves con TTL y preserva marcas sin TTL. Una prueba en Redis aislado registró 69 expulsiones, conservó V2 y rechazó el llenado V1. Las marcas consumen memoria sin expirar: retención/limpieza quedan pendientes. Un vaciado o pérdida de Redis elimina la garantía; OOM o indisponibilidad conducen a fallback con concurrencia acotada, no a un precio autoritativo cacheado. La lectura para Alquileres siempre omite Redis. Agrupación por clave y variación de TTL no se implementaron. Se ejecutó la comparación caliente frente a fallback con Redis detenido; la línea base sin dependencia de caché y la fase fría siguen pendientes. Ver [pruebas](../evidence/tests-summary.json).

## Medición real — 14 de octubre de 2026

Fuentes: [cache-warm.json](../evidence/cache-warm.json), [cache-fallback.json](../evidence/cache-fallback.json) y [cache-metrics.json](../evidence/cache-metrics.json). Revisión del backend: `8327268583782805270ff1a6156afa24a29db5c2`. Árbol documental modificado, Go 1.26.1, generador y Docker en la misma máquina.

Se consultó la misma ficha `GET /v1/vehicles/auto-001` por gateway, con carga cerrada de cinco segundos por etapa. Primero se calentó la caché; después se repitió con Redis detenido y se lo restauró. Todas las respuestas medidas fueron `200`, sin errores de transporte ni JSON inválido.

| Condición | Concurrencia | HTTP 200 | Éxitos/s | p95 (ms) |
| --- | ---: | ---: | ---: | ---: |
| Caliente | 1 | 1412 | 282,34 | 5,982 |
| Caliente | 8 | 7048 | 1403,34 | 11,736 |
| Redis detenido / fallback | 1 | 33 | 6,44 | 158,661 |
| Redis detenido / fallback | 8 | 7361 | 1470,15 | 7,883 |

| Delta de métricas, ambas réplicas | Hits | Misses | Errores Redis | Lecturas de ficha a MongoDB |
| --- | ---: | ---: | ---: | ---: |
| Caliente | 8461 | 0 | 0 | 0 |
| Fallback | 0 | 0 | 7395 | 7395 |

Los contadores se consultaron secuencialmente y se sumaron por réplica. Incluyen una comprobación previa por fase y podrían incluir tráfico simultáneo; describen lecturas de ficha instrumentadas, no todas las operaciones de MongoDB.

El resultado respalda la **reducción de accesos a la fuente** con caché caliente. No demuestra una mejora universal de latencia: a concurrencia ocho, el fallback fue más rápido en esta ejecución. El costo de detectar la falla Redis, las conexiones del driver, el conjunto pequeño de datos y los recursos compartidos no fueron aislados. Redis detenido tampoco equivale a desactivar la caché sin costo de dependencia.

La corrección de instrumentación clasifica error de transporte antes de miss. Tras un error de lectura, se consulta la fuente una sola vez y no se intenta escribir nuevamente en Redis. Las pruebas focales verificaron puerto/cliente cerrado, miss normal y conexión TCP silenciosa acotada por el contexto. La fase fría y una comparación sostenida con caché deshabilitada continúan pendientes.

## 5. Referencias e historial

Material en `clases.zip`: `teoria/03-cache.md`, `practica/clase-3-cache.md` y `teoria/06-gateway-lb-discovery.md`. [D3](ADR-015-persistencia-v2.md), [D10](ADR-010-resiliencia.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera propuesta de caché compartida, frescura y medición |
| 0.3 | 14 de octubre de 2026 | Implementación propia, evidencia local y pendientes registrados |

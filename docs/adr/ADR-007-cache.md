# ADR-007 — Caché de fichas de Flota

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D7: caché |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Hito | Preparación documental de la Entrega 2 — 23 de octubre de 2026 |

## 1. Contexto

Las fichas públicas de autos pueden consultarse repetidamente. La necesidad observable y la mejora aún deben medirse; incorporar caché no acredita por sí solo el requisito académico. La tarifa final y la agenda son datos críticos que deben validarse en sus fuentes.

## 2. Decisión

Se propone Redis como caché distribuida compartida por las dos réplicas de Flota, con patrón **cache-aside**. Ante hit vigente, la lectura exploratoria usa la ficha; ante miss, expiración o expulsión, Flota consulta MongoDB y puede poblar la caché. Se propone una abstracción/decorador que preserve la interfaz de lectura.

La clave incluirá identificador de vehículo, versión de formato y toda dimensión que cambie la respuesta. Se evitarán datos personalizados. El TTL inicial de 60 segundos es un objetivo/configuración propuesta, no un resultado ni garantía de consistencia instantánea.

Al modificar, primero se confirma la fuente y luego se invalida la clave. Se requiere tratar la carrera entre un lector anterior y la invalidación, mediante una estrategia de versión y actualización condicional por definir y probar. El TTL limita vigencia temporal, pero no resuelve por sí solo todas las carreras.

La lectura autoritativa utilizada por Alquileres al confirmar accede a la fuente y omite Redis incluso si una entrada sigue vigente. La disponibilidad temporal pertenece a Alquileres y no se cachea para autorizar reservas. Los informes de personal tampoco dependen de esta caché.

Ante Redis lento o caído se limita la espera y se intenta la fuente con concurrencia acotada. Si la fuente no admite la carga, se informa degradación o indisponibilidad controlada; no se provoca una cascada por fallback ilimitado. Los errores de caché se miden separados de misses.

Se fijarán capacidad máxima, expiración y defensa ante demanda simultánea de una clave popular, como agrupación de lecturas por clave y variación acotada del TTL.

## 3. Alternativas consideradas

Caché local es simple, pero diverge entre réplicas y requiere invalidación múltiple. Memcached es una alternativa vista en clase; Redis se mantiene como propuesta del proyecto. Write-behind agrega riesgo de pérdida y no se acepta para el dato operativo. No cachear evita desfase, pero requiere comprobar la carga de lecturas.

## 4. Consecuencias y validación

Comparar la misma carga sin caché, con caché fría y caliente: p50/p95/p99, accesos a MongoDB, hit rate, errores, memoria y expulsiones. Probar cambio de tarifa, reinicio, caída y carreras de invalidación. Ninguna mejora ni cumplimiento se declara comprobado antes de obtener evidencia.

## 5. Referencias e historial

Material en `clases.zip`: `teoria/03-cache.md`, `practica/clase-3-cache.md` y `teoria/06-gateway-lb-discovery.md`. [D3](ADR-015-persistencia-v2.md), [D10](ADR-010-resiliencia.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera propuesta de caché compartida, frescura y medición |

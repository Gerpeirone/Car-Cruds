# Ensayo de carga sostenida, caché y frescura de búsqueda

| Campo | Valor |
| --- | --- |
| Fecha de diseño | 10 de noviembre de 2026 |
| Estado | Procedimiento definido; resultados se registran después de ejecutar con dependencias reales |
| Alcance | Componentes propios en entorno local o CI; no genera tráfico hacia Gimnasio ni Hospital |
| Decisiones relacionadas | [D6](adr/ADR-006-busqueda.md), [D7](adr/ADR-007-cache.md) y [D13](adr/ADR-013-capacidad-costos.md) |

## Objetivo

Medir durante intervalos sostenidos y comparables la lectura de una ficha con caché fría, caché caliente y Redis indisponible. En paralelo, verificar que la búsqueda indexada mantenga una cota de frescura menor a cinco segundos para alta, actualización y baja mientras recibe tráfico.

El ensayo utiliza carga cerrada: cada trabajador inicia una solicitud después de terminar la anterior. Por ello informa rendimiento observado para una concurrencia fija, no una tasa de llegadas independiente.

## Escenario y criterios

La ruta de caché es `GET /v1/vehicles/auto-001`, mediante el gateway y con identidad local de cliente. Las tres fases usan la misma concurrencia y duración:

1. **Arranque frío:** Redis está saludable y se elimina únicamente la entrada de datos de `auto-001`; se conserva la marca de generación. Las primeras solicitudes cargan MongoDB y las siguientes pueden usar Redis. Esta fase mide el costo de calentamiento, no una línea base permanentemente sin caché.
2. **Caliente:** se confirma una lectura previa y se ejecuta la misma carga con la entrada vigente.
3. **Fallback:** se detiene Redis, se ejecuta la misma carga y se restaura la dependencia aun si el ensayo falla. El circuit breaker puede omitir intentos posteriores y Flota consulta MongoDB con capacidad acotada.

Por fase se registran solicitudes, éxitos/s, p50, p95, p99, estados HTTP, errores de transporte y del JSON, además de los deltas de `fleet_cache_requests_total` y `fleet_source_reads_total`. El criterio mínimo es cero errores de transporte/JSON y solo HTTP 200. Los percentiles y accesos a fuente se interpretan comparativamente; no se fija un SLO de latencia sin una línea base de infraestructura acordada.

Para búsqueda se mantiene una etapa de 30 segundos sobre `/v1/vehicles/search` mientras `cmd/smoke -history` crea un vehículo exclusivo, espera su alta en Solr, actualiza tarifa/versión y finalmente lo desactiva. El ensayo falla si la carga termina antes que el recorrido, de modo que las tres transiciones ocurren con tráfico concurrente. Cada transición falla si no es visible dentro de cinco segundos desde antes de la escritura HTTP. El criterio es: carga sin errores, recorrido completo satisfactorio y tres líneas `PASS índice` con `cotaDesdeEscrituraMs < 5000`.

## Ejecución

Con el conjunto Compose saludable y `.env` generado:

```bash
bash scripts/measure-sustained.sh 30 8
```

Los argumentos son segundos por fase (10–30) y concurrencia (1–64). Los archivos sin secretos se escriben en `output/`: `sustained-cache-*.json`, `sustained-search.json`, `sustained-smoke.log` y `sustained-summary.json`. `output/` permanece fuera de Git; una evidencia seleccionada se copia a `docs/evidence/` solo después de revisar revisión, entorno y resultados.

## Límites

- Generador y servicios pueden compartir equipo; CPU, memoria y red deben registrarse antes de comparar máquinas.
- Un único vehículo popular muestra calentamiento y reducción de lecturas, no el comportamiento de un catálogo grande.
- Detener Redis incluye fallback y circuit breaker; no equivale a compilar el sistema sin caché.
- La frescura se mide desde antes de la escritura, por lo que es una cota superior del retraso desde commit.
- Una ejecución corta en CI funciona como regresión reproducible. La evidencia de defensa debe usar 30 segundos por fase y registrar el entorno definitivo.

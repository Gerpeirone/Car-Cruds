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

Los argumentos son segundos por fase (10–30) y concurrencia (1–64). Los archivos sin secretos se escriben en `output/`: `sustained-cache-*.json`, `sustained-search-<n>.json` (una ventana de 30 s por archivo), `sustained-smoke.log` y `sustained-summary.json`. `output/` permanece fuera de Git; una evidencia seleccionada se copia a `docs/evidence/` solo después de revisar revisión, entorno y resultados.

### Corrección del 10 de noviembre de 2026

El diagnóstico local de la falla intermitente de CI encontró tres problemas en el script:

- **Clave de caché incorrecta.** El script borraba `fleet:vehicle:{auto-001}:data`, pero Flota usa `fleet:v1:<base>:vehicle:{id}:data` (`cmd/fleet/main.go`). El borrado no tenía efecto: la fase fría no era fría y la entrada real expiraba (TTL de 60 s, que los aciertos no renuevan) en un momento arbitrario. Si eso ocurría durante la fase caliente, varios trabajadores leían MongoDB a la vez y la validación `sourceDetail = 0` fallaba. Localmente se reprodujo con 30 s por fase: entre 2 y 8 lecturas de fuente en la fase caliente, todas juntas al expirar la entrada. Ahora el script usa la clave real (la base se lee de `MONGODB_DATABASE` en el contenedor de Flota), renueva la entrada antes de cada fase y falla si la clave no existe tras una lectura.
- **Carga de búsqueda limitada a 30 s.** `cmd/load` admite como máximo 30 s por ejecución. Si el recorrido de frescura tardaba más, por ejemplo en un runner lento, el script fallaba. Ahora encadena ventanas de 30 s hasta que el recorrido termina.
- **Diagnóstico insuficiente.** El resumen imprime los contadores y las cotas antes de validar, y la CI registra los archivos de `output/` y los logs de Flota, Solr y Redis si el paso falla. La medición se separó de `checkshared` en un paso propio.

**Estado de verificación:** las correcciones se probaron por partes en un stack local aislado. La corrida completa de 30 s con la clave corregida se interrumpió antes de terminar y la CI todavía no se ejecutó con estos cambios, así que no hay resultados sostenidos registrados. Que la clave incorrecta sea la causa de la falla en CI es una hipótesis consistente con lo observado: no se confirmó con los logs del runner.

**Deuda observada:** al expirar una entrada, cada solicitud concurrente lee MongoDB (no hay *single-flight*). El impacto está acotado por el límite de 32 lecturas de fuente simultáneas por réplica.

## Reproducir la línea base sin caché

En un proyecto Compose aislado, mantener Redis activo. Guardar una superposición temporal fuera de Git con este contenido:

```yaml
services:
  fleet-a:
    environment:
      REDIS_ADDR: ""
  fleet-b:
    environment:
      REDIS_ADDR: ""
```

Con las credenciales de laboratorio configuradas, ejecutar `go run ./cmd/load -gateway "$ORIGEN_LOCAL" -path /v1/vehicles/auto-001 -role client -concurrency 8 -duration 20s` en cada fase. Ver los resultados en [D7](adr/ADR-007-cache.md#comparación-equivalente-de-la-candidata--10112026). Para la fase fría, esperar el vencimiento de la ficha y comprobar que no existe en Redis. Repetir inmediatamente para la fase caliente. Para la línea base, añadir la superposición temporal a los mismos archivos Compose, ejecutar `up -d --no-deps --wait fleet-a fleet-b` y reiniciar `fleet-lb`. Repetir el comando de carga y tomar deltas de lecturas fuente y caché en ambas réplicas. Restaurar las réplicas con los archivos originales y reiniciar el balanceador de laboratorio.

## Límites

- Generador y servicios pueden compartir equipo; CPU, memoria y red deben registrarse antes de comparar máquinas.
- Un único vehículo popular muestra calentamiento y reducción de lecturas, no el comportamiento de un catálogo grande.
- Detener Redis incluye fallback y circuit breaker; no equivale a compilar el sistema sin caché.
- La frescura se mide desde antes de la escritura, por lo que es una cota superior del retraso desde commit.
- Una ejecución corta en CI funciona como regresión reproducible. La evidencia de defensa debe usar 30 segundos por fase y registrar el entorno definitivo.

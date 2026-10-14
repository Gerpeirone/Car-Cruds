# Servicio Flota

Flota implementa una arquitectura en capas: handlers HTTP, servicio de aplicación y repositorios/adaptadores. MongoDB es la fuente de fichas y tarifas; Redis es una copia temporal; Solr es una proyección de búsqueda. La disponibilidad por período pertenece a Alquileres.

## Ejecutables y configuración

- `cmd/fleet`: API stateless, utilizada por dos réplicas detrás del LB.
- `cmd/indexer`: relay de outbox e indexador, con HTTP privado de readiness y métricas provisto por la plataforma.

La configuración se suministra por entorno: `MONGODB_URI`, `MONGODB_DATABASE` (por defecto `fleet`), `REDIS_ADDR`, `REDIS_PASSWORD`, `RABBITMQ_URL`, `SOLR_URL` (URL del core `vehicles`), `PORT`/`HTTP_ADDR` y credenciales `AUTH_*_TOKEN` de la plataforma. El arranque reproducible del sistema se documenta en la raíz.

`SEED_DEMO=true` crea, si no existen, `auto-001` y `auto-002`: datos ficticios con tarifas diarias de 5.000.000 y 6.500.000 centavos ARS. Reiniciar no reemplaza fichas ya existentes.

## API implementada

| Operación | Propósito | Roles |
| --- | --- | --- |
| `GET /v1/vehicles` | Lista desde Mongo, filtros y paginación | admin, client, service |
| `GET /v1/vehicles/{id}` | Ficha mediante cache-aside | admin, client, service |
| `GET /v1/vehicles/{id}/quote` | Tarifa autoritativa desde Mongo, sin Redis ni Solr | admin, service |
| `GET /v1/vehicles/search` | Búsqueda real en Solr | admin, client, service |
| `POST /v1/vehicles` | Crear ficha y su evento atómicamente | admin, service |
| `PUT /v1/vehicles/{id}` | Reemplazar atributos con versión esperada | admin, service |
| `POST /v1/vehicles/reindex` | Encolar reconstrucción desde Mongo | admin, service |

La ficha incluye `vehicleId`, `brand`, `model`, `category`, `transmission`, `seats`, `dailyRateMinor`, `currency`, `active`, `version` y `updatedAt`. `active` es habilitación administrativa, no disponibilidad de agenda. La tarifa usa enteros; moneda inicial ARS y transmisión `manual` o `automatic`.

Lista y búsqueda aceptan `page` (desde 1), `pageSize` (1–100), filtros `brand`, `category`, `transmission`, `seats` y `sort=price_asc|price_desc`, con desempate por identificador. Búsqueda admite `q`. Las respuestas incluyen `items`, `page`, `pageSize` y `total`. Solo el administrador puede incluir inactivos en la lista con `includeInactive=true`.

PUT requiere la `version` obtenida previamente; una escritura concurrente produce 409. Crear usa versión 1; tarifas, plazas, identificación y atributos se validan antes de escribir.

## Persistencia, caché e indexación

La modificación y el evento `VehicleChanged.v1` se guardan en **un mismo documento MongoDB**, sin transacción distribuida ni requisito de replica set. El outbox contiene como máximo 128 pendientes por vehículo. Al llenarse se rechaza la escritura con 503 `OUTBOX_FULL`; no se confirma el cambio ni se descartan eventos.

El relay publica mensajes persistentes en el exchange durable `carcruds.events` y espera confirmación del broker antes de retirar exactamente el `eventId`. La escritura de red y la confirmación comparten un presupuesto de tres segundos; un deadline del socket y su cierre por cancelación limitan también una publicación bloqueada dentro del SDK. Ante una entrega incierta se cierra la sesión y el evento queda pendiente. Una caída puede duplicar la publicación. El indexador usa ACK manual, recarga el estado actual desde Mongo y aplica upsert con versión de origen y CAS de Solr: un evento o rebuild atrasado no revierte una ficha más reciente.

El consumidor tiene tres intentos acotados con espera creciente y variación; los inválidos y los que agotan la política van a `fleet.indexer.dlq`. Una reconstrucción encola de nuevo el estado de la fuente, incluidos inactivos; estos permanecen fuera de los resultados públicos. El comando devuelve 202 con la cantidad encolada, que no significa indexación ya terminada.

Redis compartido aplica TTL de 60 segundos. Scripts Lua conservan un watermark de versión, invalidan después de escribir y rechazan el fill de un lector anterior a una invalidación ya registrada. Una invalidación fallida se mide; una ficha exploratoria puede conservarse hasta su expiración. **Quote siempre omite la caché.** Las fallas de Redis permiten consultar la fuente con un máximo de 32 lecturas concurrentes por réplica; superar esa capacidad produce 503, evitando fallback ilimitado.

La política `volatile-lru` puede expulsar fichas con TTL y preserva los watermarks sin expiración. `allkeys-lru` permitiría perder esa defensa de versión bajo presión de memoria. La capacidad sigue limitada: si los marcadores no permiten liberar espacio suficiente, Redis rechaza nuevas escrituras y el servicio utiliza el fallback acotado. Esta elección se apoya en teoría 3/práctico 3 de caché y en la [documentación oficial de evicción de Redis](https://redis.io/docs/latest/develop/reference/eviction/).

Si Solr falla, búsqueda devuelve indisponibilidad en lugar de una lista vacía exitosa. Ficha, quote y escritura permanecen independientes del índice. Broker caído conserva outbox y puede causar backpressure; MongoDB caído impide operaciones autoritativas. Readiness de Flota depende de Mongo, mientras readiness del indexador también comprueba broker y Solr.

## Verificación y observabilidad

Los tests unitarios comprueban validaciones, quote sin caché, fallback acotado, errores HTTP, respuesta inválida de búsqueda y CAS de versiones. Las integraciones se ejecutan contra dependencias reales cuando se suministran:

| Variable de test | Dependencia |
| --- | --- |
| `TEST_MONGODB_URI` | Mongo: versiones concurrentes, outbox atómico/acotado y reapertura |
| `TEST_REDIS_ADDR`, `TEST_REDIS_PASSWORD` | Redis: fill atrasado e invalidación fuera de orden |
| `TEST_SOLR_URL` | Solr: no retroceso, filtros, orden, paginación y visibilidad |
| `TEST_REDIS_EVICTION_ADDR`, `TEST_REDIS_EVICTION_PASSWORD` | Redis dedicado y descartable: presión real, expulsión de datos TTL, preservación de watermark V2 y rechazo de fill V1 |

Sin esas variables, el test correspondiente se identifica como omitido; eso no acredita integración. Cada test usa datos aislados y limpia exclusivamente los que creó.

La prueba de evicción requiere otra instancia de loopback, distinta de `TEST_REDIS_ADDR`, con `volatile-lru` y memoria máxima de hasta 8 MiB ya configurados (en la verificación local se utiliza 2 MiB). Lee `CONFIG GET` e `INFO`, sin alterar la configuración ni ejecutar un vaciado global. Crea claves con TTL hasta confirmar al menos 64 evicciones, comprueba la defensa de versión sin depender de otra ficha cacheada y elimina sólo sus claves. No ejecutar esta prueba de presión contra el Redis compartido del sistema.

```powershell
go test -race ./services/fleet ./cmd/fleet ./cmd/indexer
```

Métricas privadas de la plataforma: `fleet_cache_requests_total`, `fleet_source_reads_total`, `fleet_cache_invalidation_failures_total`, `fleet_outbox_published_total`, `fleet_index_events_total` y `fleet_index_write_delay_seconds`. Esta última mide hasta la escritura del índice y excluye la apertura del searcher; el objetivo de cinco segundos de visibilidad se verifica mediante consultas, no se deduce de ese histograma.

La instrumentación HTTP aporta instancia, correlación y trazas. La distribución y retirada de réplicas se verifican en el sistema desplegado; tener dos ejecutables no acredita por sí solo balanceo ni alta disponibilidad.

## Referencias

[Arquitectura](../../docs/ARCHITECTURE.md), [D5](../../docs/adr/ADR-016-comunicacion-v2.md), [D6](../../docs/adr/ADR-006-busqueda.md), [D7](../../docs/adr/ADR-007-cache.md) y [D12](../../docs/adr/ADR-012-balanceo-carga.md). Material de cátedra: Repository, caché, RabbitMQ y Solr dentro de `clases.zip`.

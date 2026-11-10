# Guion de la presentación grupal

Fecha de preparación: 9 de noviembre de 2026. Presentación: 11 o 13 de noviembre, según el sorteo. Este guion se basa en lo que hay implementado y verificado en `feature/entrega-3-pendientes`. Lo que todavía depende de terceros aparece como tal.

## 1. Preparación

**El día anterior**

1. Integrar los PR aprobados (#8 y #9) y desplegar en la VM: `bash scripts/deploy-vm.sh`. La VM debe usar Ubuntu 24.04 con kernel 6.8; ver [DESPLIEGUE](DESPLIEGUE.md).
2. Desde otra máquina, verificar la capacidad compartida: `go run ./cmd/checkshared -url https://<PUBLIC_HOST>`.
3. Revisar la interfaz publicada en un navegador real:
   ```bash
   npm --prefix scripts/ui-review ci
   BASE_URL=https://<PUBLIC_HOST> BROWSER="<ruta a Chrome o Edge>" ADMIN_TOKEN=... CLIENT_TOKEN=... node scripts/ui-review/review.mjs
   ```
   Tiene que terminar con «0 con problemas».
4. Si hay acceso del hospital, cargar `CLINIC_BASE_URL` y `CLINIC_API_KEY` en el `.env` de la VM y volver a correr el script.
5. Ensayar el recorrido de la sección 2 completo, con tiempo.

**Media hora antes**

- Abrir pestañas con: interfaz del cliente, panel del operador, Grafana (túnel `ssh -L 3000:127.0.0.1:3000 <vm>`), Jaeger (`ssh -L 16686:127.0.0.1:16686 <vm>`) y este documento.
- Tener las credenciales de laboratorio a mano. **No proyectarlas ni pegarlas en el chat.**
- Comprobar que el tablero muestre 7 servicios respondiendo y 0 alertas.

## 2. Recorrido sugerido (unos 15 minutos)

| # | Quién | Qué se muestra | Cómo |
| --- | --- | --- | --- |
| 1 | — | Contexto y contenedores | Figuras 1 y 2 de [ARCHITECTURE](ARCHITECTURE.md): cuatro servicios, gateway, NGINX con dos réplicas de Flota, Caddy y los dos equipos externos |
| 2 | — | Ingreso del cliente | `https://<host>/acceso` con `cli-001`. Si el token es inválido, mostrar el 401 claro |
| 3 | — | Búsqueda indexada | Buscar por marca; aplicar filtros (categoría, transmisión, plazas), ordenar por precio y paginar. Es Solr detrás de `/v1/vehicles/search` |
| 4 | — | Ficha con caché | Abrir un auto. La ficha sale de Redis con fallback a MongoDB (ver sección 3, caché) |
| 5 | — | Reserva idempotente | Elegir fechas, cotizar y confirmar. Mostrar el código. Opcional: recargar durante la confirmación para mostrar la recuperación con la misma clave |
| 6 | — | Consulta y cancelación | «Mis reservas» por código y cancelar con diálogo |
| 7 | — | Panel del operador | `/operador/acceso`: flota (alta y edición con versión), reserva por código, retiro y devolución |
| 8 | — | Evento de dominio | La devolución publica `RentalCompleted.v1` por outbox; Clientes lo consume una sola vez. Mostrar el historial del cliente o la traza en Jaeger |
| 9 | — | Funcionalidad compartida | Personal → Empleados: lista y alta. Contar que el gimnasio consume `GET /v1/employees` con su propio token. Si ya lo consumieron, mostrar su evidencia en Membresías |
| 10 | — | Capacidad del proveedor | Personal → Turnos médicos: crear una solicitud y enviarla al hospital. Sin credenciales del hospital, mostrar que la interfaz informa `NOT_CONFIGURED` sin inventar un turno, y el contrato probado en CI |
| 11 | — | Resiliencia en vivo | Con carga de fondo, detener Redis (`docker compose stop redis`). En Grafana: breaker «Abierto», alerta activa, bypass en lugar de hits y lecturas a MongoDB. Volver a iniciar Redis: la alerta se resuelve y el breaker se cierra |
| 12 | — | Limitaciones | Sección 9 de [ARCHITECTURE](ARCHITECTURE.md): qué es externo, qué es deuda aceptada y qué queda para la defensa |

Completar la columna «Quién» al repartir el recorrido.

## 3. Requisitos del enunciado y su evidencia

| Requisito de la presentación | Qué mostrar | Evidencia |
| --- | --- | --- |
| Frontend completo, desde el ingreso hasta la funcionalidad compartida | Pasos 2 a 10 | [EVIDENCIA-FRONTEND](EVIDENCIA-FRONTEND.md), [PANEL-OPERADOR](PANEL-OPERADOR.md), [PERSONAL-OPERADOR](PERSONAL-OPERADOR.md); 33 pantallas sin violaciones WCAG AA ([informe](evidence/ui-review-2026-11-09.json)) |
| Entrada única con autenticación, direccionamiento y control de tráfico | 401/403 por rol; Caddy deriva `/v1/*` y el gateway enruta por prefijo; admisión simultánea limitada (`GATEWAY_MAX_IN_FLIGHT`, 503 al agotarse) | [D13](adr/ADR-013-capacidad-costos.md), [DESPLIEGUE](DESPLIEGUE.md), `cmd/gateway` |
| Al menos tres servicios operativos | Clientes, Flota (2 réplicas), Alquileres y Empleados | Tablero: «Servicios respondiendo = 7»; `go run ./cmd/smoke -history` (72 solicitudes) |
| Capacidad propia real, en la nube con URL pública | Lista de empleados para el gimnasio | `checkshared` contra la URL. **Pendiente de la VM** |
| Capacidad del proveedor en el flujo principal | Turnos del hospital desde Empleados | Prueba de contrato contra el código original del proveedor en CI; [COMUNICACION-HOSPITAL](COMUNICACION-HOSPITAL.md). **Acceso real pendiente** |
| Al menos dos tipos de almacenamiento | PostgreSQL (Clientes, Alquileres, Empleados), MongoDB (Flota), Redis (caché), Solr (índice) | [D3](adr/ADR-015-persistencia-v2.md), Compose |
| Caché de lectura con comparación antes/después | Ficha de vehículo | [Sin breaker](evidence/cache-comparison-2026-11-09.json) y [con breaker](evidence/cache-breaker-2026-11-09.json): con caché, 0 lecturas a MongoDB; sin Redis, p50 de 155 ms antes del breaker y 1,6 ms después |
| Dos estilos o patrones internos reconocibles | Capas (Clientes, Flota) y Hexagonal (Alquileres, Empleados) | [ADR-002](adr/ADR-002-arquitectura-interna.md); mostrar los puertos en `services/rentals` y `services/employees/application` |
| Evento por mensajería, consumidor idempotente y mensajes no procesables | `RentalCompleted.v1` y `VehicleChanged.v1` por outbox; Clientes con efecto único; colas `customers.rental-history.failed` y `fleet.indexer.dlq` | [message-failure.json](evidence/message-failure.json), [D5](adr/ADR-016-comunicacion-v2.md) |
| Búsqueda indexada con paginación, filtros, orden y reconstrucción | `/v1/vehicles/search`; `POST /v1/vehicles/reindex` desde Flota en el panel | [D6](adr/ADR-006-busqueda.md); en el smoke, la búsqueda refleja un cambio en unos 1,5 s |
| Decisiones verificadas con evidencia del sistema | Tablero, trazas y mediciones | [evidence/](evidence/), [simulacro de alerta](evidence/alert-drill-2026-11-09.json) |
| Limitaciones y deuda técnica en ARCHITECTURE | Sección 9 | [ARCHITECTURE §9](ARCHITECTURE.md#9-limitaciones-conocidas-y-deuda-técnica-aceptada) |

## 4. Preguntas probables

- **¿Qué pasa si se cae Redis?** Las fichas se leen de MongoDB. Con el circuit breaker, tras cinco fallas no se espera más el deadline y se prueba Redis cada 2 s. La invalidación siempre se intenta, para no servir una versión vieja cuando Redis vuelve. Hay alerta y panel en Grafana.
- **¿Cómo evitan reservas duplicadas?** `Idempotency-Key` obligatoria: la misma clave con el mismo cuerpo recupera la respuesta guardada. Las fechas ocupadas se rechazan en la transacción de Alquileres (`409 VEHICLE_UNAVAILABLE`). El precio esperado se compara con la tarifa autoritativa (`409 PRICE_CHANGED`).
- **¿Por qué la ficha va a la caché y la cotización no?** La cotización fija el precio de una reserva y lee siempre la fuente. La ficha tolera hasta 60 s de TTL y está protegida por marcas de versión.
- **¿Qué garantiza el consumidor de eventos?** Entrega al menos una vez. El efecto en Clientes es único por identificador de evento, y un mensaje incompatible va a la cola de fallidos sin bloquear la cola.
- **¿Por qué el navegador no llama al hospital?** La llamada al proveedor sale del microservicio responsable (Empleados), como pide el enunciado. La clave del hospital nunca llega al navegador.
- **¿Por qué la CI usa MongoDB 7?** El runner de GitHub pasó al kernel 7.0 y MongoDB 8 se niega a iniciar en kernels 6.19 o superiores. Solo la CI usa 7.0; local y VM usan 8 ([ARCHITECTURE §9](ARCHITECTURE.md#9-limitaciones-conocidas-y-deuda-técnica-aceptada)).

## 5. Plan B

- **Si la VM no está lista:** presentar con el entorno local (`docker compose -f compose.yaml -f compose.frontend.yaml -f compose.observability.yaml up -d --wait`) y decirlo explícitamente. La URL pública es un requisito y no debe simularse.
- **Si falla una dependencia externa durante la demo:** mostrar cómo la interfaz y el tablero lo informan. Es parte de lo que se evalúa.
- **Si se corta la red:** usar las capturas de [evidence/img](evidence/img) y los informes JSON.

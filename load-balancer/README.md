# Balanceador interno de Flota

| Campo | Valor |
| --- | --- |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | NGINX configurado; distribución local verificada |
| Configuración | [nginx.conf](nginx.conf), cargada por Docker Compose |

## Responsabilidad y distribución

NGINX Open Source se ubica en la red privada entre gateway/Alquileres y dos réplicas equivalentes de Flota, `fleet-a` y `fleet-b`. Ambas comparten MongoDB y Redis; el estado de negocio no depende de memoria local. El upstream utiliza round-robin con pesos iguales y conexiones persistentes.

Gateway mantiene autenticación y encaminamiento de API; Flota autoriza y ejecuta las operaciones. El balanceador no contiene reglas de alquiler ni ofrece una entrada pública alternativa.

## Configuración de fallas y tiempos

Se utiliza detección **pasiva**: `max_fails=3` y `fail_timeout=30s`. No hay una comprobación activa periódica ni discovery dinámico. Las dependencias Compose usan `restart: true`: una recreación controlada de Flota reinicia el LB para renovar sus direcciones; no es un mecanismo de discovery ni cubre todas las fallas espontáneas. La distinción entre comprobaciones pasivas de OSS y activas de NGINX Plus se describe en la [documentación oficial](https://docs.nginx.com/nginx/admin-guide/load-balancer/http-health-check/).

La conexión tiene límite de 1 s; lectura del proxy de 3 s por período de inactividad y envío de 1 s. Se permiten hasta dos intentos ante error o timeout, limitados por `proxy_next_upstream_timeout=1s`. Esos valores no reemplazan el deadline total del cliente ni amplían el presupuesto de confirmación de una reserva. Alquileres controla su espera para obtener la tarifa autoritativa.

Se propagan autorización, request ID y contexto de traza. Un encabezado de instancia permite observar el destino sin afirmar que NGINX instrumenta spans propios.

## Evidencia y límites

El recorrido real mostró respuestas de ambas réplicas desde el gateway. El [ensayo de retirada/recuperación](../docs/evidence/resilience.json) registró tres 503 iniciales y 17 respuestas 200 en A de 20 solicitudes; B reapareció 29 315,18 ms desde el sondeo tras su restauración. La caída del balanceador y la fuente común todavía no se ensayaron. La detección pasiva puede admitir fallas iniciales antes de apartar una réplica; no promete una transición sin errores.

Un balanceador, un gateway y almacenes compartidos permanecen como puntos únicos de falla. Dos réplicas no constituyen alta disponibilidad integral. La configuración de laboratorio no prueba capacidad del despliegue cloud.

Referencias: [D12](../docs/adr/ADR-012-balanceo-carga.md), [D11](../docs/adr/ADR-011-observabilidad.md), [arquitectura](../docs/ARCHITECTURE.md) y [estado](../docs/ESTADO-IMPLEMENTACION.md). Material en `clases.zip`: `teoria/06-gateway-lb-discovery.md` y `practica/clase-7-balanceo-nginx.md`.

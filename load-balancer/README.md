# Balanceador interno de Flota

| Campo | Valor |
| --- | --- |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Implementación | Diseño documental; sin configuración NGINX ni contenedores ejecutables |

## Responsabilidad y ubicación

Se propone NGINX Open Source en la red interna entre el gateway y dos réplicas stateless de Flota. Alquileres utiliza el mismo punto lógico para consultar ficha y tarifa autoritativas. No llama directamente a una réplica.

El gateway mantiene autenticación, políticas de entrada y routing de API. El LB selecciona una réplica; no contiene reglas de alquiler ni sustituye la autorización del servicio. Las réplicas no ofrecen accesos públicos alternativos.

## Distribución y detección previstas

Las réplicas tendrán configuración equivalente, almacenamiento MongoDB y caché Redis compartidos, sin sesión de negocio en memoria local. Se propone round-robin con pesos iguales.

El diseño utilizará detección **pasiva** de fallas en solicitudes reales; la detección activa periódica documentada para NGINX Plus no se presenta como una capacidad implementada por OSS. [Referencia oficial](https://docs.nginx.com/nginx/admin-guide/load-balancer/http-health-check/).

Los umbrales, tiempos del proxy y recuperación serán explícitos y medidos. No se multiplicarán reintentos entre proxy y cliente. El cambio de destino para comandos necesita una política idempotente de aplicación; el LB no la proporciona.

## Límites y evidencia necesaria

Un LB único, gateway único y fuentes compartidas siguen siendo puntos de falla. Dos réplicas toleran la caída de una instancia de Flota; no acreditan alta disponibilidad completa. No hay discovery dinámico ni readiness activa automáticamente configurados en esta propuesta.

La demostración futura deberá mostrar destino por instancia mediante logs/métricas, tráfico desde gateway y Alquileres, caída/retirada y recuperación de una réplica, y comportamiento ante caída del propio LB. Readiness, si se incorpora, será interna y diferenciada de liveness; su formato, ruta y mecanismo se definirán al implementar.

## Referencias

[D12 — Balanceo](../docs/adr/ADR-012-balanceo-carga.md), [arquitectura C4](../docs/ARCHITECTURE.md) y [D11 — Observabilidad](../docs/adr/ADR-011-observabilidad.md).

Material en `clases.zip`: `teoria/06-gateway-lb-discovery.md` y `practica/clase-7-balanceo-nginx.md`. La configuración de laboratorio es una referencia; sus valores de ejemplo no son mediciones de Car Cruds.

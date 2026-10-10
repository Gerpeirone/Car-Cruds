# ADR-012 — Balanceo interno de Flota con NGINX

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D12: balanceo de carga |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Hito | Preparación documental de la Entrega 2 — 23 de octubre de 2026 |

## 1. Contexto

La consigna exige al menos un servicio con dos o más instancias, distribución y detección de instancias no disponibles. Flota tiene lecturas frecuentes y permite réplicas sin sesión local. El gateway gobierna el acceso público; seleccionar una réplica es otra responsabilidad.

## 2. Decisión

Se propone un contenedor **NGINX Open Source** en la red interna, entre el gateway y dos réplicas stateless de Flota. Alquileres consulta la dirección lógica del mismo LB para obtener ficha y tarifa; no selecciona una réplica directamente.

Flota comparte MongoDB y Redis entre sus réplicas y no guarda sesiones o negocio en RAM local. El indexador sigue siendo un trabajador separado con consumo idempotente. Se propone round-robin con pesos iguales como punto inicial, porque las réplicas tendrán configuración equivalente. NGINX documenta la distribución ponderada por round-robin en upstream. [Referencia oficial](https://nginx.org/en/docs/http/ngx_http_upstream_module.html).

El gateway mantiene autenticación, límites de entrada y routing de API. Flota autoriza operaciones con identidad propagada por un canal interno confiable. Las réplicas y el LB no se exponen como vías públicas alternativas.

### Detección y recuperación

Se propone **detección pasiva** de fallas en solicitudes reales, mediante `max_fails` y `fail_timeout`; tolerancias concretas se definirán y medirán. La documentación oficial distingue esta capacidad de OSS de los chequeos activos periódicos de NGINX Plus. No se afirma que OSS ejecute automáticamente esos chequeos activos. [Health checks oficiales](https://docs.nginx.com/nginx/admin-guide/load-balancer/http-health-check/).

La ausencia de tráfico no permite al mecanismo pasivo descubrir una caída por adelantado. La recuperación debe observarse con solicitudes posteriores y sus resultados. La comprobación de readiness de la plataforma, si se incorpora, será una capacidad distinta y debe definirse explícitamente.

El proxy tendrá tiempos y eventual cambio de destino acotados por el presupuesto del caso de uso. Para evitar amplificación, se definirá un único dueño de reintentos. No se habilitarán reenvíos de comandos sin idempotencia de aplicación; la opción `non_idempotent` no constituye esa garantía. [Proxy oficial](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_next_upstream).

## 3. Alternativas consideradas

Integrar LB dentro del gateway reduce componentes, pero mezcla la demostración de políticas públicas con selección de réplica. Least-connections puede evaluarse con operaciones de distinta duración; no se elige sin datos. Afinidad por IP es innecesaria con servicio stateless. Un registro dinámico de discovery agrega operación sin una necesidad demostrada para dos réplicas iniciales.

## 4. Consecuencias y límites

El LB único, gateway, MongoDB y demás dependencias continúan siendo puntos únicos de falla. Dos réplicas de Flota aportan tolerancia a una instancia caída; no constituyen alta disponibilidad completa. La topología inicial será explícita; cambios de direcciones/nombres deberán actualizarse mediante un procedimiento documentado, sin afirmar discovery automático.

## 5. Validación pendiente

Identificar instancia en logs/métricas internas y demostrar tráfico desde gateway y Alquileres distribuido entre ambas. Detener una réplica, registrar errores iniciales, tiempo de retirada y continuidad; restaurarla y observar recuperación. Probar caída del LB y saturación común de la base. La configuración, contenedores y ensayos aún no existen.

## 6. Referencias e historial

Material en `clases.zip`: `teoria/06-gateway-lb-discovery.md` y `practica/clase-7-balanceo-nginx.md`. Fuentes oficiales consultadas el 10 de octubre de 2026. [D10](ADR-010-resiliencia.md), [D11](ADR-011-observabilidad.md). [Componente](../../load-balancer/README.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera propuesta de LB interno y dos réplicas de Flota |

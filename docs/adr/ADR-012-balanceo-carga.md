# ADR-012 — Balanceo interno de Flota con NGINX

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D12: balanceo de carga |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Distribución, retirada y recuperación verificadas localmente |
| Hito | Entrega 2 — 23 de octubre de 2026 |

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

### Parámetros iniciales propuestos

Valores de partida para calibrar con mediciones; no son resultados de Car Cruds.

| Parámetro | Valor inicial | Propósito |
| --- | --- | --- |
| Algoritmo | Round-robin, pesos iguales | Réplicas con configuración equivalente |
| `max_fails` / `fail_timeout` | 3 fallas / 30 s | Retirar temporalmente una réplica que falla en solicitudes reales |
| Tiempo de conexión al backend | 1 s como límite de fase para catálogo exploratorio | No representa el plazo completo de una validación de reserva |
| Tiempo de lectura del backend | 3 s entre lecturas para catálogo exploratorio | Limitar inactividad de lectura; no equivale a un deadline de operación |
| Validación de Flota desde Alquileres | Presupuesto total de como máximo 1 s, reducido al tiempo restante de la confirmación | Incluye conexión, envío, lectura y eventual reintento, según D5 |
| Cambio de destino ante falla | Como máximo un intento adicional de lectura segura, solo si resta presupuesto | Un único dueño del reintento; no reiniciar el deadline ni reenviar comandos sin idempotencia |
| Conexiones persistentes al backend | 32 | Reducir el costo de abrir conexiones por solicitud |
| Identificación | Registro de acceso en JSON con réplica atendida e identificador de solicitud | Evidencia de distribución para D11 |
| Exposición | Gateway como entrada externa de API; LB y réplicas internos | Evitar vías de acceso alternativas |

Los límites por fase no reemplazan el deadline del caso de uso. `proxy_connect_timeout` limita el establecimiento de conexión; `proxy_read_timeout` mide el intervalo entre lecturas sucesivas, no la transferencia completa. [Semántica oficial de NGINX](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_read_timeout).

Los valores de conexión de 1 s y lectura de 3 s corresponden únicamente al perfil exploratorio del catálogo, cuyo deadline total debe calibrarse. No se aplican como presupuesto de una validación autoritativa de reserva. Alquileres limitará toda esa llamada a Flota a `mínimo(1 s, tiempo restante de la confirmación de 5 s)`, incluyendo todos sus intentos. LB y réplica deberán respetar el mismo plazo restante; ninguna fase o cambio de destino reiniciará el reloj. Alquileres aplica cancelación y límite de espera; la configuración actual y los límites de los ensayos se registran en la sección de avance.

Se elegirá un único dueño del eventual reintento, cliente o proxy. Los límites de intentos y de tiempo para pasar al siguiente upstream se configurarán en coherencia con ese presupuesto; no sustituyen el deadline completo de la operación. Si el primer intento consume el tiempo disponible, no se inicia otro. [Controles oficiales de cambio de upstream](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_next_upstream_timeout).

## 3. Alternativas consideradas

Integrar LB dentro del gateway reduce componentes, pero mezcla la demostración de políticas públicas con selección de réplica. Least-connections puede evaluarse con operaciones de distinta duración; no se elige sin datos. Afinidad por IP es innecesaria con servicio stateless. Un registro dinámico de discovery agrega operación sin una necesidad demostrada para dos réplicas iniciales.

## 4. Consecuencias y límites

El LB único, gateway, MongoDB y demás dependencias continúan siendo puntos únicos de falla. Dos réplicas de Flota aportan tolerancia a una instancia caída; no constituyen alta disponibilidad completa. La topología inicial será explícita; cambios de direcciones/nombres deberán actualizarse mediante un procedimiento documentado, sin afirmar discovery automático.

## 5. Validación pendiente

Identificar instancia en logs/métricas internas y demostrar tráfico desde gateway y Alquileres distribuido entre ambas. Detener una réplica, registrar errores iniciales, tiempo de retirada y continuidad; restaurarla y observar recuperación. Probar caída del LB y saturación común de la base. Medir una validación lenta y otra con cambio de réplica: el total debe caber en el presupuesto restante de D5, sin multiplicar intentos entre capas. La configuración y los contenedores existen; la evidencia y los ensayos pendientes se registran en la sección de avance.

## Avance de implementación — 14 de octubre de 2026

[nginx.conf](../../load-balancer/nginx.conf) configura round-robin, dos destinos, max_fails=3/fail_timeout=30s y keepalive 32. El cambio de destino está limitado a dos intentos y 1 s total; el cliente Alquileres aplica su propio deadline de 1 s sin reintento adicional. Las solicitudes exploratorias poseen perfil separado; read timeout de 3 s es un límite de inactividad, no el plazo total de confirmación. El recorrido real utilizó ambas réplicas. El [ensayo](../evidence/resilience.json) detuvo B: de 20 solicitudes hubo tres fallas iniciales 503 y 17 éxitos en A, con 16 consecutivos al final. B reapareció 29 315,18 ms después del inicio del sondeo tras restaurarla, conforme al fail_timeout de 30 s. La detección pasiva admite esos errores iniciales; no es una transición sin fallas. Compose configura restart:true en las dependencias fleet-a/fleet-b del LB: al recrear las réplicas mediante una operación Compose, reinicia NGINX para renovar sus direcciones. El rebuild y arranque posteriores resultaron saludables. Esta coordinación no es discovery dinámico ni cubre todo reinicio espontáneo. No hay chequeo activo NGINX. Ver [componente](../../load-balancer/README.md).

## 6. Referencias e historial

Material en `clases.zip`: `teoria/06-gateway-lb-discovery.md` y `practica/clase-7-balanceo-nginx.md`. Fuentes oficiales consultadas el 10 de octubre de 2026; semántica de timeouts contrastada el 12 de octubre de 2026. [D5](ADR-016-comunicacion-v2.md), [D10](ADR-010-resiliencia.md), [D11](ADR-011-observabilidad.md). [Componente](../../load-balancer/README.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera propuesta de LB interno y dos réplicas de Flota |
| 0.3 | 14 de octubre de 2026 | Implementación propia, evidencia local y pendientes registrados |

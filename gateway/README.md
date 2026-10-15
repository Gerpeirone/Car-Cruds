# API Gateway

| Campo | Valor |
| --- | --- |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Implementado; recorrido propio verificado localmente |
| Ejecutable | `cmd/gateway`, Go con `net/http` |
| Entrada local | `http://127.0.0.1:8080`, configurable mediante `APP_PORT` |

## Responsabilidad

El gateway autentica aplicaciones mediante Bearer tokens configurados, encamina las rutas de Clientes, Flota, Alquileres y Empleados y propaga correlación HTTP. Flota se alcanza mediante el balanceador interno; los demás destinos son servicios propios. La autorización del recurso y las reglas de negocio se verifican en cada servicio.

Su máximo de solicitudes simultáneas funciona como límite de concurrencia; al excederlo responde `503`. Se configura con `GATEWAY_MAX_IN_FLIGHT` (entero entre 1 y 4096, por defecto 64); un valor inválido impide el arranque. No implementa una cuota por usuario ni un limitador de tasa con respuesta `429`. Los errores de proxy `502/504` se traducen a indisponibilidad controlada `503`.

La verificación de disponibilidad consulta en paralelo a los cuatro servicios con identidad interna. La autenticación, registros JSON, request ID, trazas y métricas proceden de `internal/platform`. Las rutas usan etiquetas estables para evitar que los identificadores de negocio multipliquen las series de métricas.

## Alcance de acceso

El gimnasio puede leer la lista mínima de empleados activos. Las operaciones administrativas, candidatos y datos de clientes no forman parte de ese permiso. Los acuerdos externos y el entorno público permanecen pendientes; los tokens identifican aplicaciones del laboratorio, no cuentas personales.

La futura llamada al hospital saldrá directamente de Empleados mediante su adaptador. El gateway propio no actúa como proxy saliente al proveedor. No se implementó una interfaz HTML ni terminación TLS pública.

## Ejecución y evidencia

`compose.yaml` configura los destinos en red privada y publica la entrada únicamente en loopback. [cmd/smoke](../cmd/smoke/README.md) ejercita el conjunto desde este punto de entrada. El recorrido, los roles y una traza real se registran en [ESTADO-IMPLEMENTACION](../docs/ESTADO-IMPLEMENTACION.md).

Consultar [arquitectura](../docs/ARCHITECTURE.md), [D5](../docs/adr/ADR-016-comunicacion-v2.md), [D8](../docs/adr/ADR-017-contrato-empleados.md) y [balanceador](../load-balancer/README.md). Los directorios `src/` conservan la estructura documental inicial; el código operativo está en `cmd/gateway`.

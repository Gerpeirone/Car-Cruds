# ADR-013 — Capacidad observada y estimación de costos

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D13: capacidad y costos |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Medición local inicial y estimación; validación de capacidad sostenida pendiente |
| Hito | Entrega 2 — 23 de octubre de 2026 |

## 1. Contexto y decisión

D13 requiere resultados de carga, identificación del límite principal, alternativa de escalado y estimación de costos en la entrega 2. Se registra una primera medición del flujo operativo de disponibilidad y se conserva el plan de ampliar escenarios. La defensa individual no posterga estos cuatro resultados.

La prueba utilizó `cmd/load`, Go 1.26.1, Docker con 12 CPU lógicas y 7,61 GiB de memoria disponible, dos réplicas de Flota y dos autos activos del conjunto ficticio. Generador y sistema comparten la máquina. La revisión de código es `7f5c52b02d1418d7ac37d26644fe237115f079b2`; el informe indica árbol modificado por documentación, scripts y evidencia en preparación. El código ejecutado y la configuración del entorno corresponden a esa revisión.

Se envió `GET /v1/availability` por gateway con identidad de cliente, para un auto y período constantes. El modelo es de **carga cerrada**: cada trabajador envía la siguiente solicitud cuando termina la anterior, sin pausa. Cada etapa solicita cinco segundos; el tiempo informado incluye terminar las solicitudes en curso. No es una tasa de llegada independiente del tiempo de respuesta.

## 2. Resultados reales

Fuentes: [load-availability.json](../evidence/load-availability.json) y [load-resource-samples.json](../evidence/load-resource-samples.json).

| Concurrencia | HTTP 200 | HTTP 503 | Éxitos/s | p95 total (ms) | p95 de éxitos (ms) |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1 | 757 | 0 | 151,21 | 9,926 | 9,926 |
| 8 | 4205 | 0 | 839,23 | 16,963 | 16,963 |
| 32 | 4502 | 0 | 897,86 | 76,127 | 76,127 |
| 64 | 7418 | 0 | 1471,16 | 80,522 | 80,522 |
| 128 | 4654 | 9982 | 916,97 | 137,901 | 171,408 |

No se registraron errores de transporte ni JSON inválido. Con 128 trabajadores, **68,2 %** de respuestas fueron `503`. La tasa total de esa etapa incluye rechazos rápidos y no debe presentarse como rendimiento útil. Los percentiles generales también incluyen rechazos; por eso se informa por separado el p95 de los éxitos.

### Límite principal observado

El primer límite explícito observado es la admisión de **64 solicitudes simultáneas del gateway**, que protege los servicios mediante un bulkhead. Es un límite configurado y una respuesta de protección; este ensayo no identifica la capacidad máxima de PostgreSQL, MongoDB o Flota.

Las muestras de CPU incluyen picos aproximados de gateway 195 %, Alquileres 220 %, MongoDB 141 % y réplicas Flota 110/107 %. Son muestras de utilización multicore, no promedios sostenidos ni prueba causal de un cuello de botella. Generador y servidor compiten por recursos; la duración breve, el calentamiento mínimo y un único auto/período limitan la representatividad.

### Alternativa de escalado

Evaluar una segunda réplica de gateway detrás de un balanceador o ajustar su admisión junto con los pools y límites de sus dependencias. Se debe comparar una sola modificación por ensayo y observar latencias, éxito útil, CPU y conexiones. Aumentar el límite sin comprobar capacidad downstream puede trasladar la saturación y empeorar la respuesta. Esta alternativa está justificada por el límite encontrado; su mejora todavía no se midió.

## 3. Estimación de laboratorio cloud

Consulta de precios: **14 de octubre de 2026**. Se considera Linux/Unix con IPv4 pública en Amazon Lightsail, una VM y todos los componentes propios dentro de ella. La región definitiva se seleccionará con el grupo; la transferencia incluida debe verificarse para esa región. No hay despliegue ni contratación cloud realizada.

| Configuración | Memoria | vCPU | SSD incluido | Transferencia anunciada | Precio base mensual |
| --- | ---: | ---: | ---: | ---: | ---: |
| Base de laboratorio | 8 GB | 2 | 160 GB | 5 TB | USD 44 |
| Alternativa vertical | 16 GB | 4 | 320 GB | 6 TB | USD 84 |

Los paquetes y precios se obtuvieron de [AWS: paquetes Lightsail](https://docs.aws.amazon.com/lightsail/latest/userguide/amazon-lightsail-bundles.html). Son precios base del paquete, con almacenamiento y transferencia incluidos sujetos a las condiciones/región del proveedor. Se excluyen respaldos/snapshots, transferencia excedente, impuestos, servicios externos y operación. La estimación no asigna una tarifa al gimnasio ni al hospital.

Lightsail utiliza capacidad de CPU con ráfagas y una línea base propia del paquete; debe contemplarse para cargas sostenidas. [AWS: rendimiento base de CPU](https://docs.aws.amazon.com/lightsail/latest/userguide/baseline-cpu-performance.html). El entorno local de 12 CPU lógicas no equivale a una VM de 2 o 4 vCPU: no se extrapolan sus solicitudes/s ni sus SLO a estos paquetes.

Una VM única constituye un laboratorio económico, con puntos únicos de falla y sin alta disponibilidad. El uso real de memoria, almacenamiento, respaldos y carga deberá comprobarse antes de seleccionar el paquete. La alternativa de USD 84 es una opción de recursos; no es una mejora de rendimiento verificada.

## 4. Validación pendiente y plan de ampliación

Esta medición inicial cubre los cuatro resultados solicitados por D13 con límites explícitos. No acredita un SLO sostenido, capacidad máxima del sistema ni escalabilidad cloud. Se deben repetir etapas más largas con calentamiento, mezcla de lecturas/escrituras, mayor volumen de datos, generador separado y una variación controlada de admisión o réplicas.

La comparación de fichas caliente/fallback sobre backend posterior está registrada en [D7](ADR-007-cache.md); muestra reducción de accesos, con latencia no uniforme. No sustituye la carga de disponibilidad ni una línea base sin dependencia Redis.

También se ampliarán:

- Fichas sin caché, con caché fría y caliente: latencias, llamadas a MongoDB, hit rate y recursos comparables.
- Una y dos réplicas: distribución, rendimiento útil y efecto sobre dependencias comunes.
- Búsqueda y cambios simultáneos: frescura desde commit y backlog.
- Reservas y contención: conflictos esperados, ausencia de duplicados y tiempo de respuesta.
- Broker detenido/recuperado: conservación de outbox y aplicación eventual.
- Clínica y gimnasio: solo con contratos y coordinación; no se carga a otro equipo sin acuerdo.

Un simulador local, si se utiliza, se identificará como tal y no acreditará integración con el proveedor real. Las mejoras porcentuales se registrarán únicamente después de ejecutar comparaciones equivalentes.

## 5. Referencias e historial

[Enunciado](../../Enunciado%20TP%20Final.md), D13 y sección 6.3. Material en `clases.zip`: teoría 03, 05 y 06; práctica 07. [D7](ADR-007-cache.md), [D11](ADR-011-observabilidad.md) y [D12](ADR-012-balanceo-carga.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Plan inicial de medición y costos |
| 0.3 | 14 de octubre de 2026 | Resultados locales, límite de admisión, alternativa y estimación AWS con supuestos |

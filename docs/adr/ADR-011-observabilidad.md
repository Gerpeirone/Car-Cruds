# ADR-011 — Observabilidad, primera versión

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D11: observabilidad |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Hito | Preparación documental de la Entrega 2 — 23 de octubre de 2026 |
| Madurez | Primera versión documental; sin instrumentación ni mediciones |
## 1. Contexto

Debe poder reconstruirse una operación entre gateway, LB, servicios, almacenes y proveedor sin inspección manual aislada. Se necesita evidencia de balanceo, carga, retraso de proyecciones y fallas; todavía no hay instrumentación de los servicios reales.

## 2. Decisión

Se proponen logs estructurados, métricas, trazas distribuidas y un tablero. Cada operación tendrá correlación interna. La propagación a Clínica depende de que su contrato la admita; en caso contrario se registra el tramo local sin afirmar una traza del proveedor.

Los logs incluirán servicio, instancia, operación, duración, resultado y correlación. No contendrán tokens, credenciales, diagnósticos, documentos completos ni cargas personales del consumidor. El identificador de instancia será técnico; las dimensiones de métricas evitarán identificadores de persona, vehículo y solicitud de alta cardinalidad.

| Indicador | Uso previsto |
| --- | --- |
| Tasa, latencia p50/p95/p99 y errores por operación | Estado general y degradación |
| Solicitudes por réplica y destino del LB | Comprobar distribución y retirada de réplicas |
| Timeouts, reintentos, circuitos y rechazos por límite | Evidencia de contención de fallas |
| Hit rate, errores de Redis y accesos a MongoDB | Comparar beneficio y costo de caché |
| Retraso de índice, antigüedad de outbox y cola/DLQ | Observar proyecciones y recuperación |
| Conflictos e idempotencia de reservas | Distinguir rechazo de negocio de falla técnica |
| Integraciones administrativas pendientes/indeterminadas | Hacer visible evidencia faltante por dependencia, sin exponer datos personales |

El tablero agrupará salud del flujo de reserva, catálogo/búsqueda, mensajería, réplicas y flujo administrativo. Los traces de reserva atravesarán gateway → Alquileres → Clientes y LB/Flota; los del caso clínico mostrarán gateway → Empleados → adaptador de Clínica, con la limitación externa indicada.

### Objetivo de servicio y alertas propuestos

Como objetivo inicial de latencia, las confirmaciones de reserva deberían resolverse dentro del presupuesto de 5 segundos de D5 en operación normal. Conflictos de negocio esperados no se clasificarán como indisponibilidad. Ventana, percentil objetivo y porcentaje de disponibilidad aún deben fijarse con una línea base.

Se propone alertar ante ausencia de réplicas elegibles de Flota, acumulación sostenida de outbox/DLQ, retraso de índice superior al objetivo de 5 segundos y aumento sostenido de errores/timeouts de un flujo. Ventanas, umbrales y destinatarios se definirán antes del ensayo. Esto es un plan, no alertas activas.

## 3. Alternativas consideradas

Solo logs no acredita distribución ni tendencias. Solo métricas no explica una operación concreta. Trazas sin correlación o que expongan datos personales agregan costo y poca utilidad. No se selecciona una plataforma por sus paneles sin definir primero las preguntas operativas.

## 4. Consecuencias y validación

Se requiere seleccionar colector, almacenamiento, visualización, retención y muestreo. No hay resultados, capturas operativas ni SLO demostrado. La entrega 2 necesita logs correlacionados y primera traza del flujo implementado: esta decisión documental no sustituye esa evidencia.

Validar correlación, ausencia de datos sensibles, métricas por instancia, activación de alerta y recorrido de una falla controlada. Conservar evidencia reproducible del ensayo.

## 5. Referencias e historial

Material en `clases.zip`: `teoria/04-comunicacion-asincrona.md`, `teoria/06-gateway-lb-discovery.md`, `teoria/07-resiliencia.md` y `practica/clase-7-balanceo-nginx.md`. [D5](ADR-016-comunicacion-v2.md), [D12](ADR-012-balanceo-carga.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera versión de señales, tablero y objetivos por validar |

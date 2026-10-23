# Registro de decisiones arquitectónicas

| Campo | Valor |
| --- | --- |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Decisiones vigentes con implementación propia y evidencia parcial |

El número ADR identifica el registro histórico; **D1–D13** identifica la decisión obligatoria del enunciado. Un reemplazo utiliza un archivo nuevo y conserva el anterior. Implementación propia, evidencia local y acuerdo externo son estados diferentes; no se atribuye aprobación a ninguno por la existencia del documento.

## Decisiones vigentes

| Decisión | Registro | Estado y límite principal |
| --- | --- | --- |
| D1: límites | [ADR-014](ADR-014-limites-servicios-v2.md) | Cuatro propietarios implementados; integración administrativa externa pendiente |
| D2: arquitectura interna | [ADR-002](ADR-002-arquitectura-interna.md) | Capas y Hexagonal reconocibles en código; adaptadores externos pendientes |
| D3: persistencia | [ADR-015](ADR-015-persistencia-v2.md) | PostgreSQL/MongoDB propios, outbox y reinicio verificados |
| D4: consistencia | [ADR-004](ADR-004-consistencia-concurrencia.md) | Agenda e idempotencia PostgreSQL; concurrencia y repetición verificadas |
| D5: comunicación | [ADR-016](ADR-016-comunicacion-v2.md) | HTTP y eventos internos operativos; contratos externos pendientes |
| D6: búsqueda | [ADR-006](ADR-006-busqueda.md) | Cambios y reconstrucción inicial verificados; frescura desde commit e índice sombra pendientes |
| D7: caché | [ADR-007](ADR-007-cache.md) | Versiones/expulsión y reducción de lecturas medidas; latencia no uniforme, fría/baseline/retención pendientes |
| D8: capacidad propia | [ADR-017](ADR-017-contrato-empleados.md) | API de empleados implementada provisionalmente; aceptación/publicación/consumo pendientes |
| D9: consumo | [ADR-009](ADR-009-consumo-clinica.md) | Puertos definidos; contrato y adaptador proveedor pendientes |
| D9: alta de gimnasio | [ADR-018](ADR-018-alta-gimnasio.md) | Adaptador POST 1.0.1 e intención durable implementados; URL/credencial/prueba conjunta pendientes; hospital sigue separado en ADR-009 |
| D10: resiliencia | [ADR-010](ADR-010-resiliencia.md) | Protecciones parciales; Circuit Breaker y ampliación de ensayos pendientes |
| D11: observabilidad | [ADR-011](ADR-011-observabilidad.md) | Logs y primera traza HTTP reales; AMQP/tablero/alertas pendientes |
| D12: balanceo | [ADR-012](ADR-012-balanceo-carga.md) | NGINX, distribución, retirada/recuperación reales; LB y fuentes únicas siguen como límites |
| D13: capacidad/costos | [ADR-013](ADR-013-capacidad-costos.md) | Carga inicial, límite de admisión, alternativa y estimación fechada; no SLO sostenido ni escala cloud verificados |

D4 no integra el listado específico de ADR de entrega 2, pero es obligatoria en el proyecto final y ya cuenta con implementación propia. [ESTADO-IMPLEMENTACION](../ESTADO-IMPLEMENTACION.md) concentra evidencia y límites; [ENTREGA-2](../ENTREGA-2.md) vincula las decisiones con el hito.

## Registros reemplazados

| Registro histórico | Sucesor | Motivo |
| --- | --- | --- |
| [ADR-001](ADR-001-limites-de-servicios.md) | ADR-014 | Incorporación de Empleados |
| [ADR-003](ADR-003-persistencia.md) | ADR-015 | Persistencia del nuevo propietario |
| [ADR-005](ADR-005-comunicacion.md) | ADR-016 | Integración de personal separada de Alquileres |
| [ADR-008](ADR-008-contrato-propio.md) | ADR-017 | Lista de empleados como capacidad para el gimnasio |

El mock y contrato históricos se conservan. Los acuerdos externos del gimnasio de Pablo y hospital de Octi están pendientes en [INTEGRACIONES](../INTEGRACIONES.md); las referencias a Clínica identifican ese hospital.

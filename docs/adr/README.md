# Registro de decisiones arquitectónicas

| Campo | Valor |
| --- | --- |
| Versión documental | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Decisiones propuestas para la entrega 2; validación operativa pendiente |

El número de archivo ADR identifica el registro histórico; **D1–D13** identifica la decisión obligatoria del enunciado. Un reemplazo utiliza un nuevo archivo y conserva el anterior. Propuesto no significa implementado ni aprobado por los equipos externos.

## Decisiones vigentes del diseño

| Decisión | Registro actual | Estado y verificación necesaria |
| --- | --- | --- |
| D1: límites | [ADR-014](ADR-014-limites-servicios-v2.md) | Cuatro servicios; validar límites y ausencia de bases compartidas |
| D2: arquitectura interna | [ADR-002](ADR-002-arquitectura-interna.md) | Estilos y patrones definidos; verificar dependencias al programar |
| D3: persistencia | [ADR-015](ADR-015-persistencia-v2.md) | Almacenes propios; integración y recuperación pendientes |
| D4: consistencia y concurrencia | Sin registro definitivo | El mock verifica exclusión en un proceso; solución transaccional real pendiente |
| D5: comunicación | [ADR-016](ADR-016-comunicacion-v2.md) | Políticas propuestas; acuerdos externos y ensayos pendientes |
| D6: búsqueda | [ADR-006](ADR-006-busqueda.md) | Índice por cambios; medición y reconstrucción pendientes |
| D7: caché | [ADR-007](ADR-007-cache.md) | Necesidad y política propuestas; comparación antes/después pendiente |
| D8: capacidad propia | [ADR-017](ADR-017-contrato-empleados.md) | Borrador de lista de empleados; acuerdo y servidor pendientes |
| D9: consumo | [ADR-009](ADR-009-consumo-clinica.md) | Puerto de clínica propuesto; contrato externo pendiente |
| D10: resiliencia | [ADR-010](ADR-010-resiliencia.md) | Protecciones propuestas; falla controlada y postmortem pendientes |
| D11: observabilidad | [ADR-011](ADR-011-observabilidad.md) | Primera versión; logs/trazas/tablero/alertas reales pendientes |
| D12: balanceo | [ADR-012](ADR-012-balanceo-carga.md) | Flota con dos réplicas; demostración y mediciones pendientes |
| D13: capacidad y costos | [ADR-013](ADR-013-capacidad-costos.md) | Plan de medición; resultados, límite y costos pendientes |

D4 no se exige en el listado de ADR de la entrega 2, pero sigue siendo una decisión obligatoria para el proyecto final. Los registros D6/D7/D9/D10/D12/D13 actuales son preparación documental; sus validaciones no se consideran aprobadas por existir el archivo.

## Registros reemplazados

| Registro histórico de entrega 1 | Sucesor | Motivo |
| --- | --- | --- |
| [ADR-001](ADR-001-limites-de-servicios.md) | ADR-014 | Incorporación de Empleados y cambio del flujo externo |
| [ADR-003](ADR-003-persistencia.md) | ADR-015 | Propiedad de datos del nuevo servicio |
| [ADR-005](ADR-005-comunicacion.md) | ADR-016 | Comunicación de personal y aislamiento de alquileres |
| [ADR-008](ADR-008-contrato-propio.md) | ADR-017 | Gimnasio consume empleados en lugar de reservas |

La evidencia de la etapa anterior y el contrato/mock de alquileres permanecen conservados. Los acuerdos pendientes se registran en [INTEGRACIONES.md](../INTEGRACIONES.md).

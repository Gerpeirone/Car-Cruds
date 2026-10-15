# Evidencia técnica de la entrega 2

| Campo | Valor |
| --- | --- |
| Versión documental | 0.1 |
| Fecha de corte | 14 de octubre de 2026 |
| Estado | Evidencia local de componentes propios |
| Entorno | Windows/Go 1.26.1, Docker Compose; configuración local fuera de Git |

## 1. Índice y alcance

| Artefacto | Operación o ensayo | Resultado acreditado |
| --- | --- | --- |
| [tests-summary.json](evidence/tests-summary.json) | Suite real de componentes con `TEST_*` y detector de carreras; vet | 60 pruebas aprobadas, ninguna omitida; salidas 0; presión Redis aislada aprobada |
| [integration-summary.json](evidence/integration-summary.json) | `go run ./cmd/smoke -history`, 13 de octubre | 75 solicitudes; roles, ambas réplicas, lista mínima, reservas y evento único |
| Recorrido final, 14 de octubre | `go run ./cmd/smoke -history`, revisión `8327268` | 78 solicitudes, salida 0 y limpieza; `RentalCompleted` único observado desde sondeo |
| [trace-reservation.json](evidence/trace-reservation.json) | Exportación de Jaeger de reserva real | Siete spans enlazados, cuatro servicios propios |
| [runtime-persistence.json](evidence/runtime-persistence.json) | Reinicio y reconstrucción inicial de índice | Reserva completada/empleado inactivo conservados; fuente e índice activos equivalentes |
| [resilience.json](evidence/resilience.json) | Réplica B, Redis, RabbitMQ y Solr detenidos/restaurados | `PASS` con errores iniciales explícitos y recuperación |
| [message-failure.json](evidence/message-failure.json) | Mensaje incompatible propio a consumidor Clientes | Rechazo y cola de fallidos verificados sin tocar mensajes ajenos |
| [load-availability.json](evidence/load-availability.json) | Cinco etapas de disponibilidad, concurrencias 1/8/32/64/128 | Rendimiento útil, percentiles y códigos; límite de admisión del gateway |
| [cache-warm.json](evidence/cache-warm.json), [cache-fallback.json](evidence/cache-fallback.json) y [cache-metrics.json](evidence/cache-metrics.json) | Misma ficha, caliente/fallback y métricas sumadas | Reducción de lecturas de ficha; latencia no uniforme; Redis restaurado |
| [load-resource-samples.json](evidence/load-resource-samples.json) | Muestras Docker durante carga | Recursos asignados y muestras de CPU/memoria; no capacidad cloud |

La suite final de 60 pruebas, sin omisiones y con vet satisfactorio, se ejecutó después de la corrección Redis y la reconstrucción del entorno sobre `8327268`. El recorrido final aprobó 78 solicitudes; ejecución `10ead0c0c22142bc968ec9e8ef55cb60`, con `RentalCompleted` único observado en 1712 ms desde el primer sondeo. El recorrido de 75 solicitudes y la traza de siete spans del 13 de octubre se conservan separados como antecedentes.

Los resultados son operaciones ejecutadas sobre dependencias reales propias. No se atribuyen al gimnasio ni al hospital, cuyos contratos y accesos están pendientes. Tampoco acreditan frontend, publicación cloud ni aceptación del equipo. La ejecución de GitHub Actions sobre `11d475c` (14 de octubre) aprobó pruebas, análisis y el trabajo Compose: [ejecución 38509003244](https://github.com/Gerpeirone/Car-Cruds/actions/runs/38509003244).

## 2. Comandos reproducibles

```powershell
go run ./cmd/devsetup
docker compose up -d --build --wait --wait-timeout 300
go run ./cmd/smoke -history
go test -race -count=1 -v ./cmd/... ./internal/... ./services/...
go vet ./cmd/... ./internal/... ./services/...
pwsh -File ./scripts/verify-resilience.ps1 -ReportPath output/resilience-run.json
go run ./cmd/messagecheck
go run ./cmd/load -concurrency 1,8,32,64,128 -duration 5s
pwsh -File ./scripts/measure-cache.ps1 -DurationSeconds 5
```

Las pruebas reales requieren las variables de integración indicadas en cada README: PostgreSQL de Clientes/Alquileres/Empleados, MongoDB, Redis y Solr. Un resultado omitido sin esas variables no reproduce la evidencia de integración. Las salidas nuevas se guardan en `output/` o temporales ignorados; sólo se incorporan informes revisados sin secretos.

`cmd/smoke` usa una identidad de prueba por aplicación y registros ficticios propios. Su limpieza deja empleados/vehículos inactivos, reservas finales e historial; no borra datos ajenos. `cmd/load` usa sólo GET local, por defecto disponibilidad para mañana, y registra revisión/árbol modificado. El script de fallas restaura sus componentes y desactiva sus vehículos incluso al finalizar con error.

## 3. Interpretación

La traza verifica el recorrido HTTP; la mensajería se correlaciona por event ID, sin contexto de traza propagado todavía. La actualización de búsqueda observada por sondeo y sus recuperaciones no miden automáticamente desde commit. Las etapas de carga son breves, de circuito cerrado y sobre datos pequeños; el 68,2 % de `503` a concurrencia 128 muestra protección de admisión y no throughput útil.

[ADR-013](adr/ADR-013-capacidad-costos.md) presenta resultados, límite observado, alternativa y costos. [POSTMORTEM](POSTMORTEM.md) registra impacto, causa y acciones. La comparación caliente/fallback muestra reducción de lecturas de ficha a MongoDB, sin mejora universal de latencia. No incluye fase fría ni línea base sin dependencia Redis; los contadores se suman secuencialmente e incluyen comprobación previa.

## 4. Trazabilidad

| Decisiones | Evidencia principal |
| --- | --- |
| D1/D2/D3/D4 | Código por servicio, suite y persistencia |
| D5 | Recorrido/outbox/inbox/historial, mensaje incompatible y recuperación broker |
| D6 | Actualización y reconstrucción de búsqueda; frescura extrema pendiente |
| D7 | Redis/versiones/expulsión y comparación caliente/fallback; fría/baseline pendientes |
| D8 | Contrato propio y lista por gateway; consumidor/acceso público pendientes |
| D9 | Sin evidencia externa; puertos y estado no configurado |
| D10/D12 | Fallas/recuperación y registro de acciones |
| D11 | Logs correlacionados y traza HTTP |
| D13 | Carga local, muestras de recursos y estimación fechada |

La matriz completa de cobertura y pendientes académicos se conserva en [ENTREGA-2](ENTREGA-2.md).

# Registro de fallas controladas y recuperación

| Campo | Valor |
| --- | --- |
| Versión documental | 0.1 |
| Fecha | 14 de octubre de 2026 |
| Estado | Ensayo local realizado; acciones de mejora pendientes |
| Alcance | Componentes propios de Car Cruds |
| Registro | [resilience.json](evidence/resilience.json), ejecución `ca07eded8eb2453398497e068a20c117` |

## 1. Propósito e impacto

Se provocaron fallas de una réplica de Flota, Redis, RabbitMQ y Solr para comprobar detección pasiva, aislamiento, datos conservados y recuperación. El sistema se operó por gateway local autenticado. El informe concluyó `PASS` con duración total de **103 464,67 ms**; los componentes detenidos se restauraron y los vehículos propios del ensayo quedaron inactivos.

Se trata de un incidente de laboratorio planificado, sin usuarios reales ni llamadas al gimnasio/hospital. El impacto observable incluyó respuestas `503` durante la retirada de una réplica, búsqueda temporalmente indisponible y retraso del índice. Los datos autoritativos permanecieron consultables en los escenarios definidos.

## 2. Línea temporal y resultados

El ensayo comenzó a las **03:08:59 UTC** y finalizó a las **03:10:43 UTC** del 14 de octubre. Los tiempos de recuperación se miden desde el sondeo indicado en el informe; no son compromisos de servicio.

| Fase | Observación real | Interpretación |
| --- | --- | --- |
| Línea base | Ocho respuestas exitosas mostraron ambas réplicas | Distribución por el LB visible |
| Detención de Flota B | 20 solicitudes: tres `503 SERVICE_UNAVAILABLE` iniciales, 17 `200` en A y 16 éxitos consecutivos al final | NGINX necesita fallas reales antes de retirar el destino; la transición admite errores |
| Recuperación de Flota B | Volvió a observarse B en 29 315,18 ms desde el sondeo | Compatible con `fail_timeout=30s`; no detección activa |
| Detención de Redis | Dos lecturas de ficha y disponibilidad válidas mediante fuente | Fallback acotado conserva lectura; no prueba una garantía de marcas tras perder Redis |
| Restauración de Redis | Componente saludable y ficha consultable | Recuperación verificada en este escenario |
| Detención de RabbitMQ | Nuevo vehículo persistido/consultable y ausente del índice en tres comprobaciones | Outbox conserva cambio; proyección admite retraso |
| Restauración de RabbitMQ | Vehículo visible en 12 931,10 ms desde primer sondeo; paso completo 15 323,06 ms | Recuperación eventual observada; no medición desde commit |
| Detención de Solr | Búsqueda `503 SEARCH_UNAVAILABLE`; ficha y disponibilidad `200` | Error honesto y separación del índice respecto de confirmación |
| Restauración de Solr | Búsqueda visible en 11 886,96 ms; once respuestas no disponibles durante arranque | Datos del índice conservados; recuperación no instantánea |

Las duraciones de conexión ante Flota B detenida fueron aproximadamente 1006, 1004 y 1023 ms. El timeout del proxy agotó el margen para cambiar de destino en esas solicitudes. El criterio registra hasta tres fallas iniciales y continuidad posterior; no presume ausencia de errores durante detección pasiva.

## 3. Detección, respuesta y causa

El script utiliza estados Docker, comprobación autenticada de gateway, códigos de respuesta, request ID e instancia. No depende de un tablero de alertas desplegado. Las fallas fueron inducidas mediante detención individual de componentes; la configuración privada de NGINX explica la retirada por `max_fails` y reingreso posterior a `fail_timeout`.

Redis utiliza fallback con autoridad en MongoDB. RabbitMQ conserva mensajes pendientes mediante outbox y los entrega al recuperarse. Solr no se utiliza para validar precio o disponibilidad. Estas separaciones permitieron continuar las operaciones ensayadas; no prueban disponibilidad si también cae la fuente compartida.

La restauración se realiza en una sección de cierre del script incluso ante falla del ensayo, con tiempos acotados. Los datos ajenos no se eliminan; sólo se desactivan los vehículos propios identificados por la ejecución.

## 4. Mensaje incompatible y cola de fallidos

Un ensayo adicional, [message-failure.json](evidence/message-failure.json), publicó de forma persistente un mensaje `RentalCompleted.v1` con estructura incompleta, identificado como dato de prueba. Clientes lo rechazó y RabbitMQ lo envió a `customers.rental-history.failed` con motivo `rejected` y referencia de la cola original.

Se verificó la identidad exacta del mensaje antes de confirmarlo en la cola de fallidos; no se tocaron mensajes ajenos. La herramienta exige que la cola esté inicialmente vacía y falla si encuentra información previa. Este caso demuestra enrutamiento de un mensaje incompatible; no acredita un reproceso de negocios ni alertas operativas.

## 5. Acciones de mejora y límites

| Acción | Estado |
| --- | --- |
| Documentar errores iniciales y límites de detección pasiva | Incorporado en D12 y componente LB |
| Calibrar retirada/recuperación y observar p95 bajo falla y carga sostenida | Pendiente de ensayos ampliados |
| Implementar Circuit Breaker donde corresponda por dependencia/operación | Pendiente; no se atribuye al timeout |
| Añadir colector, tableros y alertas sobre outbox, DLQ y errores | Pendiente |
| Definir y ensayar reproceso controlado de mensajes de negocio fallidos | Pendiente |
| Ensayar caída del LB, fuente compartida y pérdida de Redis | Pendiente |
| Ensayar los proveedores externos | Bloqueado por contratos/accesos; no se simula su aprobación |

Los escenarios no acreditan alta disponibilidad, latencia sostenida ni frescura de 5 s desde commit. El fallback observado no autoriza cargas ilimitadas sobre MongoDB. La protección de caché por marcas sólo se mantiene después de invalidación exitosa y sin pérdida de los datos Redis.

## 6. Reproducción

Con Docker iniciado y configuración local generada:

```powershell
docker compose up -d --build --wait --wait-timeout 300
pwsh -File ./scripts/verify-resilience.ps1 -ReportPath output/resilience-run.json
go run ./cmd/messagecheck
```

El script requiere PowerShell 7 y genera su informe sin credenciales. La herramienta de mensaje usa exclusivamente RabbitMQ local. Ambos ensayos afectan componentes de laboratorio y deben ejecutarse cuando no haya otro integrante utilizándolos para una demostración simultánea.

Referencias: [D10](adr/ADR-010-resiliencia.md), [D12](adr/ADR-012-balanceo-carga.md), [D5](adr/ADR-016-comunicacion-v2.md) y [estado de implementación](ESTADO-IMPLEMENTACION.md).

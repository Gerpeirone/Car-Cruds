# Estado de implementación de la entrega 2

| Campo | Valor |
| --- | --- |
| Versión documental | 0.3 |
| Fecha de corte | 14 de octubre de 2026 |
| Rama de trabajo | `features`; correcciones de cierre en `entrega-2/cierre` (PR #3) |
| Estado | Backend propio verificado en entorno local; aceptación e integración externas pendientes |
| Hito | Entrega 2 — 23 de octubre de 2026 |

## 1. Componentes disponibles

Clientes y Flota aplican una organización por capas; Alquileres y Empleados separan dominio, aplicación y adaptadores mediante arquitectura hexagonal. Clientes, Alquileres y Empleados utilizan bases PostgreSQL y credenciales propias. Flota persiste documentos versionados en MongoDB.

El gateway autentica aplicaciones, encamina llamadas y limita concurrencia. NGINX distribuye solicitudes internas entre `fleet-a` y `fleet-b`. Redis almacena fichas exploratorias; Solr ofrece búsqueda derivada. RabbitMQ transporta cambios de vehículos y finalizaciones de alquiler mediante outbox y consumidores idempotentes. Jaeger recibe las trazas HTTP instrumentadas.

Empleados implementa administración de personal y candidatos independientes. La lista activa pública para consumidores autorizados contiene exclusivamente `employeeId` y `displayName`, con paginación y conteo dentro de la misma instantánea de PostgreSQL. Es una API propia provisional, todavía sin aceptación ni acceso público acordados con el gimnasio.

La interfaz web y los adaptadores de gimnasio y hospital permanecen pendientes. El endpoint administrativo de estado de integraciones informa `503 NOT_CONFIGURED`; no inventa turnos, costos o datos del proveedor. El mock histórico de alquileres continúa en su módulo independiente.

## 2. Evidencia local registrada

| Evidencia | Fecha | Resultado y alcance |
| --- | --- | --- |
| Construcción y arranque Docker Compose | 13–14 de octubre | Construcción y `up --wait` satisfactorios; componentes propios saludables |
| [Suite y análisis estático](evidence/tests-summary.json) | 14 de octubre | Go 1.26.1; `go test -race -count=1 -v ./cmd/... ./internal/... ./services/...`: 60 pruebas aprobadas, ninguna omitida; `go vet`: salida 0 |
| Dependencias reales de la suite | 14 de octubre | PostgreSQL, MongoDB, Redis y Solr configurados mediante `TEST_*`; recursos de prueba aislados |
| [Recorrido del conjunto](evidence/integration-summary.json) | 13 de octubre | `go run ./cmd/smoke -history`: salida 0, 75 solicitudes y limpieza satisfactoria |
| [Persistencia y reconstrucción](evidence/runtime-persistence.json) | 14 de octubre | Reserva `COMPLETED` y empleado inactivo conservados tras reiniciar Clientes, Empleados y Alquileres; reindexación aceptada y comparación posterior de dos vehículos activos idéntica en fuente e índice |
| Recorrido final del backend corregido | 14 de octubre | Revisión `8327268`; `go run ./cmd/smoke -history`: salida 0, 78 solicitudes y limpieza satisfactoria; ejecución `10ead0c0c22142bc968ec9e8ef55cb60` |
| [Traza distribuida](evidence/trace-reservation.json) | 13 de octubre | Exportación real de Jaeger: siete spans con relaciones padre e hijo, abarcando gateway, Alquileres, Clientes y una réplica de Flota |
| [Fallas y recuperación](evidence/resilience.json) | 14 de octubre | Ensayo PASS de réplica B, Redis, RabbitMQ y Solr; errores iniciales explícitos, restauración y limpieza verificadas |
| [Mensaje incompatible](evidence/message-failure.json) | 14 de octubre | Rechazo y enrutamiento a cola de fallidos; sólo mensaje propio inspeccionado/confirmado |
| [Carga de disponibilidad](evidence/load-availability.json) | 14 de octubre | Cinco etapas; 64 trabajadores: 1471,16 éxitos/s; 128: 68,2 % de 503 por admisión, límite y alternativa registrados en D13 |
| Corrección posterior de Redis — prueba focal | 14 de octubre | Error de transporte separado de miss; una lectura fuente y ningún Put tras Get fallido; miss normal escribe una vez; TCP silencioso 76,0772 ms con contexto de 75 ms; detector de carreras y vet satisfactorios |
| [Comparación de caché](evidence/cache-metrics.json) | 14 de octubre | Caliente: 8461 hits y cero lecturas de ficha MongoDB; fallback: 7395 errores Redis y lecturas fuente; todas las respuestas 200, sin mejora universal de latencia |
| Presión de Redis aislado | 14 de octubre | Prueba focal con `volatile-lru`: 69 expulsiones; marca V2 sin TTL conservada, llenado atrasado V1 rechazado y ficha V2 utilizable |

La suite final y el nuevo recorrido se ejecutaron después de la corrección Redis y la reconstrucción del entorno, sobre la revisión `8327268`. Las pruebas de presión utilizaron además `TEST_REDIS_EVICTION_ADDR` y `TEST_REDIS_EVICTION_PASSWORD`, con Redis separado. El recorrido anterior de 75 solicitudes y la traza del 13 de octubre se conservan como antecedentes.

El recorrido verifica respuestas `401/403`, permisos de gimnasio y cliente, exclusión de candidatos, ambas réplicas de Flota, actualización de caché/búsqueda, idempotencia y cancelación persistentes. Dos reservas concurrentes produjeron una confirmación `201` y un conflicto `409`. Retiro y devolución resultaron repetibles; `RentalCompleted` terminó publicado en outbox y aplicado una sola vez en inbox/historial de Clientes.

La actualización de Solr se observó en 1265 ms **desde el primer sondeo**. Ese intervalo no mide desde el commit de MongoDB y no acredita el objetivo de frescura extremo a extremo de D6.

El recorrido final de 78 solicitudes volvió a confirmar un único `RentalCompleted` aplicado; se observó en 1712 ms desde el primer sondeo. Ese intervalo no mide desde commit ni establece un SLO sostenido.

La operación trazada del 13 de octubre utilizó request ID `7ec72466-c00a-4141-a4c5-69a21f5f7f3b` y trace ID `8a9219ca06093073a628894ecb11b1a8`, presentes en los registros de los cuatro servicios. Esta evidencia corresponde al recorrido HTTP; la propagación de contexto de traza por mensajes RabbitMQ sigue pendiente.

## 3. Garantías y límites de implementación

- Alquileres conserva agenda e idempotencia en PostgreSQL; utiliza exclusión de períodos y un único alquiler activo por vehículo. La devolución y su evento pendiente se guardan en una transacción local.
- Flota confirma documento y evento pendiente mediante outbox embebido acotado. El índice se actualiza ante cambios; la reconstrucción inicial vuelve a encolar la fuente en el mismo core. El índice sombra con intercambio controlado permanece como mejora prevista.
- La cotización autoritativa omite caché y búsqueda. La caída de Redis admite fallback a MongoDB con espera y concurrencia acotadas.
- Las fichas Redis tienen TTL; las marcas de versión no lo tienen. `volatile-lru` permite expulsar fichas sin expulsar esas marcas. El rechazo de un llenado atrasado presupone invalidación exitosa y conservación de los datos de Redis; una pérdida o vaciado de Redis elimina esa garantía. Las marcas crecen con los vehículos retenidos: debe planificarse su limpieza. Al agotarse memoria, el error conduce a fallback, sin convertir Redis en autoridad.
- Se implementaron deadlines, límites de concurrencia, pools, outbox y colas de fallidos. Las fallas controladas y su [postmortem](POSTMORTEM.md) están registrados; Circuit Breaker y alertas operativas siguen pendientes.
- Las métricas están instrumentadas y disponibles internamente. No se incluyeron un colector Prometheus, tableros ni alertas.
- Dos réplicas de Flota y detección pasiva NGINX no acreditan alta disponibilidad: gateway, balanceador y almacenes compartidos siguen siendo puntos únicos de falla.
- Los tokens identifican aplicaciones de laboratorio. La gestión de cuentas personales y el despliegue HTTPS público no están incluidos en esta verificación.

## 4. Reproducción

```powershell
go run ./cmd/devsetup
docker compose up -d --build --wait --wait-timeout 300
go run ./cmd/smoke -history
```

La configuración local se mantiene en `.env`, fuera de Git. Las pruebas contra almacenes requieren variables `TEST_*` explícitas; omitirlas produce omisiones que no deben contabilizarse como integración. Consultar las guías de [Clientes](../services/customers/README.md), [Flota](../services/fleet/README.md), [Alquileres](../services/rentals/README.md), [Empleados](../services/employees/README.md) y [smoke](../cmd/smoke/README.md).

El [workflow](../.github/workflows/ci.yml) separa pruebas unitarias/análisis y un trabajo Compose con suite real sin omisiones, Redis de presión dedicado y smoke del historial. Ambos trabajos aprobaron en GitHub Actions sobre la revisión `11d475c` el 14 de octubre de 2026 ([ejecución 38509003244](https://github.com/Gerpeirone/Car-Cruds/actions/runs/38509003244), PR #2). La ejecución previa sobre `a7a9f97` falló y motivó la corrección de deadlines Redis en Linux.

La publicación de la lista para el gimnasio se prepara en [DESPLIEGUE](DESPLIEGUE.md): VM única con Caddy y HTTPS delante del gateway. La VM y la URL pública siguen pendientes.

## 5. Pendientes de cierre

La [revisión de cierre del 15 de octubre](REVISION-E2-2026-10-15.md) registra los cambios posteriores a este corte: validación remota de reservas fuera de transacción, verificador de la lista propia y correcciones del despliegue. Sus pruebas y limitaciones se presentan por separado de la evidencia del 13/14 de octubre.

1. Acordar población, identificadores, contrato, credenciales y acceso de la lista con el gimnasio; comprobar su consumo real y mantener la capacidad accesible durante la evaluación.
2. Obtener los contratos y accesos de gimnasio/hospital para implementar gastos, uso, necesidad de consulta, turnos y conciliación. Registrar información desconocida sin inferir resultados negativos.
3. Ampliar los ensayos locales de fallas/carga y la comparación de caché (fría y sin dependencia), validar la alternativa de escala y completar retención, alertas y operación conforme D10/D12/D13.
4. Completar interfaz web, observabilidad operativa y condiciones de publicación cloud. Revisar la cobertura de las historias con el equipo y presentar la propuesta mediante pull request.
5. Obtener aprobación antes de integrar en `main`; la verificación local no equivale a aceptación del consumidor ni de la cátedra.

Esta sección se actualiza a medida que se incorporan informes reales. El control académico está en [ENTREGA-2](ENTREGA-2.md); [EVIDENCIA-E2](EVIDENCIA-E2.md) enlaza informes y comandos.

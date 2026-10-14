# Entrega 2 — dominio propio de punta a punta

| Campo | Valor |
| --- | --- |
| Proyecto | Car Cruds |
| Versión documental | 0.3 |
| Fecha de revisión | 14 de octubre de 2026 |
| Fecha de entrega | Viernes 23 de octubre de 2026 |
| Estado | Backend propio verificado localmente; integración externa y cierre académico pendientes |
| Referencia | [Enunciado](../Enunciado%20TP%20Final.md), sección 6.3 |

## 1. Alcance y evolución

Se conserva el alquiler de autos y se incorpora Empleados para la administración de personal. El gimnasio del grupo de Pablo consumirá la lista y aportará membresías, uso y gastos. El hospital del grupo de Octi, denominado Clínica en el diseño, proveerá información y turnos. Los contratos, entornos y credenciales externos permanecen pendientes.

La preparación documental consolidó **9 épicas, 28 historias, 22 RF, 12 RNF, 22 reglas y 15 acuerdos pendientes**. Sobre ese diseño se implementaron los cuatro servicios propios, gateway, balanceador con dos réplicas de Flota, persistencia, índice, caché, mensajería y observabilidad HTTP. El mock de la entrega 1 se conserva como antecedente, independiente del backend real.

## 2. Documentación y evidencia

| Artefacto | Finalidad |
| --- | --- |
| [SPEC](../SPEC.md), [BACKLOG](BACKLOG.md) y [REQUERIMIENTOS](REQUERIMIENTOS.md) | Alcance y trazabilidad HU → RF/RNF → RN |
| [ARCHITECTURE](ARCHITECTURE.md) e [índice ADR](adr/README.md) | Propiedad de datos, estilos, comunicaciones y decisiones D1–D13 |
| [ESTADO-IMPLEMENTACION](ESTADO-IMPLEMENTACION.md) | Código operativo, resultados registrados y límites |
| [INTEGRACIONES](INTEGRACIONES.md) y [contratos](contracts/README.md) | API propias implementadas y condiciones externas sin acuerdo |
| [Suite](evidence/tests-summary.json), [recorrido](evidence/integration-summary.json), [persistencia](evidence/runtime-persistence.json) y [traza](evidence/trace-reservation.json) | Evidencia real del entorno local |

## 3. Correspondencia con el hito del 23 de octubre

| Exigencia | Cobertura obtenida | Pendiente para acreditar el hito |
| --- | --- | --- |
| Servicio operativo además de funcionalidad compartida funcionando | Flujo propio de alquiler operativo; lista de empleados operativa por gateway local | Publicar la capacidad compartida, acordar su contrato y comprobar consumo del gimnasio |
| Almacenamiento integrado al flujo | PostgreSQL y MongoDB reales; reservas/candidatos/empleados persistentes; reinicio verificado | Condiciones operativas de respaldo y despliegue definitivo |
| Logs estructurados y correlacionados, primera traza distribuida | Registros JSON y traza HTTP real entre cuatro servicios | Observabilidad de integración externa y mensajería; tableros/alertas posteriores |
| D2: estilos internos | Capas en Clientes/Flota; Hexagonal en Alquileres/Empleados | Revisión y aceptación del grupo |
| D6: búsqueda | Solr actualizado por cambios; reconstrucción inicial y comparación fuente/índice realizadas | Medir frescura desde commit; evaluar índice sombra y operación de reconstrucción |
| D7: caché | Redis/versiones/expulsión verificados; comparación caliente/fallback muestra reducción de lecturas de ficha | Fase fría, línea base deshabilitada, ensayo sostenido y retención; no mejora universal de latencia |
| D9: consumo externo | Puertos de clínica y membresías definidos; estado no configurado explícito | Contrato y pruebas contra el proveedor real; adaptadores externos |
| D10: resiliencia | Protecciones y fallas/recuperación reales con postmortem; mensaje incompatible a DLQ | Circuit Breaker, alertas, reproceso de negocios y ensayos ampliados/exteriores |
| D12: balanceo | Dos réplicas y retirada/recuperación verificadas; tres errores iniciales documentados | Caída del balanceador/fuente común y comparación sostenida |
| D13: capacidad y costos | Carga inicial, límite de admisión, alternativa y estimación USD44/84 documentados | Ampliar duración/mezcla, medir alternativa y validar recursos cloud |
| Validación D1/D3/D5 | Cuatro propietarios de datos; almacenes separados; recorrido HTTP y RentalCompleted real | Validación de comunicaciones y condiciones externas |
| Primera versión D11 | Instrumentación HTTP, registros y exportación Jaeger verificadas | Acordar indicadores/alertas y documentar propagación asíncrona pendiente |
| Actualización D8 | ADR-017 y API propia de empleados implementada provisionalmente; historial conservado | Acuerdo del consumidor, acceso publicado y prueba conjunta |

Los resultados locales acreditan las operaciones ensayadas. No acreditan automáticamente toda una historia, integración externa, acceso cloud, alta disponibilidad ni cumplimiento académico integral. Una prueba omitida por falta de variables no se considera evidencia de dependencia real.

## 4. Resultados registrados

El 14 de octubre, después de corregir Redis y reconstruir el entorno, la suite real con detector de carreras aprobó 60 pruebas sin omisiones sobre la revisión `8327268`; `go vet` finalizó con salida 0. La prueba focal de presión Redis registró 69 expulsiones y conservó la marca necesaria para rechazar un llenado anterior.

El 13 de octubre, `cmd/smoke -history` finalizó con salida 0 tras 75 solicitudes. Verificó autenticación y aislamiento, ambas réplicas, catálogo/caché/búsqueda, reservas concurrentes con una respuesta `201` y otra `409`, cancelación, retiro/devolución y un evento aplicado una sola vez en Clientes. La traza exportada contiene siete spans enlazados en cuatro servicios.

El recorrido final del 14 de octubre sobre `8327268` aprobó 78 solicitudes y la limpieza, con ejecución `10ead0c0c22142bc968ec9e8ef55cb60`. Volvió a verificar un efecto único de `RentalCompleted`, observado en 1712 ms desde el primer sondeo. La traza de siete spans corresponde al antecedente del 13; no se atribuye al recorrido final.

Tras reiniciar Clientes, Empleados y Alquileres se conservaron una reserva completada y un empleado inactivo. La reindexación inicial aceptó tres vehículos; tras completar los ensayos, la comparación de los dos vehículos activos mostró los mismos identificadores y versiones en MongoDB y Solr. No se simuló la pérdida total del core. La respuesta `202` indica aceptación, no finalización; la evidencia de conclusión es la comparación posterior.

Los [ensayos de fallas](POSTMORTEM.md) verificaron detección pasiva, fallback y recuperación de índice/outbox. [D13](adr/ADR-013-capacidad-costos.md) registró la carga de disponibilidad, el límite principal observado de 64 solicitudes simultáneas del gateway, una alternativa de escalado y costos de laboratorio. Se conserva el detalle y alcance en [EVIDENCIA-E2](EVIDENCIA-E2.md).

## 5. Prioridades de cierre

1. Publicar y validar la lista propia con el gimnasio; cerrar los acuerdos que bloquean uso, costos e intercambio administrativo.
2. Obtener el contrato del hospital para información de necesidad, solicitud y seguimiento de turnos, conservando incertidumbre y referencias.
3. Revisar y ampliar los informes iniciales de D10/D12/D13; ampliar comparación de caché y validar límites/alternativa sin extrapolar cifras locales a cloud.
4. Revisar historias, contratos y evidencia con el grupo; ejecutar la automatización de CI y registrar su resultado.
5. Mantener los datos locales y completar las condiciones de acceso durante la evaluación. La integración a `main` requiere aprobación del equipo.

## 6. Criterios de aceptación y referencia histórica

Una implementación se acredita con comportamiento acorde a HU/RF/RN, pruebas pertinentes ejecutadas, persistencia y errores comprobados, instrucciones reproducibles y documentación actualizada. La capacidad compartida además exige contrato, servidor accesible y validación con el consumidor. Los proveedores reales se prueban sobre sus contratos; simulaciones futuras deben identificarse como tales.

[ENTREGA-1](ENTREGA-1.md), [openapi-v1.json](contracts/openapi-v1.json) y `mocks/rentals` conservan la evidencia original. No se utilizan como prueba de Empleados ni de las integraciones actuales.

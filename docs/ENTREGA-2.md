# Entrega 2 — dominio propio de punta a punta

| Campo | Valor |
| --- | --- |
| Proyecto | Car Cruds |
| Versión documental | 0.4 |
| Fecha de revisión | 8 de noviembre de 2026 |
| Fecha del hito | Viernes 23 de octubre de 2026 |
| Revisión técnica evaluada | `main`, `19c4e06` |
| Estado | Backend verificado localmente; acceso del consumidor, aceptación conjunta y CI pendientes |
| Referencia | [Enunciado](../Enunciado%20TP%20Final.md), apartados 4.1 y 6.3 |

## 1. Alcance vigente

El apartado 6.3 exige al menos un servicio operativo además de la funcionalidad compartida, almacenamiento integrado, logs estructurados y correlacionados, una primera traza distribuida y las decisiones arquitectónicas indicadas en la matriz siguiente. El frontend completo y el despliegue cloud con URL pública se exigen expresamente para la presentación grupal. La sección 4.1 exige que la primera capacidad compartida operativa sea accesible al consumidor y permanezca disponible durante la evaluación.

El backend implementa Clientes, Flota, Alquileres y Empleados, además de gateway y balanceador con dos réplicas de Flota. Empleados se limita a personal registrado, membresías y seguimiento de turnos. [ADR-019](adr/ADR-019-consumo-turnos-socios.md) adaptó solicitud y consulta de turnos al contrato oficial Turnos para socios 1.0.0, con autenticación, idempotencia persistida y recuperación explícita. Estos cambios están integrados en `main` mediante PR #5. PR #6 incorporó la documentación de endpoints y el visor Swagger, que no sustituye la interfaz de negocio.

El gimnasio consulta la lista de empleados activos y puede enviar evidencia de membresías, uso y gastos a Car Cruds; la aceptación de este intercambio y su operación conjunta siguen pendientes. El hospital provee turnos: el adaptador y su prueba contra código original funcionan en laboratorio, pero faltan URL, credencial y datos operativos acordados para la conexión real.

## 2. Correspondencia con la entrega 2

| Exigencia | Cobertura comprobada | Límite o pendiente |
| --- | --- | --- |
| Servicio operativo y funcionalidad compartida | Flujo de alquiler y lista mínima de empleados operativos por gateway; recorrido y permisos aprobados | Acreditar acceso y consumo del gimnasio durante la evaluación |
| Almacenamiento integrado | PostgreSQL y MongoDB ejercitados por pruebas reales; Redis y Solr también verificados | El arranque en la CI de `19c4e06` falló en MongoDB (mitigado en la entrega 3, ver [ENTREGA-3](ENTREGA-3.md)); no se atribuye ese fallo a la persistencia ya comprobada localmente |
| Logs correlacionados y primera traza | Registros JSON de reserva en cuatro servicios y nueva traza Jaeger con siete spans | Propagación AMQP, tablero y alertas quedan para ampliación operativa |
| D2: estilos internos | Capas en Clientes/Flota y Hexagonal en Alquileres/Empleados | Conservar justificación y revisión del equipo |
| D6: búsqueda | Solr actualizado por cambios, filtros/paginación/ordenamiento y recorrido actual aprobados; reconstrucción inicial con evidencia histórica | Medir frescura desde commit y ampliar reconstrucción operativa |
| D7: caché | Suite actual con Redis y presión aislada aprobada; comparación histórica caliente/fallback con reducción de lecturas | No se repitió la comparación; fase fría y línea base sin Redis pendientes |
| D9: consumo externo | Contrato oficial, adaptador autenticado/idempotente y prueba real aislada contra gateway y Turnos originales aprobados | Ensayo conjunto con URL, clave, pacientes y médicos del hospital |
| D10: resiliencia | Timeouts, límites, outbox y tratamiento de fallidos; postmortem de fallas propias histórico | Ensayos externos y ampliaciones pendientes; Circuit Breaker es mejora prevista, no un requisito específico de 6.3 |
| D12: balanceo | Dos réplicas comprobadas en el recorrido actual; retirada y recuperación con evidencia histórica | No se repitió el ensayo de caída en este corte |
| D13: capacidad/costos | Carga histórica, límite, alternativa y estimación fechada documentados | Carga y precios no se actualizaron ni acreditan capacidad cloud |
| Validación D1/D3/D5 | Propiedad de datos, almacenes y comunicaciones verificadas; RentalCompleted con efecto único | Operación conjunta externa pendiente |
| Primera versión D11 | Instrumentación, correlación y traza HTTP actuales | No hay tablero ni alertas operativas |
| Actualización D8 | Contrato propio de personal/gimnasio 0.3.0, lista y permisos verificados el 08/11; la versión 0.4.0 agrega la consulta de estado laboral `/status` (PR #4) | Aceptación bilateral, población y acceso acordados pendientes |

## 3. Evidencia del corte del 8 de noviembre

La [revisión verificable](REVISION-E2-2026-11-08.md) y su [resumen estructurado](evidence/revision-e2-2026-11-08.json) registran:

- Suite propia con dependencias reales y detector de carreras: **155 pruebas/subpruebas aprobadas, ninguna omitida ni fallida**.
- Recorrido `cmd/smoke -history`: **67 solicitudes**, reservas concurrentes `201/409`, ambas réplicas y `RentalCompleted` único; limpieza satisfactoria.
- `cmd/checkshared`: autenticación, lista mínima, paginación y aislamiento administrativo aprobados.
- Contrato del hospital: prueba aprobada con gateway, Turnos, MySQL y RabbitMQ originales aislados, commit `c0b65f4`, sin modificar el proveedor.
- Seis contratos, 34 operaciones, 218 ejemplos y recursos Swagger validados; `go vet`, dependencias y mock histórico satisfactorios.
- [Traza de reserva](evidence/trace-reservation-2026-11-08.json) y [logs correlacionados](evidence/correlated-logs-2026-11-08.json) de una operación actual.

Los contadores incluyen subpruebas y las suites se superponen: no se suman entre sí. Las pruebas de carga, caída y persistencia tras reinicio de octubre conservan sus fechas en [EVIDENCIA-E2](EVIDENCIA-E2.md); no se presentan como repetidas en noviembre.

La [CI de `19c4e06`](https://github.com/Gerpeirone/Car-Cruds/actions/runs/42516259957) aprobó documentación y pruebas/análisis estático, pero el trabajo Compose falló al iniciar MongoDB, con salida 1. Los pasos posteriores no se ejecutaron. El log disponible no contiene el diagnóstico interno de MongoDB; un arranque nuevo aislado local sí aprobó. La causa raíz permanece abierta.

## 4. Pendientes y condiciones de cierre

[PENDIENTES-E2](PENDIENTES-E2.md) identifica qué falta, quién debe aportar cada dato y cómo comprobar el cierre. Las prioridades son:

1. Resolver y verificar el arranque de MongoDB en CI.
2. Acordar contrato, población, acceso y credencial de la lista con el gimnasio; registrar consumo real y devolución de evidencia.
3. Recibir URL y clave privadas del hospital, pacientes habilitados, médicos y horarios de ensayo; comprobar el flujo conjunto.
4. Revisar con el equipo y la cátedra la aceptación del flujo administrativo y las limitaciones declaradas.

No se declara el hito totalmente cerrado mientras falten estas acreditaciones. Las credenciales se intercambian por un canal privado y permanecen fuera del repositorio.

## 5. Documentación e historia

[SPEC](../SPEC.md), [BACKLOG](BACKLOG.md) y [REQUERIMIENTOS](REQUERIMIENTOS.md) conservan trazabilidad de alcance; [ARCHITECTURE](ARCHITECTURE.md) e [índice ADR](adr/README.md) contienen las decisiones. [API](API.md), [contratos](contracts/README.md), [gimnasio](COMUNICACION-GIMNASIO.md) y [hospital](COMUNICACION-HOSPITAL.md) documentan el uso actual.

[ENTREGA-1](ENTREGA-1.md), el mock y los informes de octubre se conservan como historia. Su evidencia se limita a la revisión y operaciones ensayadas. Esta actualización documental se presenta por rama y pull request; no implica aprobación de su integración a `main`.

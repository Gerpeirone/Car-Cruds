# ADR-017 — Capacidad compartida de Empleados

> **Revisión de alcance — 23/10/2026:** [ADR-018](ADR-018-empleados-sin-candidatos.md) retira candidatos y limita nuevas vinculaciones, intenciones y primer envío de turnos a empleados activos; conserva historia y seguimiento previo de inactivos. Los cuerpos y resultados siguientes mantienen sus fechas históricas. Los contratos propios actuales son personal/gimnasio 0.3.0 y clínica 0.2.0; la lista y membresías del gimnasio no cambian. La migración al contrato Turnos para socios publicado posteriormente por el hospital queda pendiente.

## Actualización del 22 de octubre de 2026

Se conserva el sentido original: gimnasio consume la lista de Car Cruds y proporciona membresías, uso y gastos. Se retiró el alta saliente al gimnasio agregada posteriormente, por solicitud del responsable del proyecto. No se modifica el repositorio del otro equipo ni se implementa el hospital en esta etapa.

Se ofrece una devolución HTTP entrante propia y un informe privado, documentados en [COMUNICACION-GIMNASIO](../COMUNICACION-GIMNASIO.md) y OpenAPI propio 0.2.0. Es una implementación local propuesta, no un contrato aceptado por Pablo. `MembershipInbox` sustituye el puerto de consulta de membresías sin adaptador: persiste instantáneas e historia en PostgreSQL, con transacción y bloqueo por empleado/período. Se autentica el origen, valida la existencia del empleado y conserva hechos desconocidos. Una versión mayor reemplaza, una repetición no suma y un atraso no retrocede; un conflicto devuelve 409. La historia y la evidencia vigente se confirman juntas. Los totales distinguen moneda, costo y gasto, con faltantes y control de desbordamiento.

No se agregan tarifas, facturación, asistencia inferida ni operaciones del proveedor. Persisten los acuerdos pendientes sobre semántica económica, identidad, períodos, aceptación de la devolución, despliegue y prueba conjunta. Las secciones siguientes conservan el contexto histórico de la decisión.

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D8: contrato propio |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | API propia implementada provisionalmente; acuerdo externo pendiente |
| Reemplaza | [ADR-008](ADR-008-contrato-propio.md) para la integración entre grupos |

## 1. Contexto

El grupo comunicó que el gimnasio consumirá una lista de empleados y devolverá uso/no uso de membresías para controlar gastos. La capacidad anterior de disponibilidad/reservas no satisface ese intercambio. Car Cruds también consumirá turnos de una clínica dentro del flujo de personal.

## 2. Decisión propuesta

Empleados publica la lista propia a través del gateway. La propuesta inicial se conserva en [employees-openapi-draft.json](../contracts/employees-openapi-draft.json) y [su documentación](../contracts/EMPLOYEES.md); el contrato implementado se identifica en la sección de avance. Ruta, campos mínimos, población y paginación son propuestas internas para revisión; no se presentan como acuerdos alcanzados.

La identidad de aplicación del gimnasio tendrá permiso de lectura limitado, con autorización sobre los registros y datos compartidos. No se exponen candidatos, datos de clientes, credenciales o datos médicos. La población de empleados, las bajas y el identificador externo se acordarán expresamente.

La devolución del uso requiere un contrato independiente del listado: mecanismo, identidad, período, significado y reglas de repetición/corrección están pendientes. Empleados utiliza la evidencia para un informe administrativo y conserva incertidumbre cuando falta información. La lectura tiene una consecuencia de negocio en el control de beneficios, que deberá validarse y demostrarse con el grupo consumidor.

El contrato de reservas 1.0.0 y su mock se conservan como referencia histórica. La versión de la nueva API no renombra ni modifica ese contrato. La evolución compatible y las transiciones de versiones se acordarán antes de que exista un consumidor operativo; no se declara compatibilidad con un acuerdo todavía inexistente.

## 3. Alternativas

| Alternativa | Evaluación |
| --- | --- |
| Mantener reservas como capacidad para el gimnasio | No corresponde al intercambio comunicado |
| Acceso directo a la base de Empleados | Acopla esquemas y elude permisos y contrato |
| Publicar todos los datos del personal | Agrega datos innecesarios para el beneficio y dificulta controlar accesos |
| Acordar la lista y la devolución antes de formalizar el borrador | Reduce revisiones, pero retrasa la discusión; se elige un borrador claramente identificado para facilitarla |

## 4. Consecuencias y límites

El gimnasio puede integrar una capacidad administrativa sin acceder a bases internas. La empresa asume mantener el contrato y la disponibilidad publicados. Existe servidor local de Empleados y configuración de credenciales fuera de Git; URL pública y prueba conjunta con el consumidor permanecen pendientes.

La lista por sí sola no determina cuánto debe pagar la empresa: las tarifas y reglas de imputación están pendientes. La separación y los permisos propios están implementados; el significado externo del uso debe acordarse antes de integrar.

## 5. Validación prevista

Revisar el borrador con el consumidor, validar formato y referencias, aportar ejemplos acordados, probar autenticación/población/paginación y compatibilidad y demostrar el informe con devolución real. Las verificaciones propias ejecutadas y los acuerdos externos pendientes se registran en la sección de avance.

Acuerdos pendientes y fuentes de referencia: [INTEGRACIONES.md](../INTEGRACIONES.md). Material de cátedra: teoría 02, 06 y 08; práctica 01 y 02 dentro de `clases.zip`.


## Revisión de alcance — 11 de octubre de 2026

Se amplía la propuesta sin cambiar las fronteras ni las tecnologías: Gimnasio corresponde al grupo de Pablo y Clínica al hospital de Octi. El gimnasio consume empleados y administra membresías/costos; Empleados incorporará los datos económicos individuales y totales por período, con deduplicación, correcciones y separación por moneda. Uso y gasto se tratan como evidencias distintas.

El flujo del hospital comprende empleados registrados, además del flujo previo de candidatos: información de quién necesita consulta, solicitud de turno desde Empleados y seguimiento de modificaciones/cancelaciones/reprogramaciones administradas por el proveedor. Persistirá la relación entre empleado, necesidad, intención local, turno externo y evidencia de estado, sin interpretar un timeout como ausencia ni repetir comandos sin recuperación acordada.

El alcance de membresías/costos y turnos continúa sin implementar ni acordar con los equipos externos. No se modifica el borrador de lista para inventar contratos económicos o médicos. Consultar INTEGRACIONES, RF-19/HU-23 y RN-21/RN-22 para los acuerdos y verificaciones pendientes. La revisión anterior del registro se conserva en Git.

## Avance de implementación — 14 de octubre de 2026

[employees-openapi.json](../contracts/employees-openapi.json), OpenAPI 3.0.3 versión 0.1.0, describe la API propia implementada. GET /v1/employees devuelve únicamente employeeId/displayName de activos; admite gym/admin/service, page desde 1 y pageSize 20 con máximo 100. Conteo e ítems comparten una instantánea READ ONLY REPEATABLE READ por página, con orden estable; no hay instantánea entre páginas. Candidatos y operaciones administrativas quedan restringidos a admin. Pruebas reales y recorrido por gateway verifican formato, permisos y separación. El borrador anterior se conserva histórico. Aceptación del consumidor, población/bajas, identidad externa, URL pública y devolución de membresías/gastos continúan pendientes.

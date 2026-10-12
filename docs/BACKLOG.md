# Backlog del producto — Car Cruds

**Equipos externos:** gimnasio del grupo de Pablo (consume nuestra lista de empleados) y hospital del grupo de Octi (Car Cruds consume su API). Las referencias a “Clínica” en el diseño identifican al hospital de Octi. Contratos, entornos y credenciales siguen pendientes de acuerdo.

| Campo | Valor |
| --- | --- |
| Versión | 0.2 |
| Fecha | 12 de octubre de 2026 |
| Estado | Preparación documental de la Entrega 2 |
| Hito objetivo | Viernes 23 de octubre de 2026 |
| Negocio principal | Alquiler de vehículos |
| Servicios de negocio | Clientes, Flota, Alquileres y Empleados |

## 1. Alcance y situación de la entrega

El backlog incorpora el servicio Empleados y un Load Balancer tras la revisión de la primera entrega. El gimnasio consume la lista de empleados y comunica uso o no uso de membresía; Car Cruds consume la capacidad de turnos de la clínica para consultas médicas de empleados registrados y revisiones de posibles incorporaciones. Estas integraciones pertenecen a Empleados y conservan independiente la operación de alquileres.

Los contratos, campos, protocolos, identidad de consumidores y mecanismos de devolución o consulta externos están pendientes de acuerdo. Los criterios que dependen de esos acuerdos son obligaciones de comportamiento para el diseño; todavía no constituyen endpoints, eventos o esquemas publicados.

Las historias están **diseñadas**. El único componente ejecutable disponible es el mock de reservas en memoria; su evidencia parcial no acredita servicios reales ni las nuevas integraciones. Para cumplir la Entrega 2 faltan al menos un servicio operativo, la capacidad compartida funcionando, almacenamiento integrado, logs correlacionados y una primera traza distribuida, además de las decisiones exigidas por el [enunciado, sección 6.3](../Enunciado%20TP%20Final.md#63-viernes-23-de-octubre---entrega-2-dominio-propio-de-punta-a-punta).

## 2. Convenciones de planificación

- **P1:** necesario para el primer flujo o para cerrar una dependencia crítica de E2. **P2:** completa la operación o la evidencia del sistema final.
- **E2:** objetivo de la entrega del 23 de octubre. **PG:** presentación grupal del 11 o 13 de noviembre, según asignación. **Defensa:** evaluación individual, con fecha pendiente.
- Los hitos indicados son objetivos de implementación. La preparación documental actual no implica que estén alcanzados.
- Las historias técnicas se identifican como **habilitadores** y se evalúan por evidencia operativa.
- Una historia se considera terminada cuando satisface sus criterios, cuenta con verificaciones pertinentes y actualiza la documentación y los contratos afectados.

## 3. Épicas

| ID | Épica | Resultado | Propietario | Historias |
| --- | --- | --- | --- | --- |
| E-01 | Catálogo | Administrar y seleccionar vehículos para alquiler. | Flota | HU-01, HU-02 |
| E-02 | Clientes y reservas | Validar al conductor y confirmar una asignación con precio acordado. | Clientes / Alquileres | HU-03 a HU-06 |
| E-03 | Operación de alquiler | Controlar retiro, devolución y mantenimiento. | Alquileres | HU-07 a HU-09 |
| E-04 | Personal | Distinguir posibles incorporaciones del registro de empleados. | Empleados | HU-10, HU-11 |
| E-05 | Integración con gimnasio | Publicar empleados y recibir/consultar membresías, uso y costos individuales/totales. | Empleados | HU-12 a HU-14 |
| E-06 | Integración con clínica | Identificar empleados que necesitan consulta, solicitar turnos y reflejar cambios del hospital. | Empleados | HU-15 a HU-17, HU-23 |
| E-07 | Plataforma | Controlar acceso, balancear consultas de Flota y protegerse de fallas de dependencias. | Gateway / Load Balancer / servicios | HU-18, HU-19, HU-27 |
| E-08 | Evidencia y reproducción | Verificar, observar y reproducir el sistema. | Transversal | HU-20 a HU-22, HU-28 |
| E-09 | Datos derivados y eventos | Mantener búsqueda, caché y proyecciones coherentes con las fuentes mediante eventos. | Flota / Alquileres / Clientes | HU-24 a HU-26 |

## 4. Historias y criterios de aceptación

Las reglas RN-01 a RN-12 conservan su significado en [SPEC.md](../SPEC.md). Las reglas RN-13 a RN-22 cubren personal e integraciones. Los requisitos RF y RNF se detallan en [REQUERIMIENTOS.md](REQUERIMIENTOS.md).

### HU-01 — Administrar vehículos y tarifas

Como **operador**, quiero registrar y modificar vehículos y sus tarifas para mantener el catálogo disponible para alquiler.

**Épica:** E-01. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: base del catálogo; PG: administración completa. **Dependencias:** HU-18; persistencia de Flota.

**Trazabilidad:** RF-01, RNF-05, RNF-08, RN-06.

1. **Dado** un operador autorizado y una ficha válida, **cuando** registra o modifica el vehículo, **entonces** Flota conserva el cambio y una consulta posterior devuelve la versión vigente.
2. **Dado** un cambio confirmado de características o tarifa, **cuando** se actualiza la fuente de Flota, **entonces** la indexación recibe el cambio sin esperar una búsqueda; las reservas existentes conservan su precio acordado.

### HU-02 — Buscar vehículos

Como **cliente**, quiero buscar vehículos por características y tarifa para seleccionar una alternativa de alquiler.

**Épica:** E-01. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** PG; diseño de búsqueda en E2. **Dependencias:** HU-01; motor de búsqueda; D6 y D7.

**Trazabilidad:** RF-02, RNF-08, RNF-09, RN-06.

1. **Dado** vehículos indexados y filtros válidos, **cuando** consulta por marca, categoría, transmisión o plazas, **entonces** obtiene resultados paginados y ordenados por tarifa, respetando los filtros.
2. **Dado** un vehículo mostrado en resultados, **cuando** el cliente decide reservarlo, **entonces** la confirmación vuelve a consultar precio y agenda autoritativos; el resultado de búsqueda no garantiza una reserva.

### HU-03 — Registrar clientes

Como **operador**, quiero registrar el perfil y la habilitación del conductor para permitir reservas a clientes identificados.

**Épica:** E-02. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: soporte del flujo de reserva; PG: gestión completa. **Dependencias:** HU-18; definición de datos obligatorios de Clientes.

**Trazabilidad:** RF-03, RNF-05, RN-02, RN-12.

1. **Dado** un operador autorizado y datos completos según las reglas acordadas, **cuando** registra un cliente, **entonces** obtiene un perfil identificable cuya habilitación puede consultar Alquileres.
2. **Dado** un cliente bloqueado o una licencia que no satisface las condiciones del período, **cuando** se solicita una reserva, **entonces** la validación rechaza la confirmación; registrar un perfil no lo habilita automáticamente.

### HU-04 — Consultar precio y disponibilidad

Como **cliente**, quiero consultar la disponibilidad y cotización de un auto por período para evaluar una reserva.

**Épica:** E-02. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: lectura real integrada. **Dependencias:** HU-01; agenda de Alquileres.

**Trazabilidad:** RF-04, RNF-01, RNF-09, RN-01, RN-03, RN-06.

**Evidencia actual:** Comportamiento parcial verificado en el mock; falta persistencia e integración real.

1. **Dado** un auto existente y un período válido [inicio, fin), **cuando** consulta disponibilidad, **entonces** recibe la disponibilidad y el total en centavos ARS, calculado como días por tarifa diaria.
2. **Dado** fechas imposibles, duración cero o duración mayor a 30 días, **cuando** realiza la consulta, **entonces** obtiene un rechazo de validación; la consulta nunca ocupa el auto.

### HU-05 — Confirmar una reserva

Como **cliente**, quiero confirmar una reserva con precio aceptado para asegurar la asignación del vehículo.

**Épica:** E-02. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: operación persistida. **Dependencias:** HU-03; HU-04; HU-18; D4.

**Trazabilidad:** RF-05, RNF-01, RNF-02, RNF-03, RN-01, RN-02, RN-03, RN-04, RN-05, RN-06.

**Evidencia actual:** Concurrencia e idempotencia verificadas en el mock de un proceso; falta garantía durable.

1. **Dado** un cliente habilitado, un período libre y el importe vigente, **cuando** dos solicitudes válidas compiten por el mismo auto y período, **entonces** una sola se confirma y la otra recibe conflicto, sin reservas duplicadas.
2. **Dado** una creación identificada previamente, **cuando** se repite con la misma identidad de consumidor, clave y carga, **entonces** se recupera su respuesta original; otra carga con la misma clave se rechaza.
3. **Dado** un importe esperado distinto de la cotización autoritativa, **cuando** intenta confirmar, **entonces** se rechaza el precio y no se ocupa el período.

### HU-06 — Consultar y cancelar reservas

Como **cliente**, quiero consultar y cancelar una reserva propia antes del retiro para administrar mi solicitud.

**Épica:** E-02. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2. **Dependencias:** HU-05; HU-18.

**Trazabilidad:** RF-06, RNF-01, RNF-02, RNF-05, RN-05, RN-07, RN-12.

**Evidencia actual:** Consulta y cancelación verificadas en el mock; faltan identidad individual y persistencia.

1. **Dado** una reserva propia en RESERVED, **cuando** solicita cancelarla, **entonces** se conserva el registro con estado CANCELLED y se libera la agenda una sola vez.
2. **Dado** una reserva CANCELLED, ACTIVE o COMPLETED, **cuando** se solicita cancelar, **entonces** CANCELLED admite repetición; ACTIVE y COMPLETED se rechazan sin alterar el estado.
3. **Dado** un usuario sin permiso sobre la reserva, **cuando** la consulta o intenta modificarla, **entonces** el sistema rechaza el acceso.

### HU-07 — Registrar el retiro

Como **operador**, quiero registrar el retiro de un vehículo reservado para iniciar su alquiler.

**Épica:** E-03. **Tipo:** Historia funcional. **Prioridad:** P2. **Estado:** Diseñada.

**Hito objetivo:** PG. **Dependencias:** HU-05; HU-18; revalidación del conductor.

**Trazabilidad:** RF-07, RNF-01, RNF-02, RN-02, RN-08, RN-10.

1. **Dado** una reserva vigente y un conductor habilitado, **cuando** registra el retiro y no existe otro alquiler activo del auto, **entonces** la reserva pasa de RESERVED a ACTIVE una sola vez.
2. **Dado** otro alquiler activo del mismo auto, incluso con devolución prevista vencida, **cuando** intenta registrar el retiro, **entonces** se rechaza la entrega hasta que se registre la devolución del alquiler anterior.

### HU-08 — Registrar la devolución

Como **operador**, quiero registrar la devolución física del auto para finalizar el alquiler y actualizar su ocupación.

**Épica:** E-03. **Tipo:** Historia funcional. **Prioridad:** P2. **Estado:** Diseñada.

**Hito objetivo:** PG. **Dependencias:** HU-07; HU-18.

**Trazabilidad:** RF-08, RNF-01, RNF-02, RN-09, RN-10.

1. **Dado** un alquiler ACTIVE, **cuando** registra la devolución, **entonces** pasa a COMPLETED y la devolución queda registrada una sola vez.
2. **Dado** un alquiler ya devuelto o cuya fecha prevista solamente ha vencido, **cuando** repite la devolución o consulta su estado, **entonces** la repetición no duplica el registro; el vencimiento por sí solo no finaliza el alquiler.

### HU-09 — Bloquear períodos por mantenimiento

Como **operador**, quiero registrar bloqueos de agenda para impedir reservas durante mantenimiento.

**Épica:** E-03. **Tipo:** Historia funcional. **Prioridad:** P2. **Estado:** Diseñada.

**Hito objetivo:** PG. **Dependencias:** HU-04; HU-18; agenda transaccional.

**Trazabilidad:** RF-09, RNF-01, RN-03, RN-11.

1. **Dado** un vehículo y período sin conflicto, **cuando** registra mantenimiento, **entonces** Alquileres ocupa ese período y las nuevas reservas incompatibles se rechazan.
2. **Dado** un período que se solapa con una reserva o bloqueo existente, **cuando** intenta agregar mantenimiento, **entonces** recibe un conflicto; no se cancelan reservas silenciosamente.

### HU-10 — Registrar candidatos

Como **responsable de personal**, quiero registrar una posible incorporación para gestionar su revisión y seguimiento.

**Épica:** E-04. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: modelo de personal; PG: flujo completo. **Dependencias:** HU-18; definición de datos internos de candidatos.

**Trazabilidad:** RF-10, RNF-05, RN-13, RN-17, RN-20.

1. **Dado** datos internos de un candidato aprobados para su registro, **cuando** se registra una posible incorporación, **entonces** Empleados conserva su identidad y seguimiento como candidato, separado de la condición de empleado.
2. **Dado** un candidato registrado o una revisión con turno confirmado, **cuando** se consulta la lista destinada al gimnasio, **entonces** el candidato no aparece por ese solo hecho; no se infiere contratación ni aptitud médica.

### HU-11 — Gestionar empleados

Como **responsable de personal**, quiero mantener el registro de empleados de la concesionaria para organizar los servicios vinculados al personal.

**Épica:** E-04. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: servicio Empleados operativo como objetivo. **Dependencias:** HU-18; reglas internas de alta y cambios de personal.

**Trazabilidad:** RF-11, RNF-01, RNF-05, RN-13, RN-14, RN-20.

1. **Dado** una decisión de alta autorizada y datos internos válidos, **cuando** se registra un empleado, **entonces** Empleados conserva el registro y sus cambios; la condición de empleado proviene de esa decisión explícita.
2. **Dado** un cambio de empleado registrado, **cuando** se prepara la lista para el gimnasio, **entonces** se usa la información vigente del ámbito acordado; no se accede directamente a bases de otros servicios.

### HU-12 — Publicar la lista de empleados al gimnasio

Como **aplicación del gimnasio autorizada**, quiero consultar la lista de empleados acordada para gestionar el beneficio de membresía.

**Épica:** E-05. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: capacidad compartida operativa como objetivo. **Dependencias:** HU-11; HU-18; contrato con gimnasio y publicación acordados.

**Trazabilidad:** RF-12, RNF-05, RNF-12, RN-13, RN-14, RN-20.

1. **Dado** el contrato de lista acordado y un consumidor autorizado, **cuando** solicita la lista dentro de su ámbito, **entonces** recibe los empleados correspondientes con únicamente los datos mínimos pactados.
2. **Dado** candidatos o información de revisiones clínicas en Empleados, **cuando** el gimnasio obtiene la lista, **entonces** esos datos quedan excluidos de la capacidad; no se publica información médica.

### HU-13 — Recibir membresías, uso y costos

Como **responsable de personal**, quiero recibir o consultar al gimnasio membresías, uso, costos individuales y gastos por período para controlar beneficios y costos.

**Épica:** E-05. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: acuerdo y primer intercambio; PG: flujo completo. **Dependencias:** HU-12; acuerdo del mecanismo de devolución del gimnasio.

**Trazabilidad:** RF-13, RNF-02, RNF-03, RNF-05, RNF-12, RN-14, RN-15, RN-16, RN-19, RN-20.

1. **Dado** un empleado, un período y una comunicación válida conforme al contrato acordado, **cuando** el gimnasio informa uso o no uso, **entonces** se conserva el resultado asociado al período y la evidencia de origen acordada.
2. **Dado** una comunicación duplicada, antigua, incompleta o una dependencia indisponible, **cuando** se procesa o falta la información, **entonces** no se duplica ni sustituye un resultado más reciente indebidamente; la ausencia de evidencia se distingue de no uso confirmado.

### HU-14 — Consultar el informe de membresías y gastos

Como **responsable de personal**, quiero consultar afiliación, uso, costo individual, gastos por empleado y costos totales por período para revisar el beneficio del gimnasio.

**Épica:** E-05. **Tipo:** Historia funcional. **Prioridad:** P2. **Estado:** Diseñada.

**Hito objetivo:** PG; definición en E2. **Dependencias:** HU-11; HU-13.

**Trazabilidad:** RF-14, RNF-05, RNF-07, RN-15, RN-16.

1. **Dado** un período con empleados y evidencia válida de uso o no uso, **cuando** consulta el informe, **entonces** se muestran los grupos y conteos correspondientes con referencia al período y a su evidencia.
2. **Dado** empleados sin información confirmada del gimnasio, **cuando** consulta el mismo informe, **entonces** aparecen como desconocidos y no como no usuarios; el informe no calcula tarifas, cobros ni gastos monetarios no acordados.

3. **Dado** datos económicos válidos del gimnasio, **cuando** se incorporan y consulta el informe, **entonces** se vinculan al empleado y se muestran costo de membresía, gasto individual y total del período por moneda según el contrato.
4. **Dado** un cargo duplicado o corregido, **cuando** se actualiza el informe, **entonces** no se acumula dos veces el mismo gasto y la corrección conserva trazabilidad.
5. **Dado** información económica faltante, **cuando** se consulta el total, **entonces** se identifica como incompleto; uso, ausencia de uso y falta de costo no se equiparan a un importe cero.

Los criterios 3 a 5 también aplican a HU-13 en lo que respecta a la incorporación de datos económicos.

### HU-15 — Solicitar revisión o turno a la clínica

Como **responsable de personal**, quiero solicitar por la API del hospital de Octi un turno para un empleado registrado que necesita consulta médica. El flujo adicional de candidatos se conserva separado.

**Épica:** E-06. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: acuerdo de consumo y primer flujo; PG: integración completa. **Dependencias:** HU-11; HU-18; contrato publicado por el hospital; HU-10 para el flujo adicional de candidatos.

**Trazabilidad:** RF-15, RNF-02, RNF-03, RNF-05, RNF-12, RN-17, RN-18, RN-19, RN-20.

1. **Dado** un empleado registrado y los datos mínimos requeridos por el contrato acordado, **cuando** se solicita una revisión o turno, **entonces** se conserva la solicitud y el resultado que el proveedor efectivamente confirme; pedir un turno no acredita confirmación ni asistencia.
2. **Dado** una respuesta perdida o un timeout después de solicitar, **cuando** finaliza la espera, **entonces** se informa resultado pendiente de comprobación y no se crea otro turno a ciegas; el modo de recuperación respeta el contrato de la clínica.

### HU-16 — Conciliar el estado del turno

Como **responsable de personal**, quiero comprobar el estado de las solicitudes de revisión para distinguir turnos confirmados, ausencia informada y casos desconocidos.

**Épica:** E-06. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: diseño de recuperación; PG: flujo completo. **Dependencias:** HU-15; mecanismo de consulta o notificación que acuerde la clínica.

**Trazabilidad:** RF-16, RNF-02, RNF-03, RNF-07, RNF-12, RN-17, RN-18, RN-19, RN-20.

1. **Dado** una solicitud con respuesta tardía o estado no confirmado, **cuando** el mecanismo acordado obtiene evidencia vigente, **entonces** se actualiza la clasificación y se conserva la relación con la solicitud y el origen.
2. **Dado** una caída del proveedor, falta de evidencia o respuesta inválida, **cuando** se concilia el estado, **entonces** permanece desconocido; solamente una ausencia informada válidamente permite clasificar sin turno.

3. **Dado** un turno vinculado a un empleado, **cuando** el hospital notifica o permite consultar una cancelación, reprogramación o cambio de estado, **entonces** Empleados actualiza referencia, estado y fecha conservando historia.
4. **Dado** una actualización repetida o anterior, **cuando** se recibe, **entonces** no duplica efectos ni revierte evidencia vigente según la regla de orden acordada.
5. **Dado** un turno cancelado, **cuando** se actualiza su estado, **entonces** no se crea automáticamente otro turno ni se elimina la solicitud original.

### HU-17 — Consultar el informe de turnos

Como **responsable de personal**, quiero consultar empleados y, por separado, posibles incorporaciones con turno, sin turno y con estado desconocido para gestionar revisiones pendientes.

**Épica:** E-06. **Tipo:** Historia funcional. **Prioridad:** P2. **Estado:** Diseñada.

**Hito objetivo:** PG; definición en E2. **Dependencias:** HU-11; HU-16; HU-10 para candidatos.

**Trazabilidad:** RF-17, RNF-05, RNF-07, RN-13, RN-17, RN-18.

1. **Dado** solicitudes con evidencia vigente de turno o ausencia informada, **cuando** consulta el informe, **entonces** obtiene clasificaciones con turno y sin turno respaldadas por dicha evidencia.
2. **Dado** solicitudes sin resultado confirmado, **cuando** consulta el informe, **entonces** se muestran como desconocidas; un turno no acredita asistencia, aptitud médica ni contratación.

### HU-18 — Gestionar acceso y permisos

Como **administrador del sistema**, quiero establecer identidad y permisos por capacidad para limitar cada operación al actor autorizado.

**Épica:** E-07. **Tipo:** Habilitador técnico. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: acceso a capacidad publicada; PG: roles completos. **Dependencias:** matriz de permisos; acuerdo de identidad de consumidores.

**Trazabilidad:** RF-18, RNF-05, RNF-12, RN-12, RN-14.

1. **Dado** una identidad válida y permiso sobre la operación y recurso, **cuando** realiza una solicitud, **entonces** el servicio propietario autoriza el acceso dentro del ámbito asignado.
2. **Dado** una identidad inexistente, permiso insuficiente o intento de suplantación de identidad, **cuando** accede al recurso, **entonces** se rechaza el acceso y se registra el resultado sin exponer credenciales ni datos sensibles.

### HU-19 — Balancear dos réplicas de Flota

Como **equipo de operaciones**, quiero distribuir las consultas de Flota entre dos réplicas y retirar instancias no disponibles para sostener el acceso al catálogo.

**Épica:** E-07. **Tipo:** Habilitador técnico. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: diseño y primera demostración; defensa: evidencia completa. **Dependencias:** HU-01; HU-20; D12; mecanismo de disponibilidad por definir.

**Trazabilidad:** RNF-06, RNF-07, RN-20.

1. **Dado** dos instancias de Flota disponibles detrás del Load Balancer, **cuando** se ejecuta una secuencia de solicitudes, **entonces** la evidencia muestra solicitudes atendidas por ambas instancias según el algoritmo configurado.
2. **Dado** una instancia detectada como no disponible, **cuando** continúan las consultas, **entonces** el balanceador deja de dirigirle nuevas solicitudes y se observa el cambio; una caída total se informa como indisponibilidad.

### HU-20 — Correlacionar logs y trazas

Como **equipo de desarrollo y operaciones**, quiero seguir una operación entre componentes para diagnosticar su resultado y sus dependencias.

**Épica:** E-08. **Tipo:** Habilitador técnico. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: logs y primera traza distribuida; PG/defensa: métricas y tablero completos. **Dependencias:** flujo operativo; D11.

**Trazabilidad:** RNF-07, RNF-05, RN-20.

1. **Dado** una solicitud que atraviesa el gateway y un servicio operativo, **cuando** se completa o falla, **entonces** los logs estructurados y la traza comparten correlación y permiten localizar operación, instancia, resultado y duración.
2. **Dado** una integración con resultado desconocido o una réplica caída, **cuando** se observa el sistema, **entonces** la telemetría identifica la condición sin convertirla en ausencia confirmada y sin registrar secretos o contenido médico.

### HU-21 — Verificar reglas, contratos y fallas

Como **equipo de desarrollo**, quiero demostrar consistencia y comportamiento ante fallas mediante pruebas reproducibles.

**Épica:** E-08. **Tipo:** Habilitador técnico. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: pruebas del flujo operativo; defensa: integración, carga y postmortem completos. **Dependencias:** historias implementadas; contratos acordados; dependencias reales.

**Trazabilidad:** RNF-01, RNF-02, RNF-03, RNF-06, RNF-08, RNF-09, RNF-10, RNF-12, RN-04, RN-05, RN-15, RN-18, RN-19, RN-20.

1. **Dado** una operación de reserva persistida y solicitudes concurrentes válidas, **cuando** se ejecuta la prueba, **entonces** una sola reserva ocupa el período y la evidencia conserva resultados e idempotencia tras reinicio.
2. **Dado** contratos externos acordados y fallas controladas del gimnasio o la clínica, **cuando** se repiten, retrasan o invalidan sus respuestas, **entonces** los tests distinguen ausencia informada y desconocimiento, comprueban duplicados y orden e identifican incompatibilidades.
3. **Dado** un ensayo de caída y carga definido, **cuando** se ejecuta y documenta, **entonces** se conservan resultados, limitaciones y análisis; el postmortem corresponde a un ensayo real.

### HU-22 — Reproducir el arranque

Como **integrante del grupo o evaluador**, quiero iniciar el sistema con un procedimiento documentado para reproducir la entrega desde el repositorio.

**Épica:** E-08. **Tipo:** Habilitador técnico. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: primer flujo y dependencias; PG: sistema completo. **Dependencias:** componentes implementados; configuración de entorno; publicación del repositorio.

**Trazabilidad:** RNF-11, RNF-05.

1. **Dado** una copia limpia del repositorio y los requisitos públicos documentados, **cuando** ejecuta el procedimiento único, **entonces** se crean o inician las dependencias y componentes declarados y puede probarse el flujo entregado.
2. **Dado** configuración sensible necesaria para la ejecución, **cuando** se prepara el entorno, **entonces** se suministra fuera del repositorio, sin pasos exclusivos de las computadoras del grupo.

### HU-23 — Identificar la necesidad de consulta de un empleado

Como **responsable de personal**, quiero recibir o consultar qué empleados requieren atención en el hospital de Octi para solicitar y seguir sus turnos.

**Épica:** E-06. **Tipo:** Historia funcional. **Prioridad:** P1. **Estado:** Diseñada, no implementada.

**Hito objetivo:** PG; acuerdo y diseño en E2. **Dependencias:** HU-11; HU-18; EXT-02 y EXT-15.

**Trazabilidad:** RF-19, RNF-02, RNF-05, RNF-12, RN-19, RN-21.

1. **Dado** un empleado registrado y una indicación válida del hospital, **cuando** se recibe o consulta la necesidad, **entonces** queda vinculada al empleado con origen y vigencia y puede iniciar HU-15.
2. **Dado** información repetida, desactualizada o una identidad no vinculada, **cuando** se procesa, **entonces** no se duplican necesidades ni se solicita un turno para otra persona; los casos sin evidencia suficiente quedan pendientes.
3. **Dado** una falla del hospital, **cuando** no se puede actualizar la necesidad, **entonces** se conserva la última evidencia con su fecha y se indica que falta verificarla.

### HU-24 — Publicar y consumir eventos de dominio

Como **equipo de desarrollo**, quiero que los cambios de Flota y los alquileres finalizados se comuniquen por eventos para que el índice y el historial se actualicen sin acoplar los servicios.

**Épica:** E-09. **Tipo:** Habilitador técnico. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: primer evento publicado y consumido; PG: idempotencia y mensajes fallidos completos. **Dependencias:** HU-01; HU-08; broker de mensajería; D3 y D5 vigentes.

**Trazabilidad:** RF-20, RNF-01, RNF-02, RNF-03, RNF-07, RNF-10.

1. **Dado** un cambio de negocio confirmado en el servicio propietario, **cuando** se persiste, **entonces** la intención de publicar `VehicleChanged.v1` o `RentalCompleted.v1` se registra en la misma transacción y se entrega al broker después; si el broker no está disponible, el evento queda pendiente y el cambio de negocio se conserva.
2. **Dado** un mensaje entregado más de una vez o fuera de orden, **cuando** el consumidor lo procesa, **entonces** el efecto se aplica una sola vez y una versión anterior no revierte una más reciente.
3. **Dado** un mensaje inválido o que agotó los reintentos acordados, **cuando** el consumidor no puede procesarlo, **entonces** se aparta en la cola de mensajes fallidos con su causa, se genera una alerta y puede reprocesarse sin duplicar efectos.

### HU-25 — Mantener sincronizado y reconstruible el índice de búsqueda

Como **operador**, quiero que los cambios de un vehículo aparezcan en la búsqueda dentro de un retraso acotado, y poder reconstruir el índice, para que los clientes vean datos vigentes.

**Épica:** E-09. **Tipo:** Habilitador técnico. **Prioridad:** P2. **Estado:** Diseñada.

**Hito objetivo:** E2: diseño y primera sincronización; PG: reconstrucción y medición. **Dependencias:** HU-01; HU-02; HU-24; D6.

**Trazabilidad:** RF-21, RNF-07, RNF-08, RNF-10.

1. **Dado** un cambio confirmado de un vehículo, **cuando** el indexador lo procesa, **entonces** el cambio aparece en los resultados dentro del retraso máximo objetivo (5 segundos en operación normal, a validar) sin necesidad de ejecutar búsquedas.
2. **Dado** un índice vacío, desactualizado o dañado, **cuando** el operador ejecuta la reconstrucción, **entonces** el índice se regenera desde la fuente de Flota aplicando identificador y versión, sin interrumpir las lecturas por identificador.
3. **Dado** el motor de búsqueda o el indexador indisponible, **cuando** se registran cambios, **entonces** los cambios quedan pendientes, el retraso es observable y la confirmación de reservas no depende del índice.

### HU-26 — Acelerar la consulta de fichas con caché medida

Como **cliente**, quiero que la consulta de la ficha de un vehículo responda rápido para decidir con fluidez, con una mejora demostrada con métricas.

**Épica:** E-09. **Tipo:** Habilitador técnico. **Prioridad:** P2. **Estado:** Diseñada.

**Hito objetivo:** E2: justificación y política; PG: comparación antes/después. **Dependencias:** HU-01; HU-20; D7.

**Trazabilidad:** RF-22, RNF-07, RNF-09, RNF-10.

1. **Dado** una entrada de ficha presente y vigente en la caché (TTL propuesto de 60 segundos, a validar), **cuando** se solicita la ficha, **entonces** se responde desde la caché y las métricas registran aciertos, latencia y accesos a almacenamiento antes y después de incorporarla.
2. **Dado** una ficha modificada, **cuando** se confirma el cambio, **entonces** se invalida la entrada según D7; después de la invalidación o del vencimiento, y resueltas las carreras de carga y actualización previstas, la consulta obtiene la versión vigente. Antes de ese momento puede existir el desfase permitido por la política; no se garantiza frescura instantánea.
3. **Dado** la caché indisponible, **cuando** se consulta una ficha, **entonces** se intenta la fuente con capacidad acotada y se informa degradación o indisponibilidad controlada si no admite la carga; la confirmación de una reserva siempre usa la tarifa autoritativa, sin caché.

### HU-27 — Protegerse de la caída o lentitud de una dependencia

Como **equipo de operaciones**, quiero que el sistema limite el impacto de una dependencia caída o lenta para que las operaciones independientes sigan funcionando y el resultado sea honesto.

**Épica:** E-07. **Tipo:** Habilitador técnico. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: mecanismo inicial; defensa: caída provocada y `docs/POSTMORTEM.md`. **Dependencias:** HU-05; HU-15; HU-20; D5, D9 y D10.

**Trazabilidad:** RNF-03, RNF-04, RNF-07, RNF-10; RN-18, RN-19, RN-20.

1. **Dado** una dependencia que supera su tiempo de espera o falla repetidamente, **cuando** el umbral configurado se alcanza, **entonces** el circuito se abre, las llamadas siguientes fallan de inmediato sin saturar recursos y el sistema prueba la recuperación de forma periódica.
2. **Dado** el gimnasio o la clínica fuera de servicio, **cuando** se opera el alquiler de autos, **entonces** la reserva continúa y los informes de personal indican resultado pendiente o desconocido, nunca ausencia confirmada.
3. **Dado** una caída provocada de un componente durante un ensayo, **cuando** finaliza, **entonces** `docs/POSTMORTEM.md` registra impacto, línea temporal, detección, respuesta, causa y acciones de mejora.

### HU-28 — Medir capacidad y costos con una prueba de carga

Como **equipo de desarrollo**, quiero ejecutar una prueba de carga documentada para conocer el límite principal del sistema, la alternativa de escalado y una estimación de costos.

**Épica:** E-08. **Tipo:** Habilitador técnico. **Prioridad:** P1. **Estado:** Diseñada.

**Hito objetivo:** E2: medición inicial documentada, límite principal identificado, alternativa de escalado y estimación de costos con fuente y fecha; defensa: ampliar y repetir el análisis sobre el sistema final. **Dependencias:** HU-04; HU-05; HU-20; D13. HU-19 se incorpora al escenario comparativo de réplicas cuando esté operativo; no bloquea la medición inicial del flujo real disponible.

**Trazabilidad:** RNF-06, RNF-07, RNF-10.

1. **Dado** un escenario de carga definido sobre consulta de disponibilidad y confirmación de reserva, **cuando** se ejecuta, **entonces** se registran configuración, latencia, errores y recursos, y se identifica el límite principal.
2. **Dado** los resultados obtenidos, **cuando** se analizan, **entonces** se documenta la alternativa de escalado y una estimación de costos con su fuente, sin atribuir mediciones al mock ni a las demostraciones de las clases.

## 5. Acuerdos previos a implementar integraciones

| Acuerdo pendiente | Historias afectadas | Condición de preparación |
| --- | --- | --- |
| Lista para el gimnasio | HU-12 | Definir datos mínimos, alcance, identidad, actualización y contrato formal; separar candidatos y datos clínicos |
| Devolución del gimnasio | HU-13, HU-14 | Acordar quién comunica, por qué mecanismo, período, evidencia de uso/no uso, duplicados, orden y correcciones |
| Capacidad de turnos de la clínica | HU-15 a HU-17, HU-23 | Obtener contrato publicado; distinguir solicitud, confirmación y ausencia informada; acordar recuperación e identificación |
| Datos internos y permisos | HU-03, HU-10, HU-11, HU-18 | Definir datos obligatorios y matriz de acceso, conservando minimización y separación de responsabilidades |
| Objetivos de operación | HU-19 a HU-21, HU-27, HU-28 | Registrar balanceo, detección de fallas, tiempos de espera, alertas y mediciones en los ADR correspondientes |

Un resultado desconocido conserva esa clasificación hasta contar con evidencia válida. Ni la falta de comunicación del gimnasio equivale a no uso, ni una caída de la clínica equivale a ausencia de turno. Los informes apoyan decisiones humanas: no calculan precios externos no acordados, no determinan aptitud médica y no generan contrataciones automáticas.

## 6. Referencias de diseño

- [SPEC.md](../SPEC.md): reglas, estados y alcance.
- [REQUERIMIENTOS.md](REQUERIMIENTOS.md): requisitos y verificación.
- [ARCHITECTURE.md](ARCHITECTURE.md): servicios, comunicaciones y patrones internos.
- [D1 vigente — límites](adr/ADR-014-limites-servicios-v2.md), [D3 vigente — persistencia](adr/ADR-015-persistencia-v2.md) y [D5 vigente — comunicación](adr/ADR-016-comunicacion-v2.md): propiedad y tratamiento de fallas.
- [Enunciado](../Enunciado%20TP%20Final.md): requisitos 2.1, 2.3, 2.5–2.12, integración entre grupos y Entrega 2.
- [clases.zip](../clases.zip): teoría 02 (datos), 04 (idempotencia/outbox), 06 (gateway y balanceo), 07 (resultados desconocidos y resiliencia) y 08 (capas, Hexagonal y dependencias).

Los patrones se seleccionan por responsabilidad y costo, según teoría 08; el backlog especifica comportamientos y evidencia. La distinción entre gateway y balanceador se mantiene según teoría 06.

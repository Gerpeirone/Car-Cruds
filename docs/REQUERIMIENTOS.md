# Requerimientos del sistema — Car Cruds

| Campo | Valor |
| --- | --- |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Diseño y preparación documental de la Entrega 2 |
| Hito objetivo | 23 de octubre de 2026 |
| Relación con el alcance | Alquileres como negocio principal; Clientes, Flota, Alquileres y Empleados |

## 1. Criterios de especificación

Los requerimientos funcionales expresan capacidades del sistema y los no funcionales establecen garantías y condiciones de operación verificables. Los identificadores RF-01 a RF-18 y RNF-01 a RNF-12 se mantienen estables para trazar implementación, pruebas y cambios. La metodología se completa con [BACKLOG.md](BACKLOG.md), cuyas historias incluyen criterios Dado/Cuando/Entonces, y con las reglas de [SPEC.md](../SPEC.md).

La versión 0.2 incorpora personal, la lista de empleados consumida por el gimnasio, el seguimiento del uso de membresía y el consumo de turnos de la clínica para posibles incorporaciones. Empleados conserva estas responsabilidades y Alquileres conserva reservas, agenda, retiro y devolución. La preparación documental no agrega componentes ejecutables: solo está disponible el mock de reservas de la primera entrega.

Los acuerdos con los otros grupos aún no fijan contratos, campos, protocolos ni mecanismos de devolución. El comportamiento esperado se especifica sin imponer esas decisiones externas. Los informes permiten seguimiento y decisiones humanas; no determinan aptitud médica, contratación automática ni gastos monetarios a partir de tarifas inexistentes en los acuerdos.

## 2. Requerimientos funcionales

### RF-01 — Administrar catálogo

El sistema deberá permitir al operador autorizado registrar y modificar características y tarifa vigente de vehículos en Flota. Los cambios conservarán las reservas y sus importes acordados.

**Verificación:** Consulta posterior del cambio; modificación de tarifa sin recalcular reservas existentes.

**Trazabilidad:** HU-01; RN-06. **Estado:** Diseñado.

### RF-02 — Buscar vehículos

El sistema deberá ofrecer búsqueda paginada por marca, categoría, transmisión y plazas, con ordenamiento por tarifa. La búsqueda no confirmará precio final ni disponibilidad temporal.

**Verificación:** Resultados que respeten filtros y orden; confirmación contra fuentes autoritativas.

**Trazabilidad:** HU-02; RN-03, RN-06. **Estado:** Diseñado.

### RF-03 — Registrar clientes

El sistema deberá mantener perfiles identificables y habilitación del conductor en Clientes, para validar la reserva según su estado y licencia registrada.

**Verificación:** Cliente habilitado consultable; conductor bloqueado o sin condiciones válidas rechazado.

**Trazabilidad:** HU-03; RN-02, RN-12. **Estado:** Diseñado.

### RF-04 — Consultar disponibilidad y cotización

El sistema deberá calcular disponibilidad y total para un vehículo y período válido [inicio, fin), en días calendario y centavos ARS, sin retener el vehículo.

**Verificación:** Cotización reproducible; rechazo de fechas o duración inválidas; ausencia de ocupación por consulta.

**Trazabilidad:** HU-04; RN-01, RN-03, RN-06. **Estado:** Diseñado.

### RF-05 — Confirmar reservas

El sistema deberá confirmar una reserva solo si cliente, período, precio y agenda son válidos. La exclusión por auto/período y la respuesta idempotente se conservarán en una operación durable.

**Verificación:** Prueba concurrente con una sola ganadora; precio cambiado rechazado; repetición estable y colisión de clave rechazada.

**Trazabilidad:** HU-05; RN-01 a RN-06. **Estado:** Diseñado.

### RF-06 — Consultar y cancelar reservas

El sistema deberá permitir consultar reservas dentro del ámbito autorizado y cancelar RESERVED. Repetir sobre CANCELLED conservará el resultado; ACTIVE y COMPLETED no admitirán cancelación.

**Verificación:** Estado actual consultable; cancelación libera una vez; rechazo de acceso y estados no permitidos.

**Trazabilidad:** HU-06; RN-05, RN-07, RN-12. **Estado:** Diseñado.

### RF-07 — Registrar retiro

El sistema deberá iniciar el alquiler desde una reserva vigente, revalidando al conductor y verificando que el auto no conserve otro alquiler activo, aun con devolución prevista vencida.

**Verificación:** Transición RESERVED → ACTIVE; intento incompatible rechazado sin entrega ni segunda transición.

**Trazabilidad:** HU-07; RN-02, RN-08, RN-10. **Estado:** Diseñado.

### RF-08 — Registrar devolución

El sistema deberá finalizar un alquiler activo mediante devolución explícita. La repetición no duplicará el registro; el vencimiento previsto no finalizará el alquiler.

**Verificación:** Transición ACTIVE → COMPLETED una sola vez; vencimiento sin devolución conserva ACTIVE.

**Trazabilidad:** HU-08; RN-09, RN-10. **Estado:** Diseñado.

### RF-09 — Gestionar mantenimiento en agenda

El sistema deberá registrar bloqueos de mantenimiento en Alquileres, comprobando conflictos con reservas y otros bloqueos antes de ocupar el período.

**Verificación:** Bloqueo válido impide reservas; conflicto no cancela registros existentes.

**Trazabilidad:** HU-09; RN-03, RN-11. **Estado:** Diseñado.

### RF-10 — Registrar posibles incorporaciones

El sistema deberá mantener candidatos y su seguimiento en Empleados, diferenciándolos de los empleados. Un registro o turno clínico no implicará contratación ni aptitud médica.

**Verificación:** Candidato conservado por separado y excluido de la lista de empleados por esa sola condición.

**Trazabilidad:** HU-10; RN-13, RN-17, RN-20. **Estado:** Diseñado.

### RF-11 — Gestionar empleados

El sistema deberá mantener el registro y los cambios de empleados a partir de decisiones de personal autorizadas, con persistencia propia en Empleados.

**Verificación:** Alta autorizada consultable; candidato sin alta no tratado como empleado; cambios preservados.

**Trazabilidad:** HU-11; RN-13, RN-14, RN-20. **Estado:** Diseñado.

### RF-12 — Ofrecer lista al gimnasio

El sistema deberá permitir al gimnasio autorizado obtener la lista acordada de empleados, con el ámbito y los datos mínimos pactados. La capacidad excluirá candidatos e información clínica.

**Verificación:** Test del contrato pactado y de permisos; proyección sin datos ajenos al propósito acordado.

**Trazabilidad:** HU-12; RN-13, RN-14, RN-20. **Estado:** Diseñado.

### RF-13 — Incorporar uso de membresía

El sistema deberá incorporar la comunicación válida del gimnasio sobre uso o no uso de membresía, vinculada al empleado, período y evidencia de origen acordados. Falta de evidencia significará desconocido.

**Verificación:** Recepción conforme al mecanismo pendiente de acuerdo; pruebas de duplicado, orden y ausencia de respuesta.

**Trazabilidad:** HU-13; RN-14, RN-15, RN-16, RN-19, RN-20. **Estado:** Diseñado.

### RF-14 — Informar uso de membresía

El sistema deberá generar un informe por período con uso, no uso confirmado y casos desconocidos, para apoyar la revisión del beneficio y el control de gastos. No calculará tarifas ni importes externos no acordados.

**Verificación:** Conteos y agrupaciones coinciden con la evidencia; desconocidos permanecen separados del no uso.

**Trazabilidad:** HU-14; RN-15, RN-16. **Estado:** Diseñado.

### RF-15 — Solicitar revisión o turno

El sistema deberá permitir solicitar a la clínica una revisión para una posible incorporación, usando su contrato publicado y los datos mínimos acordados. Conservará solicitud y resultado sin equiparar petición, confirmación y asistencia.

**Verificación:** Solicitud trazable; confirmación solo con evidencia del proveedor; timeout sin afirmación de ausencia ni repetición insegura.

**Trazabilidad:** HU-15; RN-17, RN-18, RN-19, RN-20. **Estado:** Diseñado.

### RF-16 — Conciliar estado de solicitudes

El sistema deberá actualizar el seguimiento mediante el mecanismo pactado con la clínica, distinguiendo turno confirmado, ausencia informada y resultado desconocido. Preservará evidencia y orden de actualizaciones.

**Verificación:** Respuesta tardía válida puede resolver un desconocido; caída, invalidez o silencio no producen sin turno.

**Trazabilidad:** HU-16; RN-17, RN-18, RN-19, RN-20. **Estado:** Diseñado.

### RF-17 — Informar existencia de turno

El sistema deberá presentar candidatos con turno confirmado, sin turno informado y estado desconocido, respaldados por evidencia. El informe no determinará asistencia, aptitud médica ni contratación.

**Verificación:** Clasificación consistente con la conciliación y ausencia de conclusiones médicas o de contratación automáticas.

**Trazabilidad:** HU-17; RN-13, RN-17, RN-18. **Estado:** Diseñado.

### RF-18 — Autorizar operaciones

El sistema deberá comprobar identidad y permiso sobre operación, recurso y ámbito, para clientes, operadores, responsables de personal y aplicaciones externas, según la matriz de acceso acordada.

**Verificación:** Accesos permitidos y rechazados comprobados por rol y consumidor; identidad manipulada no aceptada.

**Trazabilidad:** HU-18; transversal a HU-01 a HU-17; RN-12, RN-14. **Estado:** Diseñado.

## 3. Requerimientos no funcionales

Los valores identificados como propuestas son objetivos de diseño que deben validarse; no son mediciones del sistema ni acuerdos de servicio externos. La verificación debe registrar configuración, carga y resultado utilizados.

### RNF-01 — Consistencia y durabilidad

Las operaciones críticas conservarán sus invariantes dentro del servicio propietario. La reserva, sus importes, su respuesta idempotente y la intención de publicar eventos se registrarán con garantías locales de persistencia.

**Verificación:** Dos confirmaciones válidas simultáneas del mismo período libre producen una reserva; reiniciar conserva el resultado confirmado. Las devoluciones, cancelaciones y bloqueos no pierden ni duplican cambios.

**Trazabilidad:** HU-05 a HU-09, HU-11, HU-21; RN-03 a RN-11. **Fuente:** 2.8; D3 y D4. **Estado:** Diseñado.

### RNF-02 — Idempotencia y protección del orden

Los reintentos identificables y las comunicaciones externas repetidas no producirán efectos duplicados. La actualización de un estado no retrocederá por una comunicación anterior; el mecanismo de identidad y orden externo deberá pactarse antes de integrar.

**Verificación:** Repetir una creación conserva su respuesta; variar la carga con la misma clave genera conflicto. Duplicados y actualizaciones fuera de orden del gimnasio o clínica preservan el resultado correcto y su evidencia.

**Trazabilidad:** HU-05 a HU-08, HU-13, HU-15, HU-16, HU-21; RN-05, RN-19. **Fuente:** 2.5, 2.8; D4 y D5. **Estado:** Diseñado.

### RNF-03 — Resiliencia y resultados desconocidos

Cada dependencia tendrá plazo total, tiempos de espera, responsable de reintentos y recuperación documentados. Los reintentos respetarán presupuesto e idempotencia. Timeout, silencio o respuesta inválida no se interpretarán como no uso o sin turno.

**Verificación:** Fallas controladas finalizan dentro del presupuesto configurado y dejan la operación en una condición honesta. La recuperación pactada resuelve resultados desconocidos cuando obtiene evidencia válida. Valores y umbrales externos siguen pendientes de acuerdo.

**Trazabilidad:** HU-05, HU-13, HU-15, HU-16, HU-21; RN-15, RN-18, RN-19. **Fuente:** 2.5, 2.9, 4.1; D5, D9 y D10. **Estado:** Diseñado.

### RNF-04 — Aislamiento de responsabilidades

Clientes, Flota, Alquileres y Empleados conservarán datos y casos de uso propios. La integración con gimnasio y clínica se originará en Empleados; la reserva de autos no dependerá de esas capacidades.

**Verificación:** Con gimnasio o clínica fuera de servicio y dependencias propias sanas, el flujo de alquiler continúa. Trazas y revisión de accesos comprueban que Alquileres no llama esas integraciones ni lee bases ajenas.

**Trazabilidad:** HU-05, HU-10 a HU-17, HU-21; RN-20. **Fuente:** 2.1, 4.1; D1, D2 y D9. **Estado:** Diseñado.

### RNF-05 — Seguridad y minimización

La publicación operativa protegerá identidad y acceso; los servicios autorizarán sus recursos. Secretos estarán fuera del repositorio. Listas e informes expondrán únicamente información necesaria para su audiencia, sin divulgar información clínica al gimnasio.

**Verificación:** Pruebas de permisos rechazan identidades o ámbitos inválidos. Revisión de repositorio, logs, respuestas e informes no encuentra secretos ni datos clínicos publicados al gimnasio. Campos externos se verificarán contra el acuerdo, sin anticiparlos.

**Trazabilidad:** HU-01, HU-03, HU-06, HU-10 a HU-18, HU-20, HU-22; RN-12, RN-14. **Fuente:** 2.12, 4.1; D8 y D9. **Estado:** Diseñado.

### RNF-06 — Balanceo y disponibilidad de Flota

El diseño objetivo tendrá al menos dos instancias de Flota detrás de un Load Balancer, con algoritmo, comprobación de disponibilidad y retirada documentados. La política de detección y recuperación se registrará en D12.

**Verificación:** Una secuencia de solicitudes se distribuye entre ambas instancias sanas. Tras marcar una instancia no disponible, ninguna nueva solicitud se dirige a ella; la evidencia muestra detección y recuperación según la política configurada.

**Trazabilidad:** HU-19, HU-20, HU-21; RN-20. **Fuente:** 2.3; D12. **Estado:** Diseñado.

### RNF-07 — Observabilidad y correlación

Las operaciones producirán logs estructurados y correlacionados, métricas y trazas suficientes para comprender resultado y dependencias. E2 incluirá la primera traza distribuida; métricas, tablero, objetivo de servicio y alertas se completarán según D11.

**Verificación:** Desde una correlación se identifica operación, servicio, instancia, duración y resultado. El tablero permite ver reparto y fallas de réplicas, retraso del índice y casos externos desconocidos. Los registros omiten secretos y contenido médico.

**Trazabilidad:** HU-14, HU-16, HU-17, HU-19 a HU-21; RN-15, RN-18, RN-20. **Fuente:** 2.3, 2.10 y 6.3; D11. **Estado:** Diseñado.

### RNF-08 — Sincronización y recuperación del índice

La búsqueda derivará de la fuente de Flota y se actualizará por cambios de datos, independientemente de consultas. Se conservará un procedimiento de reconstrucción y un objetivo de retraso documentado; la propuesta inicial es 5 segundos en operación normal, pendiente de medición en D6.

**Verificación:** Una modificación aparece en resultados dentro del objetivo acordado sin ejecutar búsquedas para actualizar. Reconstruir recupera documentos desde Flota y aplica identificador y versión para evitar retrocesos.

**Trazabilidad:** HU-01, HU-02, HU-21; RN-06. **Fuente:** 2.6; D6. **Estado:** Diseñado.

### RNF-09 — Caché justificada y precio autoritativo

La caché cubrirá una lectura relevante con vigencia, invalidación y degradación definidas. La propuesta inicial para fichas es TTL de 60 segundos, a validar en D7. La confirmación de reserva obtiene el precio autoritativo sin depender de una entrada cacheada.

**Verificación:** Se comparan latencia, accesos a almacenamiento y aciertos antes y después. Cambiar la ficha invalida o vence la entrada conforme a la política; confirmar nunca acepta una tarifa obsoleta por la caché.

**Trazabilidad:** HU-02, HU-04, HU-05, HU-21; RN-06. **Fuente:** 2.7; D7. **Estado:** Diseñado.

### RNF-10 — Pruebas y evidencia de fallas

La verificación incluirá reglas de negocio, integración por servicio contra dependencias reales, contratos externos acordados y carga. Se conservarán resultados analizados y un postmortem de una caída provocada, sin sustituir evidencia real por el mock.

**Verificación:** Las pruebas cubren concurrencia, persistencia, duplicados, orden y ausencia frente a desconocimiento. Los tests de contrato detectan cambios incompatibles. Los informes de carga y caída identifican resultados, límites y acciones.

**Trazabilidad:** HU-21; aplica a las historias funcionales implementadas; RN-04, RN-05, RN-15, RN-18, RN-19, RN-20. **Fuente:** 2.9, 2.11 y 4.1; D10, D11 y D13. **Estado:** Diseñado.

### RNF-11 — Arranque reproducible

Un procedimiento único, automatizado y documentado iniciará componentes y dependencias del alcance entregado desde una copia limpia. Los requisitos públicos y la configuración sensible estarán separados.

**Verificación:** Un evaluador reproduce el flujo operativo sin configuraciones privadas de las computadoras del grupo; el procedimiento crea o inicia el almacenamiento requerido y permite reconocer fallas de arranque.

**Trazabilidad:** HU-22; Aplicación transversal. **Fuente:** 2.12. **Estado:** Diseñado.

### RNF-12 — Contratos y compatibilidad

Toda capacidad entre grupos contará con contrato formal, estándar, versionado y procesable, ejemplos, errores y condiciones de uso. Campos, protocolos y modos de comunicación con gimnasio y clínica se definirán mediante acuerdo y se adaptarán al modelo interno.

**Verificación:** Un consumidor integra usando la documentación publicada y un test de contrato. La evolución se revisa por compatibilidad. Hasta el acuerdo, no se presenta un esquema, endpoint, evento o condición externa inventada como obligación del otro grupo.

**Trazabilidad:** HU-12, HU-13, HU-15, HU-16, HU-18, HU-21; RN-14 a RN-19. **Fuente:** 4.1; D5, D8 y D9. **Estado:** Diseñado.

## 4. Correspondencia de reglas de negocio

Las reglas RN-01 a RN-12 de SPEC mantienen su definición. Las nuevas reglas se referencian con los siguientes identificadores, sin convertir los conceptos internos en campos de un contrato externo:

| Regla en SPEC | Finalidad | Requisitos principales |
| --- | --- | --- |
| RN-13 | Separar candidato y empleado; alta autorizada | RF-10, RF-11, RF-12, RF-17 |
| RN-14 | Datos mínimos y ámbito para el gimnasio | RF-12, RF-13, RF-18; RNF-05, RNF-12 |
| RN-15 | Uso, no uso y desconocido por período y evidencia | RF-13, RF-14; RNF-03, RNF-07 |
| RN-16 | Evitar tarifas o gastos monetarios externos no acordados | RF-13, RF-14 |
| RN-17 | Solicitud de turno distinta de confirmación o asistencia | RF-10, RF-15, RF-16, RF-17 |
| RN-18 | Sin turno informado distinto de fallo o desconocimiento | RF-15, RF-16, RF-17; RNF-03 |
| RN-19 | Proteger repetición y orden de comunicaciones | RF-13, RF-15, RF-16; RNF-02 |
| RN-20 | Aislar alquileres de integraciones de personal | RF-10 a RF-13, RF-15, RF-16; RNF-04 |

La política de desconocimiento aplica a todo fallo de integración: la ausencia válida comunicada por el proveedor es un hecho de negocio; un timeout, respuesta inválida, falta de evidencia o interrupción del canal es una condición técnica. El sistema conserva la última evidencia y su vigencia, indicando los límites conocidos sin fabricar una conclusión nueva.

## 5. Pendientes de elicitación y acuerdo

| Identificador | Información pendiente | Requisitos afectados |
| --- | --- | --- |
| PA-01 | Datos mínimos, alcance de lista, identidad del consumidor y actualización de empleados para el gimnasio | RF-12, RF-18; RNF-05, RNF-12 |
| PA-02 | Forma de devolución del gimnasio: quién comunica, protocolo o modalidad, período de uso, evidencia, correcciones, duplicados y orden | RF-13, RF-14; RNF-02, RNF-03 |
| PA-03 | Operaciones reales publicadas por la clínica para pedir, confirmar y comprobar turnos; identidad y datos exigidos | RF-15, RF-16, RF-17; RNF-12 |
| PA-04 | Tratamiento pactado de respuesta perdida, ausencia informada, resultados desconocidos y reconciliación de turnos | RF-15, RF-16; RNF-02, RNF-03 |
| PA-05 | Datos internos de candidato y empleado, permisos y decisión explícita de alta; separación de acceso a información de revisiones | RF-10, RF-11, RF-18; RNF-05 |
| PA-06 | Objetivos por dependencia: presupuesto de tiempo, reintentos, alertas, disponibilidad y conservación de evidencias | RNF-03, RNF-06, RNF-07, RNF-12 |
| PA-07 | Métricas y resultados que justifiquen búsqueda, caché, capacidad y costos | RNF-08, RNF-09, RNF-10 |

Estos pendientes bloquean la implementación contractual afectada. Los actores pueden revisar capacidades y criterios de aceptación antes de que exista el esquema externo. El modo de devolución de uso del gimnasio permanece **pendiente de acuerdo**, sin asumir notificación, consulta periódica, API o mensajería.

## 6. Criterio de cumplimiento de la Entrega 2

| Resultado exigido por sección 6.3 del enunciado | Situación de esta preparación |
| --- | --- |
| Al menos un servicio operativo | Pendiente de implementación |
| Funcionalidad compartida funcionando | Lista de empleados definida como capacidad; contrato y operación pendientes |
| Al menos un tipo de almacenamiento integrado al flujo | Diseño previsto; falta integración real |
| Logs estructurados y correlacionados y primera traza distribuida | Requeridos por RNF-07; falta evidencia operativa |
| D2, D6, D7, D9, D10, D12 y D13; validación de D1/D3/D5; primera D11; revisión de D8 | Documentación en preparación; validación vinculada a implementación y acuerdos |

Ni estas tablas ni las pruebas existentes del mock acreditan el cumplimiento completo de E2. Los hitos y prioridades de las historias se registran en [BACKLOG.md](BACKLOG.md).

## 7. Referencias y fundamento de patrones

- [Enunciado académico](../Enunciado%20TP%20Final.md): requisitos de arquitectura, integración y Entrega 2.
- [SPEC.md](../SPEC.md): reglas RN-01 a RN-20 y estados del dominio.
- [BACKLOG.md](BACKLOG.md): E-01 a E-08 y HU-01 a HU-22.
- [ARCHITECTURE.md](ARCHITECTURE.md), [D1 vigente](adr/ADR-014-limites-servicios-v2.md), [D3 vigente](adr/ADR-015-persistencia-v2.md) y [D5 vigente](adr/ADR-016-comunicacion-v2.md): límites, propiedad de datos y comunicación.
- [clases.zip](../clases.zip): teoría 02 para consistencia local; teoría 04 para outbox, duplicados y orden; teoría 06 para distinguir gateway de Load Balancer; teoría 07 para presupuestos, fallas y resultados desconocidos; teoría 08 para capas, Hexagonal y aislamiento mediante puertos/adaptadores.

Los patrones internos deben proteger reglas y dependencias de cada servicio de forma reconocible. Las capas organizan presentación, lógica y datos; Hexagonal permite adaptar proveedores sin incorporar sus modelos al núcleo. Los contratos externos se traducen al lenguaje de Empleados y no determinan por sí solos un patrón ni una tecnología de implementación.

# Integraciones de personal — Car Cruds

**Actualización — 22/10/2026:** se conserva este diseño como referencia del alcance original. Car Cruds implementa la recepción propuesta de membresías, uso y gastos, su historia y el informe administrativo, además de la lista activa. Se ofrece HTTP entrante en nuestra API; su aceptación por Pablo continúa pendiente. Contrato y límites actuales: [COMUNICACION-GIMNASIO](COMUNICACION-GIMNASIO.md). El alta por POST al gimnasio fue retirada. El hospital queda para una etapa posterior.

**Equipos externos:** gimnasio del grupo de Pablo (consume nuestra lista de empleados) y hospital del grupo de Octi (Car Cruds consume su API). Las referencias a “Clínica” en el diseño identifican al hospital de Octi. Contratos, entornos y credenciales siguen pendientes de acuerdo.

| Campo | Valor |
| --- | --- |
| Versión | 0.2 |
| Fecha | 12 de octubre de 2026 |
| Estado | Alcance comunicado por el grupo; diseño propuesto y acuerdos pendientes |
| Responsable funcional | Servicio Empleados |

Este documento distingue lo comunicado por el grupo de las propuestas de Car Cruds. No sustituye un contrato del gimnasio o de la clínica. No se recibieron contratos externos, URLs, credenciales, tarifas ni resultados de pruebas de integración.

## 1. Capacidades y consecuencia de negocio

| Integración | Alcance comunicado | Resultado administrativo |
| --- | --- | --- |
| Gimnasio | Consume la lista de empleados; administra membresías y proporciona afiliación, uso, costos individuales, gastos y totales | Informe de membresías y gastos por empleado y totales por período, con evidencia y datos faltantes |
| Clínica | Car Cruds consume turnos para empleados registrados y conserva el flujo adicional de candidatos | Informe de personas con turno y sin turno, acompañado de la información que aún falta verificar |

El alquiler de autos conserva su flujo principal de clientes. El seguimiento de incorporaciones y beneficios constituye el flujo de gestión de personal de la concesionaria. La integración debe demostrarse con consecuencias en estos informes y su uso administrativo. La validación docente del alcance y de la relevancia de este flujo se registra como pendiente; una llamada aislada no basta.

## 2. Fronteras y propiedad

- **Empleados** conserva candidatos, empleados y la evidencia administrativa de intercambios externos. Un candidato no entra automáticamente en la lista compartida con el gimnasio.
- **Gimnasio** es la fuente del uso de membresías. Car Cruds no inventa asistencia, tarifas ni facturación del gimnasio.
- **Clínica** es la fuente de disponibilidad y confirmación de turnos. Car Cruds no deduce asistencia, diagnóstico ni aptitud médica a partir de un turno.
- **Clientes** administra conductores y licencias; **Alquileres** administra reservas y agenda; **Flota** administra vehículos y tarifas. Ninguno necesita consultar a gimnasio o clínica para confirmar una reserva.
- El acceso entrante pasa por el gateway y la autorización del recurso se aplica en Empleados. La llamada saliente a la clínica se origina en Empleados mediante su adaptador del proveedor, sin recorrer el punto de entrada propio.

## 3. Gimnasio: lista y devolución de uso

### Flujo previsto

1. El responsable de personal registra y mantiene empleados.
2. El gimnasio se autentica y consulta la población de empleados autorizada para la integración.
3. El gimnasio comunica evidencia de uso/no uso por persona y período mediante el mecanismo que acuerden ambos equipos.
4. Empleados verifica identidad del emisor, vínculo de la persona y consistencia de la información. Conserva evidencia y evita duplicar efectos.
5. El responsable obtiene un informe con uso informado, no uso informado e información faltante; se registra fecha de actualización y origen.

El diseño inicial dejaba abierto el transporte. La implementación actual ofrece una llamada entrante **a Car Cruds**, descrita en [COMUNICACION-GIMNASIO](COMUNICACION-GIMNASIO.md), pendiente de aceptación bilateral; no se inventa un endpoint del gimnasio. El borrador [employees-openapi-draft.json](contracts/employees-openapi-draft.json) se conserva histórico. El contrato propio implementado es [employees-openapi.json](contracts/employees-openapi.json), versión 0.2.0.

### Semántica interna propuesta

| Situación | Tratamiento previsto |
| --- | --- |
| Uso comunicado con identidad y período válidos | Registrar uso informado para esa evidencia |
| No uso comunicado explícitamente | Registrar no uso informado para ese período |
| Falta de devolución, error, identidad sin vincular o período ambiguo | Marcar información pendiente; no convertirla en no uso |
| Repetición de la misma comunicación | Conservar un solo efecto; mecanismo de identificación pendiente del contrato |
| Corrección o mensaje atrasado | Aplicar la política de versión/fecha y precedencia acordada; si no se puede resolver, conservar conflicto para revisión |

No se acumulan cantidades, consumos o gastos simplemente al recibir otra vez el mismo mensaje. Los datos de un período no se atribuyen automáticamente al siguiente. El significado de “usar la membresía” debe ser acordado: no se supone que afiliación, pago y asistencia sean equivalentes.

### Gastos

El alcance requerido incluye empleados con membresía, costo de cada membresía, gastos por empleado y costos totales por período. El gimnasio administra y proporciona la información económica; Car Cruds la recibe o consulta y presenta un informe administrativo. Afiliación, uso y gasto son hechos distintos: no se deduce un costo a partir de asistencia. Tarifas, vigencia, moneda, período facturado, población cobrada y reglas de imputación siguen pendientes. Los totales usarán registros únicos, separarán monedas y señalarán datos incompletos; una corrección reemplazará la evidencia anterior según el contrato sin sumar dos veces. No se generan pagos ni facturas automáticamente.

## 4. Clínica: solicitud y seguimiento de turnos

### Flujo previsto

1. Empleados identifica un empleado registrado que necesita consulta a partir de una indicación administrativa o información recibida/consultada al hospital. El mecanismo externo se acordará; el flujo de candidatos permanece separado.
2. Empleados conserva una referencia local de la intención y los datos mínimos autorizados antes de comunicarse con la clínica.
3. Su adaptador solicita el turno al hospital mediante las operaciones del contrato publicado, vinculando empleado, intención local y referencia de solicitud externa.
4. Si una respuesta válida confirma un turno, se conserva la referencia y el estado administrativo. Si falta confirmación, se mantiene la incertidumbre y se concilia según las operaciones disponibles.
5. El informe identifica personas con turno confirmado, personas sin turno confirmado con evidencia suficiente y personas con estado pendiente de verificar.

**Solicitar una consulta, obtener un turno y asistir son hechos distintos.** Car Cruds solicita el turno por la API del hospital. Se acordarán operaciones, confirmación y pasos adicionales del empleado. El hospital administra cambios de estado, cancelaciones y reprogramaciones; Car Cruds los refleja sin asumir control de su agenda.

### Recuperación ante fallas

| Situación | Tratamiento propuesto |
| --- | --- |
| Rechazo explícito del proveedor | Registrar el motivo administrativo permitido; no mostrar turno confirmado |
| Timeout después de enviar una solicitud | Resultado desconocido: conservar referencia y conciliar; no afirmar fracaso ni repetir a ciegas |
| Reintento de una creación | Solo si el contrato garantiza idempotencia con la misma intención; en otro caso, consulta/revisión antes de un nuevo envío |
| Consulta de estado fallida | Conservar la última evidencia y su fecha, indicar desactualización y permitir recuperación posterior |
| Turno cancelado o reprogramado | Actualizar conforme al contrato y conservar historia; no confundirlo con “nunca solicitó” |
| Proveedor caído | Limitar espera y concurrencia, indicar situación pendiente y conservar funciones de alquiler de vehículos |

Estos son estados administrativos internos propuestos. No representan nombres de campos, estados o códigos de la API de la clínica. Tampoco se definió un plazo externo como si fuera un SLA acordado.

## 5. Acuerdos pendientes

| ID | Acuerdo necesario | Interlocutor | Consecuencia para implementación |
| --- | --- | --- | --- |
| EXT-01 | Población de empleados compartida, activos/inactivos y cambios de altas/bajas | Gimnasio y responsable de personal | Define qué lista puede publicarse |
| EXT-02 | Identificadores y vinculación de personas; campos mínimos y permisos | Ambos equipos externos | Evita vincular información a la persona incorrecta |
| EXT-03 | Formato, versión, paginación y compatibilidad de la lista | Gimnasio | Permite cerrar el borrador de contrato propio |
| EXT-04 | Mecanismo y contrato de devolución/consulta de membresías, uso y costos | Gimnasio | Define adaptador, entradas y prueba de contrato |
| EXT-05 | Significado de uso/no uso, período, evidencia y correcciones | Gimnasio | Permite interpretar el informe sin inferencias |
| EXT-06 | Costos individuales, gastos por empleado, totales, moneda, período y reglas de imputación | Gimnasio y empresa | Bloquea el informe económico de gastos individuales y totales; el informe de uso puede avanzar parcialmente |
| EXT-07 | Contrato publicado de turnos y operaciones disponibles | Clínica | Define el adaptador de consumo y el test de contrato |
| EXT-08 | Solicitud desde Car Cruds, confirmación del hospital y pasos adicionales del empleado | Clínica y empresa | Define el flujo de revisión y la confirmación |
| EXT-09 | Consulta de estado, cancelación, reprogramación e idempotencia | Clínica | Define reconciliación y recuperación de respuestas perdidas |
| EXT-10 | Autenticación, autorización, entornos y URLs accesibles | Ambos equipos externos | Necesario para integración y publicación reales |
| EXT-11 | Datos administrativos permitidos, acceso y conservación | Empresa y equipos externos | Limita los datos compartidos y registrados |
| EXT-12 | Tiempos de espera, límites, disponibilidad y cambios compatibles | Ambos equipos externos | Permite fijar políticas operativas realistas |
| EXT-13 | Responsables por equipo y ejemplos de prueba controlados | Ambos equipos externos | Permite validar con el contrato vigente |
| EXT-15 | Información del hospital sobre empleados que requieren consulta: consulta/notificación, identidad, vigencia y correcciones | Hospital de Octi | Define identificación y seguimiento de la necesidad de consulta |
| EXT-14 | Validación docente de la ampliación y su flujo de negocio | Cátedra y grupo | Confirma correspondencia con el enunciado |

Todos estos acuerdos están **pendientes**, sin fechas ni aprobaciones inventadas. Los dos sistemas externos ya fueron identificados por el grupo; lo pendiente son sus condiciones de integración, no elegir sistemas diferentes.

## 6. Validación prevista y fuentes

- Pruebas de contrato sobre la lista propia y el contrato real de la clínica; devolución del gimnasio cuando se acuerde el mecanismo.
- Pruebas con duplicados, respuestas tardías, personas sin vínculo y datos de períodos diferentes.
- Ensayo de caída externa que preserve el flujo de alquiler y mantenga estados desconocidos honestos.
- Revisión de informes con evidencias positivas, negativas y faltantes; sin tarifas ni resultados médicos sintéticos presentados como reales.

La recepción y el informe del gimnasio cuentan con pruebas propias; no equivalen a una prueba conjunta con su sistema. El hospital permanece sin implementar. Ver [comunicación del gimnasio](COMUNICACION-GIMNASIO.md), [entrega 2](ENTREGA-2.md) y [ADR-017](adr/ADR-017-contrato-empleados.md).

Material de referencia en `clases.zip`: teoría 02 (datos y consistencia), 04 (idempotencia y mensajería), 06 (identidad y límites), 07 (resultado desconocido y recuperación) y 08 (puertos/adaptadores); práctica 01 (interfaces/mocks) y 02 (Repository).

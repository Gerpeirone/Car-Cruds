# Integraciones de personal — Car Cruds

> **Gimnasio — 10/11/2026:** el equipo de Pablo informó que implementó la consulta, la vinculación y el envío mensual. La [guía de integración](contracts/INTEGRACION_GIMNASIO.md) reúne la publicación de 0.4.0, la entrega privada del token con vencimiento y los pasos de la prueba conjunta.

> **Estado operativo — 08/11/2026:** contrato y adaptador hospitalarios integrados y probados en laboratorio. Acceso compartido, aceptación del gimnasio y ensayo conjunto siguen abiertos; [PENDIENTES-E2](PENDIENTES-E2.md) enumera los aportes necesarios sin exponer credenciales.

**Revisión vigente — 23/10/2026:** Empleados administra exclusivamente personal registrado; [ADR-019](adr/ADR-019-consumo-turnos-socios.md) adapta el flujo existente a Turnos para socios 1.0.0, fijado a `c0b65f4`. Nuevos vínculos, intenciones y cualquier envío que pueda crear requieren empleado activo; evidencia previa de inactivos se conserva. La adaptación agrega clave privada, idempotencia persistida, recuperación explícita y espera durable sin ampliar el producto hacia otras operaciones clínicas. La validación de la rama y la aceptación conjunta se registran por separado.

**Gimnasio — 23/10/2026:** según el mensaje recibido del gimnasio, el profesor confirmó lista → vinculación manual → evidencia. [ACUERDO-GIMNASIO](ACUERDO-GIMNASIO.md) responde sus preguntas, registra la elección de Car Cruds de costos por meses iguales y mantiene meses parciales pendientes. OpenAPI 0.4.0 agrega la consulta mínima de estado laboral y conserva la lista y la evidencia de 0.3.0. Perfil semántico, host y prueba conjunta siguen pendientes de confirmación bilateral.

**Actualización — 22/10/2026:** se conserva el alcance original. Gimnasio: lista, recepción propuesta, historia e informe en [COMUNICACION-GIMNASIO](COMUNICACION-GIMNASIO.md); aceptación de Pablo pendiente. Hospital: vínculo de paciente, intención persistente, solicitud y consulta de turnos mediante operaciones reales observadas en `feature/entrega-2`, con informe y recuperación en [COMUNICACION-HOSPITAL](COMUNICACION-HOSPITAL.md). No se modifica ninguno de los proyectos externos ni se reemplazan turnos por correos. Acuerdos y acceso externo siguen pendientes.

**Equipos externos:** gimnasio del grupo de Pablo (consume nuestra lista de empleados) y hospital del grupo de Octi (Car Cruds consume su API). Las referencias a “Clínica” en el diseño identifican al hospital de Octi. El contrato oficial del hospital está publicado; su acceso operativo y los acuerdos bilaterales siguen pendientes. El contrato propio del gimnasio requiere aceptación del consumidor.

| Campo | Valor |
| --- | --- |
| Versión | 0.3 |
| Fecha de revisión | 8 de noviembre de 2026 |
| Estado | Componentes propios integrados y verificados localmente; acuerdos operativos pendientes |
| Responsable funcional | Servicio Empleados |

Este documento distingue propuestas propias, contrato publicado y acuerdo bilateral. No sustituye el contrato externo. El [OpenAPI oficial de Turnos para socios 1.0.0](contracts/providers/octi-turnos-partner-v1.openapi.json), commit `c0b65f45867f884e0bdc8f7c23eb0f344f646bb2`, permite adaptar solicitud y consulta. La revisión observada antigua `5e4c210` se conserva como historia. No se recibieron URL compartida ni credencial; la tarifa inicial publicada de ARS 25.000 está pendiente de confirmación bilateral y no se fija en nuestro código.

## 1. Capacidades y consecuencia de negocio

| Integración | Alcance comunicado | Resultado administrativo |
| --- | --- | --- |
| Gimnasio | Consume la lista de empleados; administra membresías y proporciona afiliación, uso, costos individuales, gastos y totales | Informe de membresías y gastos por empleado y totales por período, con evidencia y datos faltantes |
| Clínica | Car Cruds solicita nuevos turnos para empleados registrados y activos y conserva su seguimiento posterior | Informe de personas con turno y sin turno, acompañado de la información que aún falta verificar |

El alquiler de autos conserva su flujo principal de clientes. El seguimiento de turnos y beneficios constituye el flujo de gestión de personal de la concesionaria. La integración debe demostrarse con consecuencias en estos informes y su uso administrativo. La validación docente del alcance y de la relevancia de este flujo se registra como pendiente; una llamada aislada no basta.

## 2. Fronteras y propiedad

- **Empleados** conserva empleados y evidencia administrativa de intercambios externos. La lista del gimnasio mantiene solo empleados activos.
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

El diseño inicial dejaba abierto el transporte. La implementación actual ofrece una llamada entrante **a Car Cruds**, descrita en [COMUNICACION-GIMNASIO](COMUNICACION-GIMNASIO.md), pendiente de aceptación bilateral; no se inventa un endpoint del gimnasio. El borrador [employees-openapi-draft.json](contracts/employees-openapi-draft.json) se conserva histórico. El contrato propio implementado es [employees-openapi.json](contracts/employees-openapi.json), versión 0.4.0: agrega la consulta de estado laboral `/status`; la lista y el intercambio de membresías mantienen su comportamiento.

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

1. Empleados identifica un empleado registrado y activo que necesita consulta a partir de una indicación administrativa o información recibida/consultada al hospital. El mecanismo externo se acordará.
2. Empleados conserva una referencia local de la intención y los datos mínimos autorizados antes de comunicarse con la clínica.
3. Su adaptador solicita el turno al hospital mediante las operaciones del contrato publicado, vinculando empleado, intención local y referencia de solicitud externa.
4. Si una respuesta válida confirma un turno, se conserva la referencia y el estado administrativo. Si falta confirmación, se mantiene la incertidumbre y se concilia según las operaciones disponibles.
5. El informe identifica personas con turno confirmado, personas sin turno confirmado con evidencia suficiente y personas con estado pendiente de verificar.

**Solicitar una consulta, obtener un turno y asistir son hechos distintos.** Car Cruds solicita el turno por la API del hospital. Se acordarán operaciones, confirmación y pasos adicionales del empleado. El hospital administra cambios de estado, cancelaciones y reprogramaciones; Car Cruds los refleja sin asumir control de su agenda.

### Recuperación ante fallas

| Situación | Tratamiento propuesto |
| --- | --- |
| Rechazo explícito del proveedor | Registrar el motivo administrativo permitido; no mostrar turno confirmado |
| Timeout después de enviar una solicitud | Resultado desconocido: conservar intención, contrato, clave y datos; recuperación explícita posterior |
| Reintento bajo Turnos para socios | Misma clave y cuerpo persistidos, empleado activo, una llamada por acción y espera durable; 429 respeta Retry-After |
| Solicitud antigua ya enviada sin procedencia idempotente | Nunca otro POST; conciliar por ID solo si el proveedor habilita pertenencia al consumidor |
| Credencial inválida o conflicto de idempotencia | Conservar desconocido; corregir credencial del mismo consumidor o revisar conflicto, sin cambiar claves para eludirlo |
| Consulta de estado fallida | Conservar la última evidencia y su fecha, indicar desactualización y permitir recuperación posterior |
| Turno cancelado o reprogramado | Actualizar conforme al contrato y conservar historia; no confundirlo con “nunca solicitó” |
| Proveedor caído | Limitar espera y concurrencia, indicar situación pendiente y conservar funciones de alquiler de vehículos |

Estos son estados administrativos internos propuestos. No representan nombres de campos, estados o códigos de la API de la clínica. Tampoco se definió un plazo externo como si fuera un SLA acordado.

## 5. Acuerdos pendientes

| ID | Acuerdo necesario | Interlocutor | Consecuencia para implementación |
| --- | --- | --- | --- |
| EXT-01 | Población de empleados compartida, activos/inactivos y cambios de altas/bajas | Gimnasio y responsable de personal | Define qué lista puede publicarse |
| EXT-02 | Identificadores y vinculación de personas; campos mínimos y permisos | Ambos equipos externos | Evita vincular información a la persona incorrecta |
| EXT-03 | Aceptar contrato propio 0.4.0, paginación, compatibilidad de la lista y consulta de estado laboral `/status` | Gimnasio | Contrato implementado y verificado; aceptación bilateral pendiente |
| EXT-04 | Aceptar mecanismo y contrato propios de devolución de membresías, uso y costos | Gimnasio | Recepción implementada; falta productor del gimnasio y ensayo conjunto |
| EXT-05 | Significado de uso/no uso, período, evidencia y correcciones | Gimnasio | Permite interpretar el informe sin inferencias |
| EXT-06 | Costos individuales, gastos por empleado, totales, moneda, período y reglas de imputación | Gimnasio y empresa | Informe propio implementado; faltan datos y semántica aceptados para acreditar gastos reales |
| EXT-07 | Publicado y desplegado: Turnos para socios 1.0.0 en `https://api-turnos.srv1779429.hstgr.cloud/v1`; contrato idéntico al integrado. Prueba real aprobada el 10/11/2026 ([evidencia](evidence/hospital-live-2026-11-10.json)) | Clínica y Car Cruds | Cerrado del lado técnico; resta la aceptación formal del hospital |
| EXT-08 | Alta, identidad, recuperación de patientId y habilitación de empleados como pacientes | Clínica y empresa | Falta procedimiento real; el vínculo local no crea ni habilita pacientes |
| EXT-09 | Pertenencia al consumidor de turnos creados antes del contrato de socios | Clínica | Sin asociación verificada la nueva API puede devolver 404; no recrearlos para recuperarlos |
| EXT-10 | Hospital: URL y clave privada en uso (la clave solo en `.env`). Gimnasio: consume la API publicada en Azure (`https://57-156-56-201.sslip.io`) y envió evidencia real de `employee-001` para 2026-11 (origen `gimnasio-pablo`, versión 1, referencia `15ae8f6d-ce74-4453-a694-13b23cdf7c64`), confirmada en PostgreSQL con una consulta de solo lectura | Ambos equipos externos | El panel de Gimnasio la muestra con su historial; no se reenvía ni se modifica |
| EXT-11 | Datos administrativos permitidos, acceso y conservación | Empresa y equipos externos | Limita los datos compartidos y registrados |
| EXT-12 | Confirmado por el hospital: 60 solicitudes por minuto por consumidor, `429` con `Retry-After` | Clínica | Ya respetado por el adaptador |
| EXT-13 | Responsables, pacientes habilitados, médicos y horarios controlados de prueba conjunta | Ambos equipos externos | Permite comprobar identidad, reserva, repetición y consulta sin afectar datos reales Pacientes de prueba informados: `pat-001` y `pat-003` habilitados |
| EXT-16 | Informado por el hospital: ARS 25.000 por reserva creada, sin reintegro al cancelar; repetir con la misma clave no se cobra | Clínica y empresa | El panel muestra la facturación mensual (`/v1/employees/clinic/usage`); aceptación económica a cargo del equipo |
| EXT-15 | Información del hospital sobre empleados que requieren consulta: consulta/notificación, identidad, vigencia y correcciones | Hospital de Octi | Define identificación y seguimiento de la necesidad de consulta |
| EXT-14 | Validación docente de la ampliación y su flujo de negocio | Cátedra y grupo | Confirma correspondencia con el enunciado |

El contrato clínico formal está publicado y la adaptación está integrada en `main` (PR #5). La prueba aislada contra el gateway original aprobó el 8 de noviembre; ver [revisión](REVISION-E2-2026-11-08.md). Los acuerdos operativos y la aceptación bilateral continúan **pendientes**, sin fechas ni aprobaciones inventadas. Los sistemas externos ya fueron identificados por el grupo; lo pendiente es completar su operación compartida.

## 6. Validación y fuentes

La suite propia real, checkshared y la prueba aislada del contrato hospitalario aprobaron el 8 de noviembre; [evidencia](REVISION-E2-2026-11-08.md). Las pruebas conjuntas y los ensayos ampliados siguientes siguen pendientes:

- Pruebas de contrato sobre la lista propia y el contrato real de la clínica; devolución del gimnasio cuando se acuerde el mecanismo.
- Pruebas con duplicados, respuestas tardías, personas sin vínculo y datos de períodos diferentes.
- Ensayo de caída externa que preserve el flujo de alquiler y mantenga estados desconocidos honestos.
- Revisión de informes con evidencias positivas, negativas y faltantes; sin tarifas ni resultados médicos sintéticos presentados como reales.

Ambas integraciones cuentan con componentes propios y pruebas; no equivalen a aceptación ni despliegue conjunto. La guía del [hospital](COMUNICACION-HOSPITAL.md) explica las operaciones consumidas y las limitaciones verificadas. Ver también [gimnasio](COMUNICACION-GIMNASIO.md), [entrega 2](ENTREGA-2.md) y [ADR-017](adr/ADR-017-contrato-empleados.md).

Material de referencia en `clases.zip`: teoría 02 (datos y consistencia), 04 (idempotencia y mensajería), 06 (identidad y límites), 07 (resultado desconocido y recuperación) y 08 (puertos/adaptadores); práctica 01 (interfaces/mocks) y 02 (Repository).

# ADR-014 — Límites de los servicios, revisión 2

> **Vigencia — 23/10/2026:** [ADR-018](ADR-018-empleados-sin-candidatos.md) sustituye el alcance de personal de este registro: se retira candidatos de la API y se conserva su almacenamiento histórico. Las demás fronteras y tecnologías permanecen. El cuerpo siguiente conserva decisiones y evidencia de sus fechas; las menciones a candidatos no definen el dominio activo.

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D1: límites de los servicios |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Límites implementados; acuerdos externos pendientes |
| Hito | Entrega 2 — 23 de octubre de 2026 |
| Sustituye | [ADR-001](ADR-001-limites-de-servicios.md) |

## 1. Contexto

La incorporación del Gimnasio como consumidor y de la Clínica como proveedor introduce un flujo administrativo de personal. El diseño original de tres servicios de alquiler no tiene un propietario adecuado para empleados, candidatos, evidencia de uso del gimnasio y referencias de turnos. Esos conceptos requieren un modelo distinto de la habilitación de un conductor.

## 2. Decisión

Se proponen cuatro microservicios de negocio con almacenamiento propio:

| Servicio | Responsabilidad | Datos propios |
| --- | --- | --- |
| Clientes | Perfiles y habilitación de conductores; historial de alquileres derivado | Cliente, licencia, estado y eventos procesados |
| Flota | Catálogo, características y tarifa vigente; búsqueda y caché | Vehículo, tarifa, versión y cambios pendientes |
| Alquileres | Agenda, precio acordado, reserva, retiro y devolución | Reserva, alquiler, bloqueos, idempotencia y outbox |
| Empleados | Administración de candidatos y empleados; integración con Gimnasio y Clínica; informes administrativos | Registro de personal, referencias externas, evidencia por período y origen, estado de integración |

Alquileres conserva la única autoridad sobre disponibilidad temporal. Empleados publica la lista para el Gimnasio y registra la información de uso/no uso mediante un mecanismo todavía por acordar. Empleados consume directamente la capacidad de la Clínica para revisiones o turnos, y prepara un informe con/sin turno confirmado cuando exista evidencia suficiente.

Cliente conductor y empleado/candidato son entidades distintas. No se deducen permisos de conducción, contratación ni aptitud médica a partir de registros de personal. La integración no requiere diagnósticos clínicos.

El gateway autentica la entrada externa; cada servicio autoriza el acceso a sus recursos. El balanceador interno distribuye exclusivamente tráfico hacia réplicas de Flota. Indexador, publicadores de outbox y balanceador son componentes técnicos, no servicios de negocio adicionales.

## 3. Alternativas consideradas

| Alternativa | Evaluación |
| --- | --- |
| Incorporar personal a Clientes | Confunde conductor con empleado y mezcla reglas, permisos y datos |
| Integración de Clínica dentro de Alquileres | Acopla turnos de personal con reservas de autos sin una consecuencia de negocio justificada |
| Servicio por proveedor externo | Fragmenta el flujo administrativo; los adaptadores dentro de Empleados aíslan la variación del contrato |
| Empleados autónomo | Preserva lenguaje, propiedad y políticas administrativas; agrega operación y pruebas de un cuarto servicio |

## 4. Consecuencias

Se modifica la frontera de integración externa propuesta en D1 inicial. Las llamadas a Clínica parten de Empleados. No se consultan bases ajenas ni se presupone que Gimnasio o Clínica participen del broker interno. Se incrementan componentes y documentación; el flujo de reserva mantiene su frontera transaccional.

## 5. Validación y acuerdos pendientes

Revisar diagramas y casos de uso; comprobar aislamiento de datos y permisos; probar reservas concurrentes e informes con evidencia ausente o contradictoria. Acordar identificación externa de personal, datos mínimos, períodos, paginación, autenticación, protocolos y forma de informar uso. Ningún contrato externo se considera aprobado.

## Avance de implementación — 14 de octubre de 2026

Los cuatro servicios operan con propietarios y credenciales de datos separados. Alquileres consulta Clientes y Flota; no incorpora dependencias de gimnasio ni hospital. Empleados distingue candidato y empleado y publica una proyección mínima propia. El recorrido comprueba que gimnasio no accede a candidatos ni a escrituras. Las integraciones e informes externos descritos en la decisión permanecen pendientes de contrato y no se presentan como operativos.

## 6. Referencias e historial

Material en `clases.zip`: `teoria/01-monolito-microservicios.md`, `teoria/02-datos-en-microservicios.md` y `teoria/08-estilos-arquitectura-backend.md`. [Arquitectura](../ARCHITECTURE.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Añade Empleados y ubica las integraciones administrativas |
| 0.3 | 14 de octubre de 2026 | Registra implementación propia, evidencia local y pendientes externos |


## Revisión de alcance — 11 de octubre de 2026

Se amplía la propuesta sin cambiar las fronteras ni las tecnologías: Gimnasio corresponde al grupo de Pablo y Clínica al hospital de Octi. El gimnasio consume empleados y administra membresías/costos; Empleados incorporará los datos económicos individuales y totales por período, con deduplicación, correcciones y separación por moneda. Uso y gasto se tratan como evidencias distintas.

El flujo del hospital comprende empleados registrados, además del flujo previo de candidatos: información de quién necesita consulta, solicitud de turno desde Empleados y seguimiento de modificaciones/cancelaciones/reprogramaciones administradas por el proveedor. Persistirá la relación entre empleado, necesidad, intención local, turno externo y evidencia de estado, sin interpretar un timeout como ausencia ni repetir comandos sin recuperación acordada.

El alcance de membresías/costos y turnos continúa sin implementar ni acordar con los equipos externos. No se modifica el borrador de lista para inventar contratos económicos o médicos. Consultar INTEGRACIONES, RF-19/HU-23 y RN-21/RN-22 para los acuerdos y verificaciones pendientes. La revisión anterior del registro se conserva en Git.

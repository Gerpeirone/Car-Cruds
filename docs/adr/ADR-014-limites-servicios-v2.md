# ADR-014 — Límites de los servicios, revisión 2

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D1: límites de los servicios |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Hito | Preparación documental de la Entrega 2 — 23 de octubre de 2026 |
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

## 6. Referencias e historial

Material en `clases.zip`: `teoria/01-monolito-microservicios.md`, `teoria/02-datos-en-microservicios.md` y `teoria/08-estilos-arquitectura-backend.md`. [Arquitectura](../ARCHITECTURE.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Añade Empleados y ubica las integraciones administrativas |

# ADR-015 — Persistencia por servicio, revisión 2

> **Vigencia — 23/10/2026:** [ADR-018](ADR-018-empleados-sin-candidatos.md) sustituye el alcance de personal de este registro: se retira candidatos de la API y se conserva su almacenamiento histórico. Las demás fronteras y tecnologías permanecen. El cuerpo siguiente conserva decisiones y evidencia de sus fechas; las menciones a candidatos no definen el dominio activo.

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D3: persistencia |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Almacenes integrados y persistencia propia verificada |
| Hito | Entrega 2 — 23 de octubre de 2026 |
| Sustituye | [ADR-003](ADR-003-persistencia.md) |

## 1. Contexto

El nuevo servicio Empleados necesita registros administrativos y evidencia por persona, período y origen. Alquileres continúa requiriendo integridad de agenda, estados e idempotencia. El almacenamiento debe responder a accesos y garantías, conservar dueños separados e integrar dos tipos operativos, relacional y documental.

## 2. Decisión

| Servicio | Almacén propuesto | Accesos y garantías |
| --- | --- | --- |
| Clientes | PostgreSQL propio | Perfil/licencia por identificador, integridad y actualización idempotente de historial |
| Flota | MongoDB | Ficha con atributos variables y tarifa, lectura por identificador, modificación versionada |
| Alquileres | PostgreSQL propio | Rangos por vehículo, estados, precio congelado, idempotencia y outbox en transacción local |
| Empleados | PostgreSQL propio | Personal y candidatos, referencias de turnos, evidencia administrativa por período y origen, consultas para informes |

Las bases relacionales podrán compartir un servidor de desarrollo, con bases y credenciales diferentes. No se comparten tablas ni se usan claves foráneas entre servicios. Las referencias externas se validarán según contratos; no habilitan acceso directo a la base del proveedor.

Empleados conservará únicamente información administrativa necesaria para el flujo y la trazabilidad de sus informes. Identidad externa, campos, reglas de conservación y mecanismos de actualización deben acordarse; no se presupone un expediente clínico. La evidencia de uso o confirmación debe indicar período, origen y vigencia suficientes para interpretar el informe; esas son necesidades del modelo propio, no nombres de campos aceptados por externos.

Solr y Redis son estructuras derivadas de Flota, reconstruibles desde MongoDB. No autorizan precios de reserva ni disponibilidad. Las dos réplicas de Flota acceden al mismo almacén operativo y a la caché compartida; el estado de usuario no reside en memoria de una réplica.

Alquileres guarda reserva y respuesta idempotente juntos; guarda devolución y outbox en otra transacción local. Flota confirma ficha y evento pendiente mediante outbox embebido acotado. Un dual write independiente no cumple esa garantía.

## 3. Alternativas consideradas

| Alternativa | Evaluación |
| --- | --- |
| Una base común para cuatro servicios | Acopla esquemas, permisos y cambios de despliegue |
| MongoDB también para Empleados | Viable, pero las consultas relacionales por personal/período/origen y la deduplicación favorecen la propuesta SQL |
| PostgreSQL para Flota | Viable; MongoDB se propone por la unidad documental y atributos variables, no por un supuesto de escala |
| Broker o índice como registro canónico | Confunde distribución/lectura derivada con propiedad durable del negocio |

## 4. Consecuencias y validación

Se operarán al menos dos motores con respaldos y credenciales propios. Volúmenes, índices, migraciones y retención no están medidos. [D4](ADR-004-consistencia-concurrencia.md) registra la propuesta concreta de exclusión sobre rangos de agenda y de índice único parcial para el retiro exclusivo. La implementación se probó contra PostgreSQL; sus resultados y límites se registran en la sección de avance.

Integrar almacenes reales, probar recuperación, concurrencia, duplicados y separación de permisos; verificar outbox y reconstrucción. Para Empleados, probar evidencia incompleta, correcciones y límites por período sin inferir no uso ni ausencia de turno a partir de errores.

## Avance de implementación — 14 de octubre de 2026

Clientes, Alquileres y Empleados utilizan PostgreSQL con bases/credenciales distintas; Flota MongoDB. Migraciones propias se aplican al iniciar. Flota eligió outbox embebido en el documento, acotado a 128 pendientes: un documento lleno rechaza nuevos cambios antes de confirmar. Alquileres guarda idempotencia con la reserva y outbox al finalizar el alquiler. La suite real y el reinicio de servicios verifican persistencia; datos económicos y de turnos permanecen pendientes de contrato. Solr/Redis son derivados, no fuentes del precio final o de la agenda. Ver [pruebas](../evidence/tests-summary.json) y [reinicio/reindexación](../evidence/runtime-persistence.json).

## 5. Referencias e historial

Material en `clases.zip`: `teoria/02-datos-en-microservicios.md`, `teoria/04-comunicacion-asincrona.md` y `practica/clase-2-mongodb-repository-pattern.md`. [D1](ADR-014-limites-servicios-v2.md), [D4](ADR-004-consistencia-concurrencia.md), [D5](ADR-016-comunicacion-v2.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Incorpora persistencia administrativa de Empleados y réplicas de Flota |
| 0.3 | 14 de octubre de 2026 | Registra implementación propia, evidencia local y pendientes externos |


## Revisión de alcance — 11 de octubre de 2026

Se amplía la propuesta sin cambiar las fronteras ni las tecnologías: Gimnasio corresponde al grupo de Pablo y Clínica al hospital de Octi. El gimnasio consume empleados y administra membresías/costos; Empleados incorporará los datos económicos individuales y totales por período, con deduplicación, correcciones y separación por moneda. Uso y gasto se tratan como evidencias distintas.

El flujo del hospital comprende empleados registrados, además del flujo previo de candidatos: información de quién necesita consulta, solicitud de turno desde Empleados y seguimiento de modificaciones/cancelaciones/reprogramaciones administradas por el proveedor. Persistirá la relación entre empleado, necesidad, intención local, turno externo y evidencia de estado, sin interpretar un timeout como ausencia ni repetir comandos sin recuperación acordada.

El alcance de membresías/costos y turnos continúa sin implementar ni acordar con los equipos externos. No se modifica el borrador de lista para inventar contratos económicos o médicos. Consultar INTEGRACIONES, RF-19/HU-23 y RN-21/RN-22 para los acuerdos y verificaciones pendientes. La revisión anterior del registro se conserva en Git.

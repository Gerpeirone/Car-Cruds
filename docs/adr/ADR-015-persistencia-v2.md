# ADR-015 — Persistencia por servicio, revisión 2

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D3: persistencia |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Hito | Preparación documental de la Entrega 2 — 23 de octubre de 2026 |
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

Alquileres guardará cambio de negocio, respuesta idempotente y outbox en una misma transacción. Flota requiere atomicidad de ficha y evento pendiente: antes de implementar se decidirá entre outbox embebido acotado o transacción entre colecciones con topología compatible. Un dual write independiente no cumple esa garantía.

## 3. Alternativas consideradas

| Alternativa | Evaluación |
| --- | --- |
| Una base común para cuatro servicios | Acopla esquemas, permisos y cambios de despliegue |
| MongoDB también para Empleados | Viable, pero las consultas relacionales por personal/período/origen y la deduplicación favorecen la propuesta SQL |
| PostgreSQL para Flota | Viable; MongoDB se propone por la unidad documental y atributos variables, no por un supuesto de escala |
| Broker o índice como registro canónico | Confunde distribución/lectura derivada con propiedad durable del negocio |

## 4. Consecuencias y validación

Se operarán al menos dos motores con respaldos y credenciales propios. Volúmenes, índices, migraciones y retención no están medidos. D4 determinará la garantía concreta de exclusión y retiro; la selección de PostgreSQL no demuestra por sí sola ausencia de carreras.

Integrar almacenes reales, probar recuperación, concurrencia, duplicados y separación de permisos; verificar outbox y reconstrucción. Para Empleados, probar evidencia incompleta, correcciones y límites por período sin inferir no uso ni ausencia de turno a partir de errores.

## 5. Referencias e historial

Material en `clases.zip`: `teoria/02-datos-en-microservicios.md`, `teoria/04-comunicacion-asincrona.md` y `practica/clase-2-mongodb-repository-pattern.md`. [D1](ADR-014-limites-servicios-v2.md), [D5](ADR-016-comunicacion-v2.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Incorpora persistencia administrativa de Empleados y réplicas de Flota |

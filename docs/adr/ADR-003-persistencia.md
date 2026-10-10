# ADR-003 — Persistencia por servicio

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D3: persistencia |
| Versión | 0.1 — inicial |
| Fecha | 7 de octubre de 2026 |
| Estado | Reemplazado |
| Hito | Entrega 1 — 9 de octubre de 2026 |
| Reemplazado por | [ADR-015 — D3, revisión 2](ADR-015-persistencia-v2.md), 10 de octubre de 2026 |

Este registro conserva la selección histórica de la entrega 1. La propuesta vigente incorpora el almacenamiento propio de Empleados en ADR-015.

## 1. Contexto

El sistema debe integrar almacenamiento relacional y no relacional. Alquileres necesita proteger solapamientos, estados y reintentos. Clientes necesita integridad de perfiles y validaciones por identificador. Flota mantiene fichas con atributos variables, consultadas por características; la búsqueda utiliza una proyección reconstruible.

La selección de persistencia debe responder a esos accesos y conservar la propiedad de los datos definida en D1.

## 2. Decisión

Se proponen los siguientes almacenes operativos:

| Servicio | Almacenamiento | Accesos y justificación |
| --- | --- | --- |
| Alquileres | PostgreSQL | Transacciones de reserva, consulta de rangos por auto, estados, respuestas de idempotencia y outbox |
| Clientes | PostgreSQL propio | Consulta por identificador, validación de estado y licencia, integridad del perfil e historial |
| Flota | MongoDB | Documento de auto con características y tarifa, lectura por identificador y actualización versionada |

El motor de búsqueda propuesto, Solr, conservará documentos derivados de Flota. Redis se propone como caché de fichas frecuentes. Ambos cumplen funciones de lectura; los almacenes operativos conservan los datos de negocio y permiten reconstruir las estructuras derivadas.

Las bases PostgreSQL podrán compartir servidor en desarrollo, con bases y credenciales independientes. Los servicios se comunicarán mediante contratos y mantendrán separados sus accesos a datos.

Flota deberá conservar la modificación de ficha y su evento pendiente de forma atómica. Se evaluarán dos mecanismos: outbox embebido en el documento o transacción entre colecciones mediante un replica set. La elección precederá a la implementación de la indexación.

## 3. Alternativas consideradas

| Alternativa | Evaluación |
| --- | --- |
| Base compartida entre servicios | Facilita consultas, pero acopla esquemas, accesos y propiedad de los datos |
| PostgreSQL para todos y Redis solo como caché | Ofrece una separación menos clara del almacenamiento no relacional de negocio; se propone MongoDB para las fichas variables de Flota |
| MongoDB también para Alquileres | Es viable, pero la exclusión de rangos y la concurrencia requieren mayor cuidado que con la alternativa relacional propuesta |
| Índice como almacenamiento principal | Aumenta la dificultad de reconstrucción, consistencia y recuperación |

## 4. Justificación y consecuencias

PostgreSQL concentra en Alquileres las operaciones que requieren garantías transaccionales. MongoDB permite representar las características variables de los vehículos mediante documentos. Esta separación incorpora dos motores operativos y exige configuración, respaldo y mantenimiento para ambos.

La integridad entre servicios se controla mediante contratos, validaciones y eventos, sin claves foráneas entre bases. El volumen inicial corresponde a una sucursal y una flota pequeña; la capacidad deberá comprobarse mediante pruebas de carga.

La solución concreta de exclusión de rangos, los estados que ocupan agenda y el retiro de un auto con devolución atrasada se definirán en D4. La confirmación deberá proteger consulta e inserción mediante garantías transaccionales.

## 5. Validación prevista

- Integrar persistencia real en el flujo de la entrega 2.
- Ejecutar pruebas de integración contra almacenes reales y pruebas concurrentes de reserva.
- Verificar recuperación tras reinicio y separación de credenciales y accesos.
- Comprobar persistencia y publicación de outbox, y reconstrucción del índice desde Flota.

El mock de la entrega 1 conserva sus datos en memoria. La elección de estos almacenes corresponde al diseño objetivo y se validará al implementarlos.

## 6. Aspectos por resolver

- Versiones de los motores y compatibilidad con las tecnologías seleccionadas.
- Restricciones y bloqueos de Alquileres en D4.
- Mecanismo atómico de outbox en MongoDB.
- Procedimientos de respaldo, recuperación y medición de capacidad.

## 7. Historial

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.1 | 7 de octubre de 2026 | Selección inicial de persistencia y estructuras derivadas |

Una decisión sustitutiva deberá identificar este registro como reemplazado y conservar su historia.

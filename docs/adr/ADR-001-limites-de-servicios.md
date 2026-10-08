# ADR-001 — Límites de los servicios

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D1: límites de los servicios |
| Versión | 0.1 |
| Fecha | 7 de octubre de 2026 |
| Estado | Propuesto |
| Hito | Entrega 1 — 9 de octubre de 2026 |

## 1. Contexto

Car Cruds administra alquileres de autos por período y con tarifa. La confirmación de una reserva exige validar conductor, precio y disponibilidad. El retiro y la devolución afectan la ocupación física del auto. El sistema debe contar con al menos tres microservicios y un gateway, con responsabilidades y datos claramente delimitados.

La separación debe preservar una frontera transaccional para la exclusión de períodos y permitir que las lecturas de catálogo evolucionen independientemente de las reservas.

## 2. Decisión

Se propone separar tres servicios de negocio:

| Servicio | Responsabilidad | Propiedad de los datos |
| --- | --- | --- |
| Clientes | Perfil y habilitación del conductor; historial proyectado | Cliente, licencia y estado, historial derivado y eventos procesados |
| Flota | Características, catálogo y tarifa diaria; búsqueda y caché de fichas | Vehículos, tarifas, versiones y eventos de cambio |
| Alquileres | Reserva, precio acordado, agenda exclusiva, retiro y devolución | Reservas, alquileres, bloqueos de mantenimiento, idempotencia y outbox |

Alquileres es el único propietario de la disponibilidad temporal. Las reservas y los bloqueos que compiten por un auto se resuelven dentro de su misma base. Flota administra el catálogo y la tarifa vigente; las asignaciones temporales pertenecen a Alquileres.

El gateway concentra la entrada pública de la interfaz web y de la capacidad compartida. El proveedor externo se consume directamente desde el microservicio responsable de su funcionalidad. Cada servicio accede exclusivamente a sus almacenes; las interacciones entre servicios utilizan contratos.

## 3. Alternativas consideradas

| Alternativa | Evaluación |
| --- | --- |
| Servicio único | Reduce complejidad operativa, pero incumple la arquitectura de microservicios requerida |
| Separar Reservas de Alquileres o Agenda | Distribuye la exclusión del auto y exige coordinación de fallas parciales en la operación crítica, sin un beneficio suficiente para el alcance inicial |
| Servicio adicional de Pagos | Introduce integración y estados financieros ajenos al alcance inicial de reserva y alquiler |

## 4. Justificación y consecuencias

Concentrar la agenda en Alquileres mantiene las reglas de ocupación dentro de una frontera transaccional. La habilitación del conductor y la tarifa requieren consultas síncronas, cuyo comportamiento frente a lentitud o indisponibilidad se define en D5.

Clientes puede actualizar el historial mediante eventos sin administrar reservas. Flota puede optimizar búsqueda y caché con consistencia eventual; sus proyecciones facilitan la consulta mientras Alquileres conserva la autoridad para confirmar.

Los cambios administrativos de Flota preservan las reservas existentes. Las acciones que deban bloquear entregas requieren coordinación explícita con Alquileres.

## 5. Validación prevista

- Verificar correspondencia entre diagramas, responsabilidades y propietarios de datos.
- Probar dos confirmaciones concurrentes para el mismo auto y período; una sola debe ocupar la agenda.
- Comprobar durante la implementación que los servicios no consulten bases ajenas.

La estructura inicial documenta estos límites. Las garantías transaccionales se validarán sobre los servicios y almacenes reales.

## 6. Aspectos por resolver

- Formalización de los patrones internos en D2.
- Coordinación de bajas de vehículos y cambios de habilitación con reservas vigentes.
- Ubicación del adaptador externo según la capacidad asignada por la cátedra.

## 7. Historial

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.1 | 7 de octubre de 2026 | Definición preliminar de límites y propiedad de datos |

Una decisión sustitutiva deberá identificar este registro como reemplazado y conservar su historia.

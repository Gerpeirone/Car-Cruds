# Servicio Alquileres

| Aspecto | Diseño inicial |
| --- | --- |
| Estado | Estructura de directorios y responsabilidades; implementación pendiente |
| Responsabilidad | Reservas, precio acordado, agenda, bloqueos, retiro y devolución |
| Persistencia | PostgreSQL propio |
| Arquitectura interna | Hexagonal: dominio, aplicación y adaptadores |
| Tecnologías de aplicación | Lenguaje y framework pendientes de selección |

Alquileres es la autoridad de disponibilidad temporal y exclusión por vehículo y período. Preserva la tarifa acordada al reservar y administra las transiciones de reserva y alquiler. El [mock local](../../mocks/rentals/README.md) demuestra el contrato de la primera entrega; la implementación del servicio conserva el estado de diseño.

## Organización prevista

| Directorio | Responsabilidad |
| --- | --- |
| `src/domain/` | Períodos, dinero, estados y reglas de negocio |
| `src/application/` | Casos de uso y puertos de dependencias |
| `src/adapters/` | HTTP, PostgreSQL, Clientes, Flota, proveedor y mensajería |

## Interfaces y dependencias

| Relación | Propósito |
| --- | --- |
| Gateway → Alquileres, HTTP | Consultar disponibilidad y gestionar reservas y alquileres |
| Alquileres → Clientes, HTTP | Validar habilitación del conductor |
| Alquileres → Flota, HTTP | Obtener ficha y tarifa autoritativa |
| Alquileres → PostgreSQL | Mantener agenda, estados, importes e idempotencia |
| Alquileres → mensajería | Publicar eventos mediante outbox |
| Alquileres → proveedor externo | Incorporar la capacidad asignada al flujo correspondiente |

La reserva, la respuesta idempotente y el evento de outbox se almacenarán en una transacción local. La exclusión de rangos debe conservarse con múltiples instancias; un mutex de proceso es suficiente únicamente para la demostración del mock.

El retiro comprueba que el vehículo no tenga otro alquiler activo, incluso si terminó su período previsto. La devolución es una operación explícita; el calendario no finaliza automáticamente un alquiler. D2 y D4 formalizarán el patrón interno y las garantías de consistencia en los hitos correspondientes.

## Referencias

- [SPEC.md — alcance, estados y reglas](../../SPEC.md).
- [Arquitectura del sistema](../../docs/ARCHITECTURE.md).
- [ADR-001 — límites de servicios](../../docs/adr/ADR-001-limites-de-servicios.md).
- [ADR-003 — persistencia](../../docs/adr/ADR-003-persistencia.md).
- [ADR-005 — comunicación](../../docs/adr/ADR-005-comunicacion.md).
- [ADR-008 — contrato propio](../../docs/adr/ADR-008-contrato-propio.md).
- [Contrato de disponibilidad y reservas](../../docs/contracts/README.md).

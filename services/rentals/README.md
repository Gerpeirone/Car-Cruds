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
| `src/adapters/` | HTTP, PostgreSQL, Clientes, Flota mediante el balanceador y mensajería |

## Interfaces y dependencias

| Relación | Propósito |
| --- | --- |
| Gateway → Alquileres, HTTP | Consultar disponibilidad y gestionar reservas y alquileres |
| Alquileres → Clientes, HTTP | Validar habilitación del conductor |
| Alquileres → Load Balancer → Flota, HTTP | Obtener ficha y tarifa autoritativa |
| Alquileres → PostgreSQL | Mantener agenda, estados, importes e idempotencia |
| Alquileres → mensajería | Publicar eventos mediante outbox |

La reserva, la respuesta idempotente y el evento de outbox se almacenarán en una transacción local. La exclusión de rangos debe conservarse con múltiples instancias; un mutex de proceso es suficiente únicamente para la demostración del mock.

El retiro comprueba que el vehículo no tenga otro alquiler activo, incluso si terminó su período previsto. La devolución es una operación explícita; el calendario no finaliza automáticamente un alquiler. D2 propone el patrón interno y D4 debe cerrar las garantías de consistencia reales.

Gimnasio y clínica pertenecen al flujo de Empleados. Alquileres conserva sus dependencias de Clientes y Flota; su confirmación no depende del estado de esas integraciones externas.

## Referencias

- [SPEC.md — alcance, estados y reglas](../../SPEC.md).
- [Arquitectura del sistema](../../docs/ARCHITECTURE.md).
- [D1 vigente — límites](../../docs/adr/ADR-014-limites-servicios-v2.md).
- [D2 — arquitectura interna](../../docs/adr/ADR-002-arquitectura-interna.md).
- [D3 vigente — persistencia](../../docs/adr/ADR-015-persistencia-v2.md).
- [D5 vigente — comunicación](../../docs/adr/ADR-016-comunicacion-v2.md).
- [Contrato histórico de reservas](../../docs/contracts/RESERVAS.md).

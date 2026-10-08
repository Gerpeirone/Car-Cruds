# Frontend web

| Aspecto | Diseño inicial |
| --- | --- |
| Estado | Estructura inicial; interfaz pendiente de implementación |
| Usuarios | Cliente y operador de la concesionaria |
| Integración | API Gateway mediante HTTP |
| Tecnología | Framework pendiente de selección |

La interfaz web presenta los flujos de búsqueda, cotización y reserva, además de la operación de alquileres y administración según el rol.

## Flujos previstos

| Usuario | Funcionalidades |
| --- | --- |
| Cliente | Ingreso, catálogo y búsqueda, disponibilidad y precio, reserva, consulta y cancelación |
| Operador | Administración de flota y clientes, bloqueos de agenda, retiro y devolución |

`src/` aloja la estructura prevista del código. La interfaz consume el gateway; el acceso a persistencia, mensajería y proveedores corresponde a los servicios de negocio.

## Referencias

- [SPEC.md — actores, alcance y reglas](../SPEC.md).
- [Arquitectura del sistema](../docs/ARCHITECTURE.md).
- [ADR-001 — límites de servicios](../docs/adr/ADR-001-limites-de-servicios.md).
- [ADR-008 — contrato propio](../docs/adr/ADR-008-contrato-propio.md).
- [Contrato de disponibilidad y reservas](../docs/contracts/README.md).

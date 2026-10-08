# API Gateway

| Aspecto | Diseño inicial |
| --- | --- |
| Estado | Estructura de directorios y responsabilidades; implementación pendiente |
| Responsabilidad | Entrada pública, autenticación, direccionamiento y control de tráfico |
| Destinos | Clientes, Flota y Alquileres |
| Tecnología | Pendiente de selección |

El gateway concentra el acceso del frontend y las aplicaciones consumidoras. Propaga identidad y correlación hacia los servicios. Las reglas de reservas y el acceso a datos de negocio corresponden al servicio propietario.

## Organización prevista

| Directorio | Responsabilidad |
| --- | --- |
| `src/routes/` | Definición de rutas y destinos |
| `src/middleware/` | Autenticación, permisos de entrada, correlación y límites de tráfico |

El proveedor externo se consume desde el microservicio responsable del flujo. La demostración del contrato utiliza un [mock HTTP local](../mocks/rentals/README.md) accesible directamente; la integración a través del gateway corresponde a la implementación del sistema objetivo.

## Referencias

- [SPEC.md — actores y permisos](../SPEC.md).
- [Arquitectura del sistema](../docs/ARCHITECTURE.md).
- [ADR-001 — límites de servicios](../docs/adr/ADR-001-limites-de-servicios.md).
- [ADR-005 — comunicación](../docs/adr/ADR-005-comunicacion.md).
- [ADR-008 — contrato propio](../docs/adr/ADR-008-contrato-propio.md).

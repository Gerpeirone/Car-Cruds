# Servicio Clientes

| Aspecto | Diseño inicial |
| --- | --- |
| Estado | Estructura de directorios y responsabilidades; implementación pendiente |
| Responsabilidad | Perfil, licencia registrada, habilitación del conductor e historial proyectado |
| Persistencia | PostgreSQL propio |
| Arquitectura interna | Capas: presentación, negocio y datos |
| Tecnologías de aplicación | Lenguaje y framework pendientes de selección |

Clientes administra el perfil y la habilitación del conductor. El historial se obtiene de alquileres completados; la reserva y la disponibilidad temporal pertenecen a Alquileres.

Los registros de candidatos, empleados e integraciones de personal pertenecen a Empleados. Una persona que también sea cliente requiere una vinculación explícita entre identidades; no se comparte la base ni se fusionan sus funciones.

## Organización prevista

| Directorio | Responsabilidad |
| --- | --- |
| `src/presentation/` | Interfaz HTTP, validación de formato y respuestas |
| `src/business/` | Casos de uso y reglas de habilitación |
| `src/data/` | Persistencia y registro idempotente de eventos consumidos |

## Interfaces y dependencias

| Relación | Propósito |
| --- | --- |
| Alquileres → Clientes, HTTP | Consultar la habilitación del conductor |
| Mensajería → Clientes, `RentalCompleted.v1` | Actualizar el historial de alquileres |
| Clientes → PostgreSQL | Consultar y mantener los datos propios |

La autenticación individual y los permisos se integrarán con el gateway. La separación de datos impide el acceso directo a las bases de Flota o Alquileres.

## Referencias

- [SPEC.md — alcance y reglas](../../SPEC.md).
- [Arquitectura del sistema](../../docs/ARCHITECTURE.md).
- [D1 vigente — límites](../../docs/adr/ADR-014-limites-servicios-v2.md).
- [D2 — arquitectura interna](../../docs/adr/ADR-002-arquitectura-interna.md).
- [D3 vigente — persistencia](../../docs/adr/ADR-015-persistencia-v2.md).
- [D5 vigente — comunicación](../../docs/adr/ADR-016-comunicacion-v2.md).

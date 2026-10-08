# Servicio Flota

| Aspecto | Diseño inicial |
| --- | --- |
| Estado | Estructura de directorios y responsabilidades; implementación pendiente |
| Responsabilidad | Catálogo, características y tarifa diaria vigente |
| Persistencia | MongoDB propio |
| Arquitectura interna | Capas, con indexación y caché de lectura |
| Tecnologías de aplicación | Lenguaje y framework pendientes de selección |

Flota administra las fichas de vehículos y sus tarifas. La búsqueda es una proyección reconstruible y la caché acelera consultas de fichas. La disponibilidad temporal y los bloqueos que compiten con reservas se resuelven en Alquileres.

## Organización prevista

| Directorio | Responsabilidad |
| --- | --- |
| `src/presentation/` | Consultas y comandos HTTP; búsqueda paginada |
| `src/business/` | Casos de uso y validación de vehículos y tarifas |
| `src/data/` | Persistencia documental, outbox, índice y caché |
| `src/workers/` | Indexación de cambios versionados |

## Interfaces y dependencias

| Relación | Propósito |
| --- | --- |
| Alquileres → Flota, HTTP | Consultar ficha y tarifa autoritativa para confirmar |
| Flota → MongoDB | Mantener catálogo y tarifa vigente |
| Flota → mensajería, `VehicleChanged.v1` | Publicar cambios para el indexador |
| Indexador de Flota → motor de búsqueda | Actualizar la proyección por identificador y versión |
| Flota → caché | Reutilizar fichas consultadas frecuentemente |

Solr, Redis y RabbitMQ son las tecnologías propuestas para búsqueda, caché y mensajería. Las decisiones y mediciones de búsqueda y caché se formalizarán en D6 y D7 durante la entrega 2; la mensajería se describe en ADR-005. Una modificación administrativa del catálogo conserva las reservas existentes; los cambios que bloqueen entregas requieren coordinación explícita con Alquileres.

## Referencias

- [SPEC.md — alcance y reglas](../../SPEC.md).
- [Arquitectura del sistema](../../docs/ARCHITECTURE.md).
- [ADR-001 — límites de servicios](../../docs/adr/ADR-001-limites-de-servicios.md).
- [ADR-003 — persistencia](../../docs/adr/ADR-003-persistencia.md).
- [ADR-005 — comunicación](../../docs/adr/ADR-005-comunicacion.md).

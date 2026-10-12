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
| Gateway / Alquileres → Load Balancer → Flota, HTTP | Consultar catálogo o ficha y tarifa autoritativa para confirmar |
| Flota → MongoDB | Mantener catálogo y tarifa vigente |
| Flota → mensajería, `VehicleChanged.v1` | Publicar cambios para el indexador |
| Indexador de Flota → motor de búsqueda | Actualizar la proyección por identificador y versión |
| Flota → caché | Reutilizar fichas consultadas frecuentemente |

Solr, Redis y RabbitMQ son las tecnologías propuestas para búsqueda, caché y mensajería. D6 y D7 registran el diseño; sus mediciones e implementación siguen pendientes. D5 vigente describe la mensajería. Una modificación administrativa del catálogo conserva las reservas existentes; los cambios que bloqueen entregas requieren coordinación explícita con Alquileres.

Se prevén dos réplicas stateless detrás del Load Balancer, con los mismos almacenes propios de Flota y sin sesión o caché obligatoria en la memoria de una réplica. Este directorio no implementa todavía ese despliegue ni prueba distribución o retirada de instancias.

## Referencias

- [SPEC.md — alcance y reglas](../../SPEC.md).
- [Arquitectura del sistema](../../docs/ARCHITECTURE.md).
- [D1 vigente — límites](../../docs/adr/ADR-014-limites-servicios-v2.md).
- [D2 — arquitectura interna](../../docs/adr/ADR-002-arquitectura-interna.md).
- [D3 vigente — persistencia](../../docs/adr/ADR-015-persistencia-v2.md).
- [D5 vigente — comunicación](../../docs/adr/ADR-016-comunicacion-v2.md).
- [D6 — búsqueda](../../docs/adr/ADR-006-busqueda.md) y [D7 — caché](../../docs/adr/ADR-007-cache.md).
- [D12 — balanceo](../../docs/adr/ADR-012-balanceo-carga.md).

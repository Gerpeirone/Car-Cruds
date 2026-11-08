# Servicio Clientes

| Aspecto | Implementación |
| --- | --- |
| Fecha | 13 de octubre de 2026 |
| Ejecutable | `go run ./cmd/customers` |
| Persistencia | PostgreSQL propio; migración embebida en startup |
| Arquitectura | Capas: presentación HTTP → negocio → acceso a datos |
| Datos | Perfil, habilitación, licencia y proyección de alquileres completados |

`presentation/` interpreta HTTP y permisos; `business/` valida perfiles y ejecuta casos de uso; `data/` implementa PostgreSQL y la recepción de eventos. Los registros de empleados y candidatos pertenecen a Empleados y no comparten este almacenamiento.

## Operaciones

Referencia de entradas, esquemas, ejemplos y errores: [OpenAPI de Clientes](../../docs/contracts/customers-openapi.json). [Guía y visor Swagger](../../docs/API.md).

| Método y ruta | Propósito | Permisos |
| --- | --- | --- |
| `GET /v1/customers/{id}` | Perfil para validar conductor | admin/service; client para su CustomerID |
| `GET /v1/customers` | Lista administrativa, hasta 200 perfiles | admin, service |
| `POST /v1/customers` | Registrar cliente | admin |

El JSON de creación requiere `id`, `name`, `active` y `licenseValidUntil`, con fecha YYYY-MM-DD. La respuesta utiliza esos mismos campos. Un identificador repetido produce 409; la lectura de un perfil inexistente produce 404. El gimnasio no tiene acceso a este servicio.

El seed se habilita únicamente con SEED_DEMO=true: cli-001 activo y cli-blocked inactivo, ambos con licencia registrada hasta 2030-12-31. Ejecutarlo otra vez conserva modificaciones existentes.

## Historial y mensajes fallidos

Con AMQP_URL configurada, el consumidor recibe RentalCompleted.v1 del intercambio carcruds.events. Registra el identificador en customer_inbox y el historial en una misma transacción; ACK ocurre después de persistir. Un duplicado no genera otra entrada.

Los errores de persistencia tienen tres intentos acotados. Un mensaje inválido o cuyo procesamiento falla pasa a customers.rental-history.failed mediante el intercambio carcruds.failed, conservando evidencia para reproceso. El broker indisponible no impide las consultas de habilitación desde PostgreSQL.

## Configuración y verificación

DATABASE_URL identifica la base propia. AUTH_ADMIN_TOKEN y AUTH_SERVICE_TOKEN son obligatorias; AUTH_CLIENT_TOKEN habilita el cliente de demostración. Los secretos se suministran fuera del repositorio. HTTP_ADDR o PORT configuran la escucha, y OTEL_EXPORTER_OTLP_ENDPOINT activa las trazas. Las rutas internas de disponibilidad y métricas se protegen para admin/service.

`go test -race ./services/customers/...` comprueba validación y permisos. TEST_CUSTOMERS_DATABASE_URL, o TEST_DATABASE_URL, habilita tests contra PostgreSQL real en un esquema aislado: migración, seed explícito, perfiles persistidos e inbox/historial idempotente. Si no existe la variable, la integración se identifica como omitida.

Referencias: [SPEC](../../SPEC.md), [D2](../../docs/adr/ADR-002-arquitectura-interna.md), [D3 vigente](../../docs/adr/ADR-015-persistencia-v2.md), [D5 vigente](../../docs/adr/ADR-016-comunicacion-v2.md).

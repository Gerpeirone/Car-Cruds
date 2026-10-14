# Servicio Alquileres

| Aspecto | Implementación |
| --- | --- |
| Fecha | 13 de octubre de 2026 |
| Ejecutable | `go run ./cmd/rentals` |
| Persistencia | PostgreSQL propio; migración embebida en startup |
| Arquitectura | Hexagonal: dominio, aplicación, puertos y adaptadores |
| Garantías | Exclusión de rangos en agenda, idempotencia durable y un alquiler ACTIVE por vehículo |

El dominio en `domain/` no importa HTTP, PostgreSQL ni plataforma. `application/` define los casos de uso y sus puertos; `adapters/` integra HTTP, Clientes, Flota, PostgreSQL y RabbitMQ. El mock histórico de `mocks/rentals/` sigue siendo independiente.

## Operaciones

| Método y ruta | Propósito | Permisos |
| --- | --- | --- |
| `GET /v1/availability` | Cotización y agenda por vehicleId, startsOn, endsOn | admin, service, client |
| `POST /v1/reservations` | Crear con Idempotency-Key y precio esperado | admin, service; client para su CustomerID |
| `GET /v1/reservations/{id}` | Consultar el estado actual | admin/service; client dentro de su aplicación y perfil |
| `DELETE /v1/reservations/{id}` | Cancelar RESERVED; repetir CANCELLED | Igual que consulta |
| `POST /v1/reservations/{id}/pickup` | Revalidar conductor y registrar retiro vigente | admin, service |
| `POST /v1/reservations/{id}/return` | Registrar devolución; repetir COMPLETED | admin, service |
| `POST /v1/maintenance` | Bloquear vehicleId, startsOn y endsOn contra la agenda | admin, service |

Los períodos son [inicio, fin), de 1 a 30 días calendario. El inicio debe ser igual o posterior al día actual en America/Buenos_Aires. Se cobra en centavos ARS; la licencia debe cubrir hasta el último día ocupado. La consulta de disponibilidad no retiene un vehículo.

Clientes responde `GET /v1/customers/{id}`; la tarifa se obtiene por `GET /v1/vehicles/{id}/quote`, mediante el balanceador y sin caché de Flota. Cada llamada tiene un límite de 1 segundo, dentro del presupuesto total de 5 segundos. Una dependencia inválida o indisponible produce 503 sin confirmar ni consumir la clave.

## Consistencia y mensajería

La creación serializa una misma aplicación/operación/clave en PostgreSQL, recupera su respuesta antes de revalidar fechas o dependencias y compara la huella de la solicitud. Reserva, agenda y respuesta se confirman en una transacción. Un rechazo de negocio queda asociado a esa clave; para corregirlo se utiliza otra. Un fallo técnico revierte la transacción y permite repetir.

La cancelación conserva el registro y libera la agenda una sola vez. Una creación repetida devuelve su respuesta original, aunque luego se haya cancelado; GET muestra el estado actual. El retiro exclusivo se protege por índice único parcial incluso si otro alquiler ACTIVE venció su fecha prevista.

La devolución registra `RentalCompleted.v1` y su outbox en la misma transacción. El relay usa confirmaciones del publicador y una cola durable del intercambio `carcruds.events`; Clientes aplica el historial con inbox idempotente. Se publican únicamente eventos con consumidor definido. Las integraciones de gimnasio y clínica corresponden a Empleados.

## Configuración

Requiere DATABASE_URL, credenciales AUTH_ADMIN_TOKEN/AUTH_SERVICE_TOKEN y SERVICE_TOKEN para las llamadas internas; SERVICE_TOKEN puede tomar AUTH_SERVICE_TOKEN. Las credenciales se suministran fuera de Git.

| Variable | Propósito |
| --- | --- |
| DATABASE_URL | Base propia de Alquileres |
| CUSTOMERS_URL | URL interna de Clientes |
| FLEET_URL | URL del balanceador de Flota |
| AMQP_URL | Activa el relay de eventos cuando está configurada |
| HTTP_ADDR o PORT | Dirección de escucha |
| OTEL_EXPORTER_OTLP_ENDPOINT | Exportación de trazas |

La disponibilidad técnica y las métricas usan `/internal/ready` y `/internal/metrics`, protegidas para admin/service.

## Verificación

`go test -race ./services/rentals/...` ejecuta reglas de calendario, dinero, permisos y comportamiento de dependencias. TEST_RENTALS_DATABASE_URL, o TEST_DATABASE_URL, habilita integración contra PostgreSQL real en esquemas temporales aislados. Si falta la variable, esos tests se identifican como omitidos; no acreditan integración.

Las pruebas reales cubren migración idempotente, dos instancias lógicas concurrentes con una sola reserva ganadora, reintento durable tras cancelación, caída de dependencias, ownership, mantenimiento y devolución transaccional. El ensayo completo del relay y consumidor requiere RabbitMQ y ambos servicios operativos.

Referencias: [SPEC](../../SPEC.md), [D2](../../docs/adr/ADR-002-arquitectura-interna.md), [D4](../../docs/adr/ADR-004-consistencia-concurrencia.md), [D5 vigente](../../docs/adr/ADR-016-comunicacion-v2.md).

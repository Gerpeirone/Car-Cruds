# Contrato de disponibilidad y reservas

| Campo | Valor |
| --- | --- |
| Versión documental | 0.1 |
| Fecha | 7 de octubre de 2026 |
| Estado | Diseño preliminar; contrato formal y mock local disponibles |
| Hito | Entrega 1 — 9 de octubre de 2026 |
| Versión de API | 1.0.0, rutas `/v1` |
| Formato | OpenAPI 3.0.3, JSON |

[openapi-v1.json](openapi-v1.json) especifica la capacidad de consulta de disponibilidad, cotización y gestión de reservas. El contrato se basa en la [especificación formal OpenAPI 3.0.3](https://spec.openapis.org/oas/v3.0.3) y en las reglas de [SPEC.md](../../SPEC.md).

La autoridad de la disponibilidad temporal corresponde a Alquileres, según la [arquitectura](../ARCHITECTURE.md) y [ADR-001 — límites de servicios](../adr/ADR-001-limites-de-servicios.md). [ADR-008 — contrato propio](../adr/ADR-008-contrato-propio.md) documenta la capacidad publicada y su evolución; [ADR-005 — comunicación](../adr/ADR-005-comunicacion.md) define el tratamiento de reintentos e idempotencia.

## Operaciones

Todas las operaciones requieren autenticación Bearer. En la demostración local se utiliza `Authorization: Bearer demo-token`.

| Método y ruta | Entrada | Respuesta satisfactoria |
| --- | --- | --- |
| `GET /v1/availability` | Parámetros `vehicleId`, `startsOn` y `endsOn`, cada uno una sola vez | `200`: disponibilidad y cotización, incluido `available=false` |
| `POST /v1/reservations` | JSON con los cinco campos de creación; cabecera `Idempotency-Key` | `201`: reserva `RESERVED` o repetición idempotente de la respuesta original |
| `GET /v1/reservations/{id}` | Identificador de reserva en la ruta | `200`: estado actual y condiciones de la reserva |
| `DELETE /v1/reservations/{id}` | Identificador de reserva en la ruta | `200`: reserva `CANCELLED`, también al repetir la cancelación |

La consulta no retiene el vehículo. La creación comprueba nuevamente cliente, período, precio y disponibilidad antes de ocupar la agenda de forma atómica. La cancelación conserva el registro y libera la ocupación.

El esquema de reserva contempla `RESERVED`, `ACTIVE`, `COMPLETED` y `CANCELLED`. DELETE permite la transición `RESERVED → CANCELLED`, admite la repetición sobre `CANCELLED` y rechaza `ACTIVE` o `COMPLETED` con `409 INVALID_STATE`.

## Convenciones de datos

| Concepto | Regla |
| --- | --- |
| Fechas | Días calendario válidos en formato `YYYY-MM-DD`, sin hora ni offset |
| Calendario | Sucursal en `America/Buenos_Aires` |
| Período | `[startsOn, endsOn)`: inicio incluido y fin excluido |
| Duración | Entre 1 y 30 días; del 15 al 18 de octubre corresponden 3 días |
| Solapamiento | Dos períodos adyacentes que comparten únicamente su frontera no se solapan |
| Importes | Enteros de 64 bits en centavos ARS; `5000000` equivale a ARS 50.000 |
| Cotización | `totalMinor = dailyRateMinor × days` |
| Inicio en producción | Igual o posterior a la fecha actual de la sucursal |

Las fechas imposibles y los períodos vacíos, invertidos o mayores a 30 días producen `400 INVALID_INPUT`.

Datos de demostración:

| Tipo | Identificador | Condición |
| --- | --- | --- |
| Vehículo | `auto-001` | Tarifa diaria de `5000000` centavos ARS |
| Vehículo | `auto-002` | Tarifa diaria de `6500000` centavos ARS |
| Cliente | `cli-001` | Habilitado para reservar |
| Cliente | `cli-blocked` | Bloqueado; creación rechazada con `422 CUSTOMER_NOT_ELIGIBLE` |

## Representaciones

Disponibilidad libre:

```json
{
  "vehicleId": "auto-001",
  "startsOn": "2026-10-15",
  "endsOn": "2026-10-18",
  "available": true,
  "days": 3,
  "dailyRateMinor": 5000000,
  "totalMinor": 15000000,
  "currency": "ARS"
}
```

Si el vehículo está ocupado, `available` es `false` y la respuesta incluye `unavailableReason: "OVERLAPPING_RESERVATION"`. Ese campo se omite cuando está libre. La cotización se informa en ambos casos.

Solicitud de creación, con `Content-Type: application/json`:

```json
{
  "vehicleId": "auto-001",
  "customerId": "cli-001",
  "startsOn": "2026-10-15",
  "endsOn": "2026-10-18",
  "expectedTotalMinor": 15000000
}
```

Los cinco campos son obligatorios. `expectedTotalMinor` comunica el importe aceptado de la cotización. La representación de reserva contiene `id`, `status`, `vehicleId`, `customerId`, `startsOn`, `endsOn`, `days`, `dailyRateMinor`, `totalMinor` y `currency`; el importe esperado pertenece únicamente a la solicitud.

## Idempotencia

La cabecera `Idempotency-Key` es obligatoria para crear una reserva y admite entre 1 y 128 caracteres ASCII visibles, sin espacios. Su ámbito de diseño es aplicación consumidora + operación + clave. En el mock, un único token representa una única aplicación y las claves se conservan para `POST /v1/reservations` durante la ejecución.

La misma clave y los mismos valores de los cinco campos devuelven exactamente el código HTTP y cuerpo originales. Los espacios y el orden de propiedades JSON no afectan la comparación. Cambiar los valores con una clave utilizada produce `409 IDEMPOTENCY_KEY_REUSED`.

Una solicitud JSON completa, con identificadores válidos e importe entero no negativo, vincula la clave aunque posteriormente falle la validación de fechas o de negocio. Un intento corregido o realizado después de un cambio de condiciones requiere una clave nueva. Los rechazos por autenticación, tipo de contenido, estructura JSON o campos obligatorios no consumen la clave.

La repetición idempotente de una creación conserva `201 RESERVED` incluso después de una cancelación, sin reactivar la reserva. La consulta por identificador muestra el estado actual `CANCELLED`. Repetir DELETE devuelve `200 CANCELLED`.

## Errores

Las respuestas de error utilizan el objeto `{"code":"CODIGO","message":"Descripción legible"}`.

| HTTP | Código | Motivo |
| --- | --- | --- |
| `400` | `INVALID_INPUT` | Parámetros, JSON, campos, fecha, período o clave inválidos |
| `401` | `UNAUTHORIZED` | Token ausente o incorrecto; cabecera `WWW-Authenticate: Bearer` |
| `404` | `NOT_FOUND` | Vehículo, cliente, reserva o ruta inexistente |
| `405` | `METHOD_NOT_ALLOWED` | Método no admitido; cabecera `Allow` |
| `409` | `PRICE_CHANGED` | Importe esperado distinto de tarifa × días |
| `409` | `VEHICLE_UNAVAILABLE` | Una reserva existente ocupa parte del período |
| `409` | `IDEMPOTENCY_KEY_REUSED` | Clave utilizada con valores diferentes |
| `409` | `INVALID_STATE` | Cancelación solicitada para una reserva `ACTIVE` o `COMPLETED` |
| `415` | `UNSUPPORTED_MEDIA_TYPE` | La creación requiere `Content-Type: application/json` |
| `422` | `CUSTOMER_NOT_ELIGIBLE` | Cliente conocido, bloqueado para reservar |

## Ejecución y verificación

El [mock local de reservas](../../mocks/rentals/README.md) incluye una guía PowerShell para consultar, crear, repetir la creación, demostrar un conflicto y cancelar. El programa usa Go estándar, memoria y mutex; sus datos se reinician al detenerlo. Genera `RESERVED/CANCELLED`, admite fechas pasadas para reproducibilidad y utiliza un token público de demostración. La persistencia, los servicios distribuidos, los roles individuales, el retiro, la devolución y las integraciones pertenecen al diseño del sistema final.

Desde la raíz del proyecto:

```powershell
go -C mocks/rentals test -race ./...
```

Las pruebas verifican concurrencia con una sola reserva ganadora, repetición idempotente, colisión de claves, cancelación, liberación, fechas, precio, autenticación y respuestas contra los esquemas declarados. También resuelven las referencias internas `$ref`. El verificador de respuestas cubre las palabras clave utilizadas por este contrato; la validación formal OpenAPI se realiza por separado.

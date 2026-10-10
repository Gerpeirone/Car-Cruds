# Mock local de disponibilidad y reservas

> Mock histórico de la entrega 1. Conserva el contrato y comportamiento de reservas; no implementa la nueva capacidad de empleados para el gimnasio ni la integración con la clínica. Ver [el contrato de reservas](../../docs/contracts/RESERVAS.md) y [la preparación de entrega 2](../../docs/ENTREGA-2.md).

| Aspecto | Estado |
| --- | --- |
| Fecha | 7 de octubre de 2026 |
| Hito | Entrega 1 — 9 de octubre de 2026 |
| Implementación | Servidor HTTP ejecutable con biblioteca estándar de Go |
| Contrato | [OpenAPI 3.0.3, API 1.0.0](../../docs/contracts/openapi-v1.json) |
| Almacenamiento | Memoria del proceso; datos y claves se descartan al reiniciar |

El mock demuestra consulta de disponibilidad, cotización, creación idempotente, consulta de estado y cancelación. La comprobación y ocupación del período se ejecutan bajo un mutex. Utiliza datos de demostración, un consumidor y los estados `RESERVED/CANCELLED`; admite fechas pasadas para reproducir los ejemplos. La persistencia, el gateway, los servicios independientes, los roles individuales, el retiro, la devolución y las integraciones corresponden al [diseño objetivo](../../docs/ARCHITECTURE.md).

Las reglas de negocio se especifican en [SPEC.md](../../SPEC.md). [ADR-008 — contrato propio](../../docs/adr/ADR-008-contrato-propio.md) establece la capacidad publicada y [ADR-005 — comunicación](../../docs/adr/ADR-005-comunicacion.md) define los reintentos. Los formatos, estados y errores completos se describen en la [documentación del contrato](../../docs/contracts/README.md).

## Ejecución y configuración

Requiere Go 1.22 o posterior. Desde la raíz del proyecto:

```powershell
go -C mocks/rentals run .
```

El servidor escucha en `http://127.0.0.1:8080`. `Ctrl+C` finaliza el proceso.

| Variable opcional | Valor predeterminado | Propósito |
| --- | --- | --- |
| `MOCK_ADDR` | `127.0.0.1:8080` | Dirección de escucha; por ejemplo, `127.0.0.1:8081` |
| `MOCK_API_TOKEN` | `demo-token` | Token Bearer utilizado por el servidor |

`demo-token` es un dato público de demostración. Todas las rutas requieren `Authorization: Bearer demo-token` con la configuración predeterminada. Los ejemplos siguientes utilizan esa configuración.

## Datos de demostración

| Tipo | Identificador | Condición |
| --- | --- | --- |
| Vehículo | `auto-001` | `5000000` centavos ARS por día, equivalentes a ARS 50.000 |
| Vehículo | `auto-002` | `6500000` centavos ARS por día, equivalentes a ARS 65.000 |
| Cliente | `cli-001` | Habilitado para reservar |
| Cliente | `cli-blocked` | Bloqueado; creación rechazada con `422 CUSTOMER_NOT_ELIGIBLE` |

Los períodos son `[inicio, fin)` de 1 a 30 días calendario en `America/Buenos_Aires`, con fechas `YYYY-MM-DD`. El sistema final deberá validar que el inicio sea igual o posterior al día actual de la sucursal. La consulta no retiene el vehículo; POST comprueba y ocupa el período.

## Demostración en PowerShell

Iniciar el servidor y abrir una segunda terminal en la raíz del proyecto. Ejecutar los bloques en orden sobre un proceso recién iniciado. Son compatibles con Windows PowerShell 5.1 y PowerShell 7.

### 1. Consultar disponibilidad y cotización

```powershell
$base = 'http://127.0.0.1:8080'
$headers = @{ Authorization = 'Bearer demo-token' }
$query = '/v1/availability?vehicleId=auto-001&startsOn=2026-10-15&endsOn=2026-10-18'
Invoke-RestMethod -Uri ($base + $query) -Headers $headers
```

Resultado esperado: `available=true`, `days=3`, `dailyRateMinor=5000000` y `totalMinor=15000000` centavos ARS.

### 2. Crear una reserva

```powershell
$body = @{
    vehicleId = 'auto-001'
    customerId = 'cli-001'
    startsOn = '2026-10-15'
    endsOn = '2026-10-18'
    expectedTotalMinor = 15000000
} | ConvertTo-Json -Compress
$createHeaders = @{
    Authorization = 'Bearer demo-token'
    'Idempotency-Key' = 'demo-reserva-001'
}
$reservation = Invoke-RestMethod -Method Post -Uri ($base + '/v1/reservations') -Headers $createHeaders -ContentType 'application/json' -Body $body
$reservation
```

Resultado esperado: HTTP `201`, identificador de reserva y estado `RESERVED`.

### 3. Repetir la creación con la misma clave

```powershell
$repeated = Invoke-RestMethod -Method Post -Uri ($base + '/v1/reservations') -Headers $createHeaders -ContentType 'application/json' -Body $body
[PSCustomObject]@{
    sameId = ($reservation.id -eq $repeated.id)
    status = $repeated.status
    totalMinor = $repeated.totalMinor
}
```

Resultado esperado: HTTP `201`, `sameId=true`, `status=RESERVED` y el total original. La repetición idempotente no crea otro registro.

### 4. Demostrar conflicto por solapamiento

Otra clave identifica un intento distinto sobre el mismo vehículo y período:

```powershell
$conflictHeaders = @{
    Authorization = 'Bearer demo-token'
    'Idempotency-Key' = 'demo-reserva-002'
}
try {
    Invoke-RestMethod -Method Post -Uri ($base + '/v1/reservations') -Headers $conflictHeaders -ContentType 'application/json' -Body $body -ErrorAction Stop
    throw 'Se esperaba HTTP 409 por solapamiento.'
}
catch {
    $response = $_.Exception.Response
    if ($null -eq $response -or [int]$response.StatusCode -ne 409) {
        throw
    }
    [PSCustomObject]@{
        http = [int]$response.StatusCode
        error = $_.ErrorDetails.Message
    }
}
Invoke-RestMethod -Uri ($base + $query) -Headers $headers
```

Resultado esperado: HTTP `409` con `code=VEHICLE_UNAVAILABLE`. La disponibilidad muestra `available=false` y `unavailableReason=OVERLAPPING_RESERVATION`. La reserva original permanece `RESERVED`.

### 5. Consultar, cancelar y verificar la liberación

```powershell
$reservationUri = $base + '/v1/reservations/' + $reservation.id
Invoke-RestMethod -Uri $reservationUri -Headers $headers
Invoke-RestMethod -Method Delete -Uri $reservationUri -Headers $headers
Invoke-RestMethod -Method Delete -Uri $reservationUri -Headers $headers
Invoke-RestMethod -Uri $reservationUri -Headers $headers
Invoke-RestMethod -Uri ($base + $query) -Headers $headers
```

La consulta inicial muestra `RESERVED`. Las dos cancelaciones y la consulta posterior muestran `CANCELLED`; la última disponibilidad muestra `true`. La repetición de DELETE conserva el registro y el resultado de la cancelación.

Una nueva creación después de cancelar requiere una clave nueva. La repetición de `demo-reserva-001` conserva su respuesta original `201 RESERVED`, sin modificar el estado actual de la reserva. La clave del intento rechazado también conserva su respuesta original. El alcance de idempotencia se detalla en la [documentación del contrato](../../docs/contracts/README.md#idempotencia).

## Verificación automatizada

Desde la raíz del proyecto:

```powershell
go -C mocks/rentals test -race ./...
```

Las pruebas usan servidores temporales propios, independientes del puerto 8080. Verifican una única reserva ganadora entre 24 solicitudes concurrentes con claves distintas, repetición idempotente, colisiones, cancelación, liberación, fechas, importes, autenticación y errores. También contrastan respuestas con los esquemas utilizados por el contrato y resuelven sus referencias internas.

En Windows, la detección de carreras requiere CGO y un compilador C compatible. Si no están disponibles, `go -C mocks/rentals test ./...` ejecuta las mismas pruebas sin detección de carreras. La modalidad utilizada debe registrarse junto con el resultado.

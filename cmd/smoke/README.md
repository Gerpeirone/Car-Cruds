# Verificación integrada local

Ejecutar desde la raíz, con el stack de Compose ya iniciado y saludable:

```powershell
go run ./cmd/smoke -history
```

El comando carga `.env` mediante `internal/devconfig`, únicamente en esta herramienta local. Los valores ya presentes en el entorno tienen prioridad, incluso si están vacíos. El loader admite asignaciones literales `KEY=value`, comillas simples/dobles y líneas de comentario; no ejecuta ni expande contenido. Los servicios continúan recibiendo configuración por entorno.

Todos los escenarios HTTP utilizan exclusivamente el gateway de loopback. La URL se obtiene de `-gateway`, `SMOKE_GATEWAY_URL` o `http://127.0.0.1:${APP_PORT}`, con puerto 8080 por defecto. No se siguen redirecciones ni se envían tokens a hosts remotos. Requiere `AUTH_ADMIN_TOKEN`, `AUTH_CLIENT_TOKEN` y `AUTH_GYM_TOKEN` diferentes. `-env=""` permite usar sólo el entorno; `-timeout=120s` configura el presupuesto de escenarios.

## Comprobaciones reales

- Sin autenticación: 401. Cliente sin acceso a la lista de personal: 403. Gimnasio con lista activa que contiene exclusivamente `employeeId` y `displayName` por elemento; sin acceso a fichas, CRUD administrativo ni candidatos.
- Dos instancias distintas de Flota observadas por `X-Service-Instance` en doce respuestas a través del balanceador. Escritura de una ficha propia, rechazo de versión anterior, actualización de caché y búsqueda de la versión actual en Solr mediante consultas durante un máximo de cinco segundos.
- Cotización y agenda para un intervalo vigente `[hoy, mañana)` calculado en `America/Buenos_Aires`. Reserva idempotente, rechazo de otra carga con la misma clave, cancelación repetible y respuesta original preservada después de cancelar.
- Dos solicitudes simultáneas con claves distintas: exactamente una reserva ganadora y un conflicto. Retiro y devolución repetibles, con comprobación de estado, identidad y dinero entero ARS.
- `-history`: outbox `RentalCompleted.v1` publicado e inbox/historial de Clientes para esa misma devolución, con una sola entrada incluso tras repetirla. Sólo ejecuta consultas en transacciones PostgreSQL de lectura; no accede a RabbitMQ directamente.

El historial usa `SMOKE_CUSTOMERS_DATABASE_URL` y `SMOKE_RENTALS_DATABASE_URL` si están definidos, limitados a PostgreSQL de loopback. En el stack estándar construye las direcciones de `customers` y `rentals` en `127.0.0.1:55432` con `CUSTOMERS_DB_PASSWORD` y `RENTALS_DB_PASSWORD` de la configuración local. Las direcciones y credenciales no aparecen en la salida. Sin `-history`, esa comprobación se informa como `SKIP`; no se atribuye evidencia de mensajería.

Cada solicitud incluye un UUID de correlación y exige un único encabezado de respuesta con el mismo `X-Request-ID`, además de una instancia identificable. La salida muestra `PASS`/`FAIL`, estados e identificadores; las reservas exitosas imprimen `reservationID` y `requestID` para localizar sus trazas. Código de salida: 0 si todos los escenarios habilitados y la limpieza pasan, 1 ante fallo real, 2 ante configuración inválida.

## Datos y limpieza

Cada ejecución utiliza identificadores o nombres `smoke-<UUID>` nuevos. Un contexto independiente y acotado permite intentar la limpieza también al fallar o recibir Ctrl+C: cancela reservas `RESERVED`, devuelve las `ACTIVE`, desactiva empleado y vehículo y elimina el candidato creado. Verifica que el empleado ya no aparezca al gimnasio y que el vehículo desaparezca del índice público.

Las reservas canceladas/completadas y los vehículos/empleados inactivos permanecen como evidencia. Clientes no tiene modificación ni baja en la API actual: el conductor ficticio se conserva con identificador y nombre `smoke` trazables. Una entrega HTTP incierta de reserva se recupera con su clave original antes de limpiarla. Si la limpieza falla, el comando termina con `FAIL` y muestra el identificador de ejecución para recuperar sus fixtures; no elimina datos ajenos. No usa contratos de gimnasio u hospital pendientes ni modifica el mock histórico.

Los tiempos impresos corresponden a lo observado por el harness después de comenzar sus consultas. No acreditan por sí solos capacidad máxima, failover, latencia desde el commit original ni recuperación del broker; esos ensayos se realizan por separado.

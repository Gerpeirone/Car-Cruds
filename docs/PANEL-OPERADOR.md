# Panel del operador — F3-02

Fecha: 8 de noviembre de 2026. Estado: implementado en `feature/entrega-3`, con verificación automatizada y contra servicios reales. Revisión visual en navegador pendiente.

[Resumen estructurado de la verificación](evidence/operator-validation-2026-11-08.json).

## Acceso y navegación

Abrir `/operador/acceso` desde la misma entrada web del cliente, o usar **Soy operador** en su pantalla de acceso. Se solicita exclusivamente la credencial `AUTH_ADMIN_TOKEN` del `.env` privado del conjunto; no se usa un identificador de cliente. La credencial permanece en memoria y no se incluye en URLs, almacenamiento, bundle, configuración pública ni logs.

El gateway ofrece `GET /v1/session`, autenticado y no cacheable: devuelve `role` y `customerId` únicamente cuando corresponde. El panel exige `role=admin` y rechaza client, gym y service. La autorización de cada operación sigue en su servicio propietario. Pasar del acceso de cliente al panel descarta el contexto del cliente; salir o recargar requiere ingresar nuevamente. El endpoint no crea cuentas personales ni expone tokens.

| Ruta web | Funcionalidad |
| --- | --- |
| `/operador` | Resumen con accesos a tareas; no inventa contadores de negocio |
| `/operador/flota` | Catálogo administrativo de activos/inactivos, marca exacta, orden y paginación; solicitud de reindexación |
| `/operador/flota/nuevo` | Alta de vehículo con tarifa en pesos ARS |
| `/operador/flota/:id` | Edición y baja/reactivación lógica con versión de la ficha |
| `/operador/clientes` | Lista de hasta 200 perfiles y consulta individual por ID |
| `/operador/clientes/nuevo` | Alta de perfil, actividad y fecha de licencia |
| `/operador/reservas?id=...` | Consulta por código y acciones según estado: cancelar, retirar o devolver |
| `/operador/mantenimiento` | Bloqueo de agenda por vehículo y fechas |

## Operaciones y garantías

La interfaz utiliza los contratos existentes de [Flota](contracts/fleet-openapi.json), [Clientes](contracts/customers-openapi.json) y [Alquileres](contracts/rentals-openapi.json). La única ampliación técnica de API es la [proyección de sesión del gateway](contracts/operations-openapi.json), versión 0.2.0. No se implementan nuevos CRUD de negocio.

- Las tarifas se ingresan en pesos con hasta dos decimales y se convierten exactamente a centavos, entre 1 y 1.000.000.000 centavos. Las reservas confirmadas conservan su precio.
- Editar usa la versión leída. Si aparece `VERSION_CONFLICT`, el borrador permanece visible y el operador decide explícitamente si descarta sus cambios y carga la ficha actual. No hay sobrescritura forzada.
- El alta incierta de auto o cliente ofrece consulta por el mismo ID antes de otra escritura. El formulario no reintenta POST automáticamente. Debe conservarse el ID si se abandona o recarga ese formulario.
- Clientes ofrece registro y consulta; no muestra edición/eliminación inexistentes. El máximo de 200 perfiles no se presenta como un total global paginado; se permite consultar cualquier ID autorizado.
- No existe listado general de reservas. Se busca por código. `RESERVED` permite cancelación o retiro; `ACTIVE`, devolución; los estados finales son de consulta. Las tres acciones exigen diálogo explícito y leen nuevamente el estado del servicio.
- Retirar vuelve a validar fecha, licencia/habilitación y ocupación física. Devolver conserva las garantías del backend y su evento de finalización. Un timeout no demuestra que la acción no ocurrió: se consulta el estado, sin repetir automáticamente.
- Mantenimiento valida intervalo calendario de 1 a 30 días, inicio desde hoy de Buenos Aires y fin excluido. Un conflicto no cancela reservas existentes. Antes del envío conserva vehículo/fechas en sessionStorage, sin credencial. Una respuesta incierta mantiene ese registro tras recarga y bloquea el reenvío. La API no ofrece consulta/cancelación de mantenimiento ni idempotencia de creación: la verificación del bloqueo incierto requiere intervención del equipo y no se inventa una recuperación automática.
- Reindexar muestra aceptación pendiente, no finalización del índice.

## Verificaciones ejecutadas

La suite ordinaria del frontend aprobó **36 pruebas** sin fallos; los dos escenarios de integración optativos se ejecutan por separado con su configuración. Incluye los 25 casos del recorrido de cliente y once casos adicionales del operador (tres roles rechazados, sesión/salida, importes, alta de vehículo, conflicto de versión, alta de cliente, retiro/devolución y mantenimiento incierto tras nuevo acceso, más una lectura tardía al pasar del alta a la edición).

`go test -race ./cmd/gateway ./docs` y `go vet ./cmd/gateway` aprobaron. La prueba de sesión verifica identidad derivada de credencial, rechazo de acceso ausente/inválido, proyección mínima, `no-store` y que un encabezado de rol fabricado no cambie la identidad. El validador OpenAPI comprueba **35 operaciones y 222 ejemplos** sobre seis contratos, con el endpoint nuevo incluido en el inventario del gateway.

`operator.live.test.tsx` aprobó contra gateway, servicios y almacenes reales: registró un vehículo ficticio propio, editó su tarifa, registró un cliente ficticio, preparó una reserva vigente por API, realizó retiro y devolución desde los componentes del panel y registró mantenimiento. La consulta real de disponibilidad devolvió `available=false` para el período bloqueado. El vehículo quedó inactivo y el alquiler finalizado. El perfil ficticio y el bloqueo de mantenimiento se conservan, porque sus API no ofrecen baja/eliminación; no se modifican ni borran registros ajenos.

Las pruebas React usan jsdom: verifican interacción y red real, no renderizado visual. La revisión visual móvil, contraste y teclado sigue pendiente; no se declara aprobada por estas pruebas. No se probaron integraciones de personal ni se publicó cloud.

## Reproducción

El panel forma parte del build y del mismo servidor documentado en [frontend/README](../frontend/README.md). Desde la raíz:

```powershell
docker compose -f compose.yaml -f compose.frontend.yaml up -d --build --wait --wait-timeout 300
```

La excepción local de puerto reservado de Solr y el uso de `.env` existente se describen en ese README. Las pruebas ordinarias se ejecutan con `npm --prefix frontend test`. Para la prueba real, configurar privadamente `OPERATOR_TEST_TOKEN` con la credencial administrativa y:

```powershell
$env:OPERATOR_LIVE_TEST = "true"
$env:OPERATOR_TEST_BASE_URL = "http://127.0.0.1:8081"
npm --prefix frontend test -- src/operator/operator.live.test.tsx
```

Solo admite loopback y crea fixtures propios. El token no se escribe en el comando, el informe ni Git. Para verificar la sesión real del panel se requiere reconstruir también el gateway; actualizar solo el contenedor web no incorpora el endpoint nuevo.

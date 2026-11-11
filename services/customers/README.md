# Servicio Clientes

## Cuentas personales

La portada ofrece `/registro` y `/iniciar-sesion`. Tras iniciar sesión, el cliente puede abrir `/catalogo`, fichas activas y `/perfil`. `/acceso` conserva el flujo de conductor con credencial de aplicación y perfil habilitado. Crear una cuenta personal asigna `client` en el servidor y no vincula automáticamente el usuario a un conductor. El panel de operador, Hospital y Gimnasio conservan sus permisos y credenciales.

Clientes aplica la migración aditiva `002_accounts.sql` al arrancar. `customer_accounts` guarda UUID, nombre, apellido, correo normalizado único, rol fijo y fechas de creación y actualización. La contraseña se protege con Argon2id (19 MiB, dos iteraciones, paralelismo 1, sal aleatoria de 16 bytes y salida de 32 bytes). El formato guarda versión y parámetros. Acepta de 12 caracteres a 1.024 bytes, con validación en ambos extremos y sin truncamiento. El alta válida devuelve la misma respuesta 202 si el correo ya existía, sin crear otra cuenta ni cambiar la contraseña guardada. La interfaz lo aclara sin revelar si el correo existía: si ya había una cuenta, hay que ingresar con la contraseña original. El correo se recorta y se pasa a minúsculas igual en el alta y en el login. La contraseña se usa exactamente como se escribió, sin recortes ni normalización. El login devuelve el mismo 401 para cuenta desconocida o contraseña incorrecta.

Clientes genera 32 bytes aleatorios para cada sesión. Guarda el digest del token y su vencimiento en `customer_sessions`, mientras el navegador recibe la cookie `__Host-carcruds-account` con `HttpOnly`, `Secure`, `SameSite=Strict` y ruta `/`. El token opaco no se incluye en el JSON ni se guarda en el almacenamiento web. El logout elimina la fila y expira la cookie. `CUSTOMER_SESSION_TTL` acepta entre `1m` y `168h`, con valor predeterminado `12h`.

El gateway permite las cuatro [operaciones de cuenta](../../docs/API.md#cuenta-personal) y, con cookie, solo lecturas de catálogo. Flota valida esa cookie contra Clientes y asigna el permiso interno `catalog`; la cookie sola no habilita cotizaciones internas, reservas ni administración. El frontend usa JSON del mismo origen y el backend rechaza solicitudes de escritura marcadas como cross-site. El límite es de 60 intentos por minuto por instancia y 8 por correo cada diez minutos (`429 AUTH_RATE_LIMITED`), con dos operaciones de hash simultáneas. Si ambas están ocupadas, la respuesta es `503 AUTH_BUSY` con `Retry-After: 1`: es un límite de capacidad momentáneo, no consume intentos y la interfaz no lo presenta como «demasiados intentos». Estos contadores son locales a la instancia.

En el frontend, solo `GET /v1/auth/me` decide que una sesión terminó. Si una lectura del catálogo responde 401, la interfaz vuelve a consultar `/v1/auth/me`. Si la sesión sigue vigente, muestra un error del servicio con reintento y no pide iniciar sesión otra vez. Sin esa verificación, un backend mal enrutado expulsaba al usuario al abrir una ficha. No hay verificación de correo ni recuperación de contraseña.

### Verificación y despliegue

Para pruebas de persistencia, apuntar `TEST_CUSTOMERS_DATABASE_URL` a un PostgreSQL desechable y ejecutar `go test -race ./services/customers/...`. Los tests usan un esquema temporal. En el navegador usar el mismo origen para la interfaz y la API, con HTTPS fuera de localhost.

Con un Compose desechable accesible por HTTPS en loopback y vehículos ficticios sembrados, ejecutar `npm --prefix scripts/ui-review ci`. Configurar `BROWSER` con la ruta de Edge o Chrome y `ACCOUNT_TEST_URL` con el origen HTTPS local, y ejecutar `node scripts/ui-review/accounts.mjs`. El recorrido crea una cuenta ficticia y recorre crear cuenta → iniciar sesión → catálogo → ficha → catálogo (seis veces, para pasar por ambas réplicas de Flota) → perfil → recargas → nueva pestaña → cerrar sesión. También comprueba el alta duplicada, la contraseña incorrecta, el correo normalizado, los atributos de la cookie, que no haya respuestas 401 durante la sesión, el bloqueo de rutas administrativas y que la cookie revocada deje de funcionar.

Tras aprobar el PR, registrar SHA y los archivos Compose efectivos con `docker compose ls`, respaldar PostgreSQL y conservar `.env` y volúmenes. Seguir el procedimiento de [actualización de Azure](../../docs/DESPLIEGUE.md#6-actualizar-la-vm-de-azure). Con el SHA aprobado ya seleccionado y los tres archivos en uso, ejecutar:

```bash
compose_args=(-f compose.yaml -f compose.frontend.yaml -f compose.deploy.yaml)
docker compose "${compose_args[@]}" config --quiet &&
docker compose "${compose_args[@]}" build customers gateway fleet-a fleet-b web &&
docker compose "${compose_args[@]}" up -d --wait --wait-timeout 600 &&
docker compose "${compose_args[@]}" up -d --no-deps --force-recreate --wait fleet-lb &&
docker compose "${compose_args[@]}" ps
```

**Recrear siempre `fleet-lb` después de recrear `fleet-a` o `fleet-b`.** Al recrear contenedores, Docker puede asignar a otro servicio una IP que antes tenía una réplica de Flota. En el laboratorio, `customers` quedó con la IP anterior de `fleet-a`. Un NGINX que resolvió los nombres al arrancar sigue enviando ahí parte del tráfico de Flota: Clientes responde 401 a `/v1/vehicles/{id}` y la ficha devolvía al usuario al inicio de sesión. Desde esta corrección, [load-balancer/nginx.conf](../../load-balancer/nginx.conf) vuelve a resolver los nombres cada 10 s (`resolve`). Recrear `fleet-lb` aplica esa configuración y elimina la ventana inicial.

Clientes aplica la migración al arrancar y debe quedar saludable antes de validar. Por HTTPS, probar alta, ficha y cierre de sesión con una cuenta ficticia autorizada. Verificar que esa cuenta reciba 401 en rutas de operador y comprobar Hospital y Gimnasio por lectura con sus credenciales correspondientes. Ante rollback, restaurar las imágenes de la revisión anterior con los mismos volúmenes. Las tablas nuevas son aditivas.

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

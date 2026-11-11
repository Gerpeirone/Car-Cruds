# Seed de demostración

`cmd/demoseed` carga datos ficticios para presentar Car Cruds. No se ejecuta solo: sin `-apply` ni `-cleanup` solo imprime el plan. Pasa por el gateway y las APIs de cada servicio, así que aplica las validaciones del dominio, las versiones, las claves idempotentes de Alquileres y la indexación Flota → Solr por eventos. No escribe en las bases de datos directamente.

## Qué carga

| Tipo | Registros |
| --- | --- |
| Clientes (conductores) | `demo-cli-habilitada` (activa y habilitada), `demo-cli-inactiva`, `demo-cli-licencia-vencida`, `demo-cli-alquiler-activo`, `demo-cli-reserva-futura` y `demo-cli-sin-reservas`. Tienen nombres ficticios, estado y vencimiento de licencia, que son los campos del dominio |
| Empleados | Diez empleados «Demo · …» con su función en el nombre (atención al cliente, taller, administración, logística, ventas, mecánica, reservas, preparación de vehículos, recursos humanos y seguridad): nueve activos y uno de baja. Empleados solo registra nombre visible y estado, así que la función no se guarda en otro campo. El ID lo asigna Empleados. `employee-001` y `employee-002` (Gimnasio y Hospital) no se tocan |
| Vehículos | Dieciséis autos `demo-*` de diez marcas: Corolla, Yaris, Polo, Golf, Ranger, Hilux, 208, Onix, Sandero, Civic, Renegade, Cronos, Kicks, Kangoo, Tracker y 2008. Tienen marca, modelo, categoría, transmisión, plazas y tarifa, que son los campos de Flota. Flota no tiene año, patente ni imagen, así que no se inventan |
| Reservas | Una futura para `demo-cli-reserva-futura` y un alquiler activo (reserva de hoy más retiro) para `demo-cli-alquiler-activo`. Para el conductor de la credencial de alquiler (`AUTH_CLIENT_CUSTOMER_ID`, `cli-001` por defecto) crea otra futura y otro activo con su propia credencial, para que **Mis alquileres** tenga contenido. El total lo cotiza Alquileres |

Los clientes y empleados demo no reciben contraseñas, tokens ni cuentas. Para reservar sigue haciendo falta la credencial de alquiler existente.

## Garantías

- Idempotente: los registros existentes se reconocen (409 o lista) y no se modifican. Si un registro existe con otros datos, se informa un `AVISO`. Una reserva se crea solo si ese conductor no tiene ya una `RESERVED` o `ACTIVE` en ese auto.
- No destructivo: solo crea registros `demo-*` o «Demo ·». Nunca edita ni borra registros previos.
- Seguro: rechaza orígenes que no sean loopback salvo con `-allow-remote`. Lee `AUTH_ADMIN_TOKEN` y `AUTH_CLIENT_TOKEN` del entorno o del `.env` y nunca los imprime.
- No llama a Hospital ni a Gimnasio, y no crea turnos ni cargos externos.

## Solo vehículos y empleados (`-catalog`)

Este modo amplía el catálogo y el personal sin tocar nada más. No crea clientes, reservas ni alquileres. Tampoco llama a Hospital ni a Gimnasio, y no edita, desactiva ni borra registros existentes.

| Comando | Efecto |
| --- | --- |
| `-catalog` | Vista previa de solo lectura contra el destino: lista `CREAR`, `EXISTE` u `OMITIDO` por registro y un resumen. Solo hace `GET` |
| `-catalog -apply` | Crea solo lo marcado `CREAR` y luego ejecuta la verificación |
| `-verify` | Verificación de solo lectura del catálogo publicado |

- **Sin duplicados:**
  - Un vehículo se omite si existe su identificador o si otro vehículo, activo o inactivo, ya tiene la misma marca y modelo.
  - Un empleado se omite si existe otro con el mismo nombre. La comparación ignora mayúsculas y espacios repetidos.
- **Idempotencia:** una segunda ejecución no crea nada.
- **Qué verifica:**
  - Que todos los vehículos activos de Flota (MongoDB) figuren en la búsqueda Solr; espera hasta 60 s al indexador.
  - Que haya al menos 14 modelos distintos.
  - Búsqueda de texto, filtros por marca, categoría, transmisión y plazas, orden por precio en ambos sentidos y paginado sin repetidos.
  - Cantidad de empleados demo en el panel.

Cada problema se informa como `AVISO` y el comando termina con código 1. Flota no tiene año, patente ni imagen, así que esos datos no se cargan.

## Uso

```bash
go run ./cmd/demoseed                         # plan, sin cambios
go run ./cmd/demoseed -apply                  # cargar (gateway local http://127.0.0.1:${APP_PORT:-8080})
go run ./cmd/demoseed -apply                  # repetir: no crea nada nuevo
go run ./cmd/demoseed -catalog                # vista previa de vehículos y empleados (solo lectura)
go run ./cmd/demoseed -catalog -apply         # cargar solo vehículos y empleados faltantes, y verificar
go run ./cmd/demoseed -verify                 # verificar Solr, filtros, orden y paginado (solo lectura)
go run ./cmd/demoseed -cleanup                # cancelar reservas demo y desactivar autos y empleados demo
go run ./cmd/demoseed -cleanup -return-active # además registrar la devolución de los alquileres demo activos
```

Las reservas con fechas relativas se calculan el día de la carga en America/Buenos_Aires. Un alquiler activo necesita que el auto esté libre hoy; si no lo está, se informa y se omite.

**Retiro:** las reservas `RESERVED` se cancelan; los autos se desactivan (`active=false`, con su versión), y los empleados demo pasan a baja. Clientes no expone la baja de conductores, así que `demo-cli-*` se conservan inertes. Un alquiler `ACTIVE` solo termina con la devolución, como en el dominio. La devolución publica `RentalCompleted.v1`, que queda en el historial del conductor, y por eso requiere `-return-active`.

En Azure no se ejecuta automáticamente. Cargar datos ficticios en la VM pública requiere una decisión del equipo y `-allow-remote` (o ejecutarlo en la VM contra su gateway local). No se usa para datos reales.

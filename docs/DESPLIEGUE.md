# Despliegue público

| Campo | Valor |
| --- | --- |
| Versión documental | 0.2 |
| Fecha | 9 de noviembre de 2026 |
| Estado | Configuración con interfaz web verificada con HTTPS local; VM y URL pública pendientes |
| Objetivo | Publicar la interfaz completa y la capacidad compartida para la presentación grupal |

## 1. Topología

Una VM Linux ejecuta el mismo `compose.yaml` del entorno local, más [compose.deploy.yaml](../compose.deploy.yaml), que agrega **Caddy** como única entrada pública:

```text
Internet ──HTTPS 443──> Caddy ─┬─ /v1/*  ──> gateway ──> Clientes / Flota (NGINX, 2 réplicas) / Alquileres / Empleados
                               └─ resto  ──> web (NGINX, interfaz estática)
```

- Caddy obtiene y renueva el certificado con Let's Encrypt para `PUBLIC_HOST` y redirige HTTP a HTTPS.
- Las rutas `/internal/*` responden 404 en Caddy. `/v1/*` llega directo al gateway, que exige el token Bearer: gimnasio y hospital usan la misma URL que antes. Cualquier otra ruta la sirve el contenedor `web` (SPA del cliente, del operador y de personal), que no publica puertos propios.
- PostgreSQL, MongoDB, Redis, RabbitMQ, Solr y Jaeger quedan ligados a `127.0.0.1` de la VM: no son accesibles desde Internet.
- Es la configuración de laboratorio de [D13](adr/ADR-013-capacidad-costos.md): una VM, sin alta disponibilidad.

## 2. Requisitos de la VM

- **Kernel Linux menor que 6.19.** MongoDB 8.x se niega a iniciar en kernels 6.19 o superiores por un error de TCMalloc/rseq ([SERVER-121912](https://jira.mongodb.org/browse/SERVER-121912)). La excepción para 7.0.14+ no se aplica en Ubuntu, porque `uname -r` informa `7.0.0` ([SERVER-136848](https://jira.mongodb.org/browse/SERVER-136848), abierto). Usar Ubuntu 24.04 con su kernel estándar 6.8, sin HWE, y no Ubuntu 26.04. Comprobar con `uname -r` antes de desplegar.
- Ubuntu 24.04, **8 GB de memoria** y 2 vCPU como mínimo (la base de D13, USD 44/mes en Lightsail). Con 4 GB, Solr, MongoDB y RabbitMQ compiten por memoria.
- Puertos abiertos en el firewall del proveedor: 22 (SSH), 80 y 443. No abrir otros.
- Un nombre DNS que apunte a la IP pública. Sin dominio propio sirve `sslip.io`: la IP `203.0.113.10` se publica como `203-0-113-10.sslip.io`. El script lo configura solo si `PUBLIC_HOST` está vacío.
- IP estática (en Lightsail, "Static IP"), para que la URL no cambie al reiniciar.

## 3. Puesta en marcha

```bash
git clone https://github.com/Gerpeirone/Car-Cruds.git
cd Car-Cruds
git checkout integracion/panel-integraciones
bash scripts/deploy-vm.sh
```

[deploy-vm.sh](../scripts/deploy-vm.sh) instala Docker si falta, genera `.env` con credenciales aleatorias (mediante `cmd/devsetup` dentro de un contenedor Go), define `PUBLIC_HOST` y levanta todo con `--wait`. Es repetible: conserva `.env` y los volúmenes de datos. Para actualizar, `git pull` y volver a ejecutarlo.

La rama indicada contiene la interfaz y la entrada pública de la entrega 3. Cambiarla a `main` únicamente cuando el equipo haya aprobado e integrado la propuesta. El script valida Compose, restringe `.env` a `0600` y comprueba que el motor y el plugin Compose estén disponibles. Todos los servicios permanentes del despliegue tienen `restart: unless-stopped`; `rabbit-init` es una tarea de inicialización y no debe reiniciarse en bucle. La recuperación debe comprobarse reiniciando la VM y ejecutando la verificación externa. `--wait` confirma el estado local de los contenedores; no acredita DNS, emisión del certificado ni acceso desde Internet.

Los controles reproducibles son `bash scripts/test-deploy-vm.sh` (seis escenarios sin instalaciones ni conexiones), `bash -n scripts/deploy-vm.sh` y `docker compose -f compose.yaml -f compose.deploy.yaml config --format json | python3 scripts/check-deploy-config.py` (con `.env` y `PUBLIC_HOST` configurados). La CI además valida el Caddyfile con la imagen de Caddy fijada. Estos controles no sustituyen un ensayo de recuperación en la VM definitiva.

Verificación local de la topología, el 9 de noviembre, con `PUBLIC_HOST=localhost` (Caddy emite un certificado interno) y `docker compose -f compose.yaml -f compose.deploy.yaml up -d --build --wait`:

| Solicitud | Resultado |
| --- | --- |
| `https://localhost/` y `/operador/personal` (enlace directo) | 200, `index.html` de la SPA |
| `https://localhost/v1/employees` sin token | 401 del gateway |
| `https://localhost/v1/employees` con token del gimnasio | 200, lista mínima |
| `https://localhost/internal/ready` | 404 de Caddy, sin cuerpo |
| `http://localhost/` | 308 a HTTPS |

La interfaz se revisó en navegador sobre esa misma entrada ([evidencia](EVIDENCIA-FRONTEND.md#6-revisión-en-navegador-real--9-de-noviembre)). Esto no acredita DNS público, un certificado de Let's Encrypt ni acceso desde Internet.

Comprobación desde fuera de la VM:

```bash
curl -i https://<PUBLIC_HOST>/v1/employees                                  # 401: exige token
curl -s -H "Authorization: Bearer <AUTH_GYM_TOKEN>" \
  "https://<PUBLIC_HOST>/v1/employees?page=1&pageSize=20"                   # 200: lista paginada
```

Verificación automática, sin modificar datos, desde una máquina con Go y el token del gimnasio en `AUTH_GYM_TOKEN`:

```bash
go run ./cmd/checkshared -url "$CAR_CRUDS_URL" \
  -contract docs/contracts/employees-openapi.json -employee employee-001
```

Definir `CAR_CRUDS_URL` con el origen HTTPS de la VM y ajustar `-employee` al ID ficticio activo elegido. La herramienta valida TLS, rechaza redirecciones y comprueba autenticación, paginación, permisos, estado y coincidencia con el archivo servido en `/contracts/employees-openapi.json`. Para una comprobación local en la VM puede usarse `-env .env`. Registrar la prueba bilateral por separado.

`SEED_DEMO=true` (valor que genera `devsetup`) carga empleados ficticios para la prueba conjunta. Los empleados reales se dan de alta con el token de administración por la API de Empleados.

## 4. Credenciales

- `.env` vive solo en la VM, con permisos `0600`, y nunca se versiona.
- Configurar `AUTH_GYM_TOKEN_EXPIRES_AT` en RFC3339 antes de compartir el token. Al gimnasio se le entrega únicamente `AUTH_GYM_TOKEN`, por canal privado. [Procedimiento de entrega](contracts/INTEGRACION_GIMNASIO.md#emitir-y-entregar-el-token-de-prueba)
- Rotar un token: editar `.env` y volver a ejecutar el script; avisar al consumidor antes.

## 5. Operación

| Tarea | Comando (en la VM, desde la raíz del repositorio) |
| --- | --- |
| Estado | `docker compose -f compose.yaml -f compose.deploy.yaml ps` |
| Registros de la entrada | `docker compose -f compose.yaml -f compose.deploy.yaml logs -f caddy gateway` |
| Trazas (Jaeger) | Túnel SSH: `ssh -L 16686:127.0.0.1:16686 <vm>` y abrir `http://localhost:16686` |
| Tablero y alertas | Agregar `-f compose.observability.yaml` a los comandos de Compose y `GRAFANA_ADMIN_PASSWORD` al `.env` (`openssl rand -hex 24`). Túnel `ssh -L 3000:127.0.0.1:3000 -L 9090:127.0.0.1:9090 <vm>`: Grafana en `http://localhost:3000` y alertas en `http://localhost:9090/alerts` |
| Detener | `docker compose -f compose.yaml -f compose.deploy.yaml down` (conserva datos) |

La capacidad compartida debe permanecer accesible hasta finalizar la evaluación: no detener la VM sin acordarlo con el gimnasio.

## 6. Actualizar la VM de Azure

La VM `vm-carcruds` (Ubuntu 24.04) publica `https://57-156-56-201.sslip.io`. Al 11/11/2026 su interfaz no incluye Personal, Hospital ni Gimnasio. Además, la VM sirve la interfaz aunque el Caddyfile de `main` envía todo al gateway: tiene configuración local que no está en Git. Por eso el primer paso es revisar y guardar esos cambios. **Estos comandos no se ejecutaron; requieren autorización del equipo.**

```bash
cd ~/Car-Cruds                      # ruta real del checkout en la VM
uname -r                             # debe ser menor que 6.19 (MongoDB 8)
git status --short                   # cambios locales de la VM
git diff > ~/vm-cambios-$(date +%F).patch   # guardarlos antes de tocar nada
cp .env ~/env-respaldo-$(date +%F)          # copia privada, nunca a Git
git log --oneline -1                 # anotar este commit para volver atrás
git fetch origin
git stash push -m "vm-local-$(date +%F)"    # solo si git status mostró cambios
git checkout integracion/panel-integraciones   # o main, cuando el equipo la integre
grep -c '^CLINIC_' .env              # debe dar 2: CLINIC_BASE_URL y CLINIC_API_KEY
docker compose -f compose.yaml -f compose.deploy.yaml config --quiet
docker compose -f compose.yaml -f compose.deploy.yaml up -d --build --wait --wait-timeout 600
```

No usar `down -v` ni borrar volúmenes: conservan PostgreSQL (con la evidencia real del gimnasio y el turno del hospital), MongoDB y los certificados de Caddy. Si falta `CLINIC_API_KEY`, agregar las dos líneas de [COMUNICACION-HOSPITAL](COMUNICACION-HOSPITAL.md) antes del `up`: con la URL y sin clave, Empleados no inicia.

Verificación desde fuera de la VM:

```bash
curl -s -o /dev/null -w "%{http_code}\n" https://57-156-56-201.sslip.io/operador/hospital   # 200 (SPA)
curl -s -o /dev/null -w "%{http_code}\n" https://57-156-56-201.sslip.io/v1/employees        # 401
npm --prefix scripts/ui-review ci
BASE_URL=https://57-156-56-201.sslip.io BROWSER="<Chrome o Edge>" ADMIN_TOKEN=... CLIENT_TOKEN=... node scripts/ui-review/review.mjs
```

En el panel, Gimnasio con período `2026-11` debe mostrar la evidencia de `employee-001` con origen `gimnasio-pablo`. Para volver atrás: `git checkout <commit anotado>`, repetir el `up` y, si había cambios locales, `git stash pop`.

## 7. Pendientes

1. Crear la VM, fijar la IP y ejecutar el despliegue; registrar aquí y en el [contrato](contracts/README.md) la URL y la fecha.
2. Prueba conjunta con el gimnasio contra la URL pública.
3. Para la presentación final: respaldos programados de PostgreSQL y MongoDB, y tablero/alertas según [D11](adr/ADR-011-observabilidad.md).

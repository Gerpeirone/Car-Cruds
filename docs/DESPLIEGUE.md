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

La VM `vm-carcruds` (Ubuntu 24.04) publica `https://57-156-56-201.sslip.io`. Al 11/11/2026 su interfaz no incluye Personal, Hospital ni Gimnasio, y sirve la interfaz con configuración local que no está en Git. Este procedimiento actualiza la VM sin perder datos. **Requiere autorización del equipo.**

**Por qué no se pierden datos:** `compose.yaml` fija `name: car-cruds`, así que los volúmenes (`car-cruds_postgres-data`, `car-cruds_mongo-data`, etc.) son siempre los mismos y `up` los reutiliza. La versión no agrega migraciones, por lo que volver atrás no exige tocar el esquema. **Nunca usar `down`, `rm` ni la opción `-v`.**

### A. En la computadora (antes)

```bash
git fetch origin
gh run list --branch integracion/panel-integraciones --limit 2   # ambas en success para el commit a desplegar
git rev-parse --short origin/integracion/panel-integraciones       # commit a desplegar (<COMMIT>)
ssh <usuario>@57.156.56.201
```

### B. En la VM: preparar y respaldar (no cambia nada)

```bash
cd ~/Car-Cruds                          # ruta real del checkout
FECHA=$(date +%F-%H%M); mkdir -p ~/respaldos/$FECHA && chmod 700 ~/respaldos/$FECHA
uname -r                                # menor que 6.19 (MongoDB 8)
df -h /                                 # unos 5 GB libres
git rev-parse --short HEAD | tee ~/respaldos/$FECHA/commit-anterior.txt
docker compose ls                       # anotar CONFIG FILES en uso
docker ps --format '{{.Names}} {{.Status}}' > ~/respaldos/$FECHA/contenedores.txt
git status --short | tee ~/respaldos/$FECHA/git-status.txt
git diff > ~/respaldos/$FECHA/cambios-locales.patch
git ls-files --others --exclude-standard | tar -czf ~/respaldos/$FECHA/archivos-sin-seguimiento.tgz -T - 2>/dev/null
cp .env ~/respaldos/$FECHA/env && chmod 600 ~/respaldos/$FECHA/env
docker compose exec -T postgres pg_dumpall -U postgres | gzip > ~/respaldos/$FECHA/postgres.sql.gz
docker compose exec -T mongo sh -c 'mongodump --archive --gzip -u root -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin' > ~/respaldos/$FECHA/mongo.archive.gz
gunzip -t ~/respaldos/$FECHA/postgres.sql.gz && ls -lh ~/respaldos/$FECHA
docker compose exec -T postgres psql -U postgres -d employees -c "SELECT employee_id, period, source_app, source_reference, version FROM membership_evidence WHERE employee_id='employee-001' AND period='2026-11';"
docker compose exec -T postgres psql -U postgres -d employees -c "SELECT count(*) FROM membership_evidence_history;" | tee ~/respaldos/$FECHA/historial-antes.txt
```

Si `docker compose ls` muestra archivos distintos de `compose.yaml` y `compose.deploy.yaml`, detenerse y adaptar el paso C: la VM fue levantada de otra forma.

### C. En la VM: actualizar

```bash
git stash push --include-untracked -m "vm-local-$FECHA"   # solo si hubo cambios en el paso B
git checkout --detach <COMMIT>
grep -c '^CLINIC_' .env                 # si no da 2, agregar con nano:
#   CLINIC_BASE_URL=https://api-turnos.srv1779429.hstgr.cloud/v1
#   CLINIC_API_KEY=<clave de Car Cruds, por canal privado>
chmod 600 .env
grep '^PUBLIC_HOST=' .env               # 57-156-56-201.sslip.io
docker compose -f compose.yaml -f compose.deploy.yaml config --quiet
docker compose -f compose.yaml -f compose.deploy.yaml build
docker compose -f compose.yaml -f compose.deploy.yaml up -d --wait --wait-timeout 600
git rev-parse --short HEAD              # <COMMIT>
docker compose -f compose.yaml -f compose.deploy.yaml ps
docker compose exec -T postgres psql -U postgres -d employees -c "SELECT employee_id, period, source_app, source_reference, version FROM membership_evidence WHERE employee_id='employee-001' AND period='2026-11';"
docker compose exec -T postgres psql -U postgres -d employees -c "SELECT count(*) FROM membership_evidence_history;"   # igual que antes
```

El `build` corre mientras el sitio sigue atendiendo; `up` recrea solo los contenedores que cambiaron.

### D. En la computadora (verificar desde afuera)

```bash
curl -s -o /dev/null -w "%{http_code}\n" https://57-156-56-201.sslip.io/operador/hospital   # 200
curl -s -o /dev/null -w "%{http_code}\n" https://57-156-56-201.sslip.io/v1/employees        # 401
curl -s https://57-156-56-201.sslip.io/ | grep -o '/assets/index-[^"]*\.js'                # <BUNDLE>
curl -s https://57-156-56-201.sslip.io<BUNDLE> | grep -c "Gimnasio – Membresías"          # 1 o más
```

En el panel, Gimnasio con período `2026-11` muestra `employee-001`, origen `gimnasio-pablo`, versión 1 y referencia `15ae8f6d-ce74-4453-a694-13b23cdf7c64`; Hospital muestra el turno confirmado y el cargo de ARS 25.000. No hacer reservas nuevas: cada una se factura.

### E. En la VM: volver atrás

```bash
cd ~/Car-Cruds
git checkout --detach $(cat ~/respaldos/<FECHA>/commit-anterior.txt)
git stash list                          # "vm-local-<FECHA>"
git stash pop
docker compose <los mismos -f anotados en el paso B> up -d --build --wait --wait-timeout 600
```

Sin migraciones, los datos sirven con la versión anterior. Los respaldos del paso B son para una corrupción de datos; restaurarlos reemplaza información y requiere acordarlo antes.

## 7. Pendientes

1. Crear la VM, fijar la IP y ejecutar el despliegue; registrar aquí y en el [contrato](contracts/README.md) la URL y la fecha.
2. Prueba conjunta con el gimnasio contra la URL pública.
3. Para la presentación final: respaldos programados de PostgreSQL y MongoDB, y tablero/alertas según [D11](adr/ADR-011-observabilidad.md).

# Despliegue público

| Campo | Valor |
| --- | --- |
| Versión documental | 0.1 |
| Fecha | 15 de octubre de 2026 |
| Estado | Configuración preparada; VM y URL pública pendientes |
| Objetivo | Publicar la lista de empleados para el gimnasio antes de la entrega 2 (23 de octubre) |

## 1. Topología

Una VM Linux ejecuta el mismo `compose.yaml` del entorno local, más [compose.deploy.yaml](../compose.deploy.yaml), que agrega **Caddy** como única entrada pública:

```text
Internet ──HTTPS 443──> Caddy ──HTTP──> gateway ──> Clientes / Flota (NGINX, 2 réplicas) / Alquileres / Empleados
```

- Caddy obtiene y renueva el certificado con Let's Encrypt para `PUBLIC_HOST` y redirige HTTP a HTTPS.
- Las rutas `/internal/*` no se publican; el resto llega al gateway, que exige el token Bearer.
- PostgreSQL, MongoDB, Redis, RabbitMQ, Solr y Jaeger quedan ligados a `127.0.0.1` de la VM: no son accesibles desde Internet.
- Es la configuración de laboratorio de [D13](adr/ADR-013-capacidad-costos.md): una VM, sin alta disponibilidad.

## 2. Requisitos de la VM

- Ubuntu 24.04, **8 GB de memoria** y 2 vCPU como mínimo (la base de D13, USD 44/mes en Lightsail). Con 4 GB, Solr, MongoDB y RabbitMQ compiten por memoria.
- Puertos abiertos en el firewall del proveedor: 22 (SSH), 80 y 443. No abrir otros.
- Un nombre DNS que apunte a la IP pública. Sin dominio propio sirve `sslip.io`: la IP `203.0.113.10` se publica como `203-0-113-10.sslip.io`. El script lo configura solo si `PUBLIC_HOST` está vacío.
- IP estática (en Lightsail, "Static IP"), para que la URL no cambie al reiniciar.

## 3. Puesta en marcha

```bash
git clone https://github.com/Gerpeirone/Car-Cruds.git
cd Car-Cruds
git checkout entrega-2/cierre
bash scripts/deploy-vm.sh
```

[deploy-vm.sh](../scripts/deploy-vm.sh) instala Docker si falta, genera `.env` con credenciales aleatorias (mediante `cmd/devsetup` dentro de un contenedor Go), define `PUBLIC_HOST` y levanta todo con `--wait`. Es repetible: conserva `.env` y los volúmenes de datos. Para actualizar, `git pull` y volver a ejecutarlo.

La rama indicada contiene la preparación de E2. Cambiarla a `main` únicamente cuando el equipo haya aprobado e integrado la propuesta. El script valida Compose, restringe `.env` a `0600` y comprueba que el motor y el plugin Compose estén disponibles. Todos los servicios permanentes del despliegue tienen `restart: unless-stopped`; `rabbit-init` es una tarea de inicialización y no debe reiniciarse en bucle. La recuperación debe comprobarse reiniciando la VM y ejecutando la verificación externa. `--wait` confirma el estado local de los contenedores; no acredita DNS, emisión del certificado ni acceso desde Internet.

Comprobación desde fuera de la VM:

```bash
curl -i https://<PUBLIC_HOST>/v1/employees                                  # 401: exige token
curl -s -H "Authorization: Bearer <AUTH_GYM_TOKEN>" \
  "https://<PUBLIC_HOST>/v1/employees?page=1&pageSize=20"                   # 200: lista paginada
```

Verificación automática, sin modificar datos, desde una máquina con Go y el token del gimnasio en `AUTH_GYM_TOKEN`:

```bash
go run ./cmd/checkshared -url https://<PUBLIC_HOST>
```

La herramienta exige HTTPS fuera de loopback, rechaza redirecciones y comprueba autenticación, contrato mínimo, paginación y denegación de lecturas administrativas. No imprime nombres ni tokens. Para una comprobación local en la VM puede usarse `-env .env`; al consumidor se entrega únicamente su token, nunca ese archivo. Una comprobación satisfactoria no equivale a aceptación del contrato por el gimnasio.

`SEED_DEMO=true` (valor que genera `devsetup`) carga empleados ficticios para la prueba conjunta. Los empleados reales se dan de alta con el token de administración por la API de Empleados.

## 4. Credenciales

- `.env` vive solo en la VM, con permisos `0600`, y nunca se versiona.
- Al gimnasio se le entrega únicamente `AUTH_GYM_TOKEN`, por canal privado. Los tokens de administración y de servicio no salen de la VM.
- Rotar un token: editar `.env` y volver a ejecutar el script; avisar al consumidor antes.

## 5. Operación

| Tarea | Comando (en la VM, desde la raíz del repositorio) |
| --- | --- |
| Estado | `docker compose -f compose.yaml -f compose.deploy.yaml ps` |
| Registros de la entrada | `docker compose -f compose.yaml -f compose.deploy.yaml logs -f caddy gateway` |
| Trazas (Jaeger) | Túnel SSH: `ssh -L 16686:127.0.0.1:16686 <vm>` y abrir `http://localhost:16686` |
| Detener | `docker compose -f compose.yaml -f compose.deploy.yaml down` (conserva datos) |

La capacidad compartida debe permanecer accesible hasta finalizar la evaluación: no detener la VM sin acordarlo con el gimnasio.

## 6. Pendientes

1. Crear la VM, fijar la IP y ejecutar el despliegue; registrar aquí y en el [contrato](contracts/README.md) la URL y la fecha.
2. Prueba conjunta con el gimnasio contra la URL pública.
3. Para la presentación final: respaldos programados de PostgreSQL y MongoDB, y tablero/alertas según [D11](adr/ADR-011-observabilidad.md).

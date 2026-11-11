# Frontend del alquiler

Interfaz React/TypeScript/Vite de F3-01, implementada en la entrega 3 (PR #8). Acceso de laboratorio, búsqueda/filtros, ficha, disponibilidad, confirmación idempotente, recuperación, consulta por código y cancelación. Tipografías locales e ilustración vectorial neutra propia; diseño adaptable en CSS.

## Desarrollo local

Requiere Node **24.11.1** y el backend saludable por gateway en `http://127.0.0.1:8080`. Desde `frontend/`:

```powershell
npm ci
npm run dev
```

Abrir [la interfaz local](http://127.0.0.1:5173). Vite reenvía `/v1/*` al gateway; el navegador no se conecta a almacenes ni microservicios directamente. El servidor escucha exclusivamente en loopback. Usar el identificador vinculado a `AUTH_CLIENT_CUSTOMER_ID` (predeterminado `cli-001`) y el valor de `AUTH_CLIENT_TOKEN` del `.env` privado del backend. No usar tokens administrativos, del gimnasio ni del servicio.

No hay registro público ni login con contraseña personal. La credencial se conserva solo en memoria y se solicita otra vez al recargar; la operación de reserva incierta puede recuperarse después de volver a entrar al mismo perfil. No copiar `.env` dentro del frontend ni definir tokens en variables `VITE_*`.

## Build y servidor estático

```powershell
npm run build
npm run preview
```

El build comprueba tipos y genera `dist/`. Preview sirve para revisar ese build en `http://127.0.0.1:4173`, con proxy al gateway; no es un servidor de publicación cloud.

Desde la raíz del repositorio, la alternativa Docker construye la interfaz y la sirve con NGINX:

```powershell
docker compose -f compose.yaml -f compose.frontend.yaml up -d --build --wait --wait-timeout 300
```

Abrir [la entrada web del conjunto](http://127.0.0.1:8081). NGINX sirve archivos y enlaces directos de la SPA, pasa `/v1/*` al gateway y bloquea `/internal/*`. El puerto se configura con `WEB_PORT`. Se reutiliza el `.env` privado del conjunto; si está en otro checkout del mismo entorno, indicarlo con `--env-file <ruta>` sin copiarlo al frontend. La configuración pública de Caddy y su publicación quedan para la etapa de despliegue.

En esta computadora Windows reserva el puerto 58983 de Solr. La revisión usó una superposición local ignorada por Git en `tmp/frontend/compose.override.yaml`:

```yaml
services:
  solr:
    ports: !override
      - "127.0.0.1:18983:8983"
```

Agregar `-f tmp/frontend/compose.override.yaml` al comando de Compose cuando sea necesario; requiere una versión de Compose compatible con `!override`. El puerto interno de Solr no cambia. Detener con el mismo par/trío de archivos y `stop`; conservar los volúmenes existentes.

## Pruebas

```powershell
npm test
npm run build
```

La suite ordinaria usa respuestas controladas e incluye pérdida de respuesta y recuperación tras remontar la aplicación. La prueba adicional `src/live.test.tsx` se omite salvo configuración explícita:

```powershell
$env:FRONTEND_LIVE_TEST = "true"
$env:FRONTEND_TEST_BASE_URL = "http://127.0.0.1:8081"
$env:FRONTEND_TEST_CUSTOMER_ID = "cli-001"
# Cargar privadamente FRONTEND_TEST_CLIENT_TOKEN y FRONTEND_TEST_ADMIN_TOKEN
# con credenciales distintas del mismo entorno local. No escribir valores en Git.
npm test -- src/live.test.tsx
```

Esta prueba solo admite loopback. Crea un vehículo ficticio exclusivo, usa el cliente habilitado configurado, provoca una respuesta perdida después del commit, recupera la misma reserva y la cancela. En la limpieza desactiva su propio vehículo y conserva la reserva final; no borra datos ajenos. Se requiere el conjunto real saludable y licencia vigente del perfil. No ejecutarla contra un proveedor externo ni con datos de producción.

## Estructura y límites

`src/App.tsx` contiene rutas y pantallas; `api.ts`, transporte y errores; `domain.ts`, fechas/importes/operación pendiente; `CarArt.tsx`, ilustración propia; `styles.css`, diseño. `main.tsx` carga fuentes locales. El lockfile fija las dependencias. La CI de frontend ejecuta `npm ci`, pruebas ordinarias y build.

La API no lista reservas: se consulta una propia por código. El precio/agenda/propiedad siguen siendo autoridad del backend. El [panel del operador](../docs/PANEL-OPERADOR.md) está implementado en `/operador/acceso`: usa exclusivamente `AUTH_ADMIN_TOKEN`, sin ID de cliente, y verifica `role=admin` por `/v1/session`. La sección [Personal](../docs/PERSONAL-OPERADOR.md) (`/operador/personal`) cubre empleados, membresías del gimnasio y turnos médicos. En la VM, Caddy sirve esta interfaz y deriva `/v1/*` al gateway ([despliegue](../docs/DESPLIEGUE.md)).

Criterio para la futura interfaz de personal: el informe de clínica distingue turno confirmado, ausencia confirmada y estado desconocido; el de gimnasio distingue uso, no uso e información faltante. La interfaz mostrará fecha/origen y condiciones pendientes; no tratará una falla externa como un resultado negativo ni generará tarifas o conclusiones médicas. Nuevas vinculaciones, intenciones y primer envío de turnos requieren empleado activo; la baja conserva seguimiento previo.

- [Evidencia y limitación de revisión visual](../docs/EVIDENCIA-FRONTEND.md).
- [Especificación de pantallas y criterios](../docs/FRONTEND-ALQUILER.md).
- [Plan de entrega 3](../docs/ENTREGA-3.md).
- [ADR-020](../docs/adr/ADR-020-frontend-alquiler.md).

## Prueba real del operador

`src/operator/operator.live.test.tsx` se activa con `OPERATOR_LIVE_TEST=true`, `OPERATOR_TEST_BASE_URL` de loopback y `OPERATOR_TEST_TOKEN` cargado privadamente. Registra fixtures propios de auto/cliente, opera retiro y devolución y crea mantenimiento. Desactiva su auto y conserva cliente y mantenimiento por falta de API de baja/eliminación. Ver [guía y evidencia](../docs/PANEL-OPERADOR.md).

## Referencias

- [SPEC.md — actores, alcance y reglas](../SPEC.md).
- [Arquitectura del sistema](../docs/ARCHITECTURE.md).
- [D1 vigente — alcance de personal](../docs/adr/ADR-018-empleados-sin-candidatos.md).
- [D8 vigente — empleados](../docs/adr/ADR-017-contrato-empleados.md).
- [Contrato histórico de reservas](../docs/contracts/RESERVAS.md).
- [Historias y criterios de aceptación](../docs/BACKLOG.md).

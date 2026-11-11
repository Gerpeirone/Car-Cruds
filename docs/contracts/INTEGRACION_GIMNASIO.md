# Integración de Car Cruds con Gimnasio

Car Cruds publica una lista mínima de empleados activos y recibe evidencia de membresías. Gimnasio vincula manualmente el `employeeId` con su socio y administra los datos del socio, los planes, los ingresos y los pagos. Su equipo informó que ya implementó la consulta, la vinculación y el envío mensual.

## Entorno y contrato

La versión implementada es **0.4.0**, descrita en **OpenAPI 3.0.3**. Conserva las operaciones de lista y recepción de evidencia de 0.3.0 y agrega la consulta de estado laboral por ID. El archivo de referencia es [employees-openapi.json](employees-openapi.json). La configuración de Caddy sirve ese mismo archivo en `/contracts/employees-openapi.json` sin autenticación.

La URL pública, su certificado y la versión efectivamente servida se registran después de verificar el entorno desde fuera de la VM. Al preparar esta guía, el repositorio no identificaba un origen público verificado. El procedimiento de publicación está en [Despliegue](../DESPLIEGUE.md).

## Acceso

Enviar `Authorization: Bearer <token>` por HTTPS. La credencial es opaca y el servidor resuelve su identidad como `gimnasio-pablo` y su rol como `gym`. El gateway reenvía el Bearer a Empleados, que lo valida y aplica los permisos de la operación. El token también permite consultar el rol propio con `GET /v1/session`, documentado en [Operaciones](operations-openapi.json). Administración, informes y datos clínicos requieren otros roles.

| Método | Ruta | Resultado |
| --- | --- | --- |
| GET | `/v1/employees?page=1&pageSize=100` | Lista paginada de empleados activos con `employeeId` y `displayName`. |
| GET | `/v1/employees/{id}/status` | Estado laboral mínimo de un ID conocido, aunque esté inactivo. |
| POST | `/v1/employees/membership-evidence` | Recibe una fotografía del período. Responde 200 con `applied`. |

### Emitir y entregar el token de prueba

1. En una VM nueva, ejecutar `go run ./cmd/devsetup` desde la raíz o dejar que [deploy-vm.sh](../../scripts/deploy-vm.sh) lo ejecute. El generador escribe `AUTH_GYM_TOKEN` aleatorio en el `.env` privado. En un entorno existente, conservar las demás credenciales. Para rotar la del gimnasio, generar un valor aleatorio de 32 bytes con el gestor de secretos y reemplazar solo `AUTH_GYM_TOKEN`.
2. En el mismo `.env`, fijar `AUTH_GYM_TOKEN_EXPIRES_AT` en RFC3339 UTC para una fecha posterior a la prueba. En Bash, `date -u -d '+7 days' +%Y-%m-%dT%H:%M:%SZ` calcula un ejemplo de vigencia. El middleware devuelve 401 desde ese instante. La variable se puede omitir en desarrollo local para conservar la configuración anterior.
3. Ejecutar `bash scripts/deploy-vm.sh` para aplicar el entorno. Si se rotó el token, la credencial anterior deja de funcionar en las instancias recreadas. Verificar desde fuera de la VM antes de entregar.
4. Entregar únicamente el valor de `AUTH_GYM_TOKEN` mediante el canal privado acordado, por ejemplo un enlace de un solo uso de un gestor de secretos. Comunicar por separado la URL, el contrato y el vencimiento. Mantener `.env` fuera de Git y evitar imprimirlo o enviarlo.

## Solicitudes y respuestas

Los siguientes ejemplos suponen `CAR_CRUDS_URL` con el origen HTTPS verificado y `AUTH_GYM_TOKEN` cargado desde un gestor de secretos en el entorno local. `employee-001` es ficticio. Comprobar su ID y estado en el entorno de ensayo antes de escribir.

```bash
curl --fail-with-body --max-time 8 \
  -H "Authorization: Bearer $AUTH_GYM_TOKEN" \
  "$CAR_CRUDS_URL/v1/employees?page=1&pageSize=1"
curl --fail-with-body --max-time 8 \
  -H "Authorization: Bearer $AUTH_GYM_TOKEN" \
  "$CAR_CRUDS_URL/v1/employees/employee-001/status"
```

```json
{"items":[{"employeeId":"employee-001","displayName":"Empleado ficticio"}],"page":1,"pageSize":1,"total":2}
```

```json
{"employeeId":"employee-001","active":true}
```

Recorrer las páginas de hasta 100 registros. Para comprobar una baja, consultar el estado del ID vinculado. La ausencia de un ID en la lista puede depender de la paginación o de cambios concurrentes. Ante `active:false`, el gimnasio decide cómo gestionar la membresía.

Guardar la fotografía de prueba en `evidencia.json` y adaptar el ID, el período, la referencia y la fecha al ensayo. Conservar ese archivo para el reenvío.

```json
{
  "employeeId": "employee-001",
  "period": "2026-10",
  "sourceReference": "gym-demo-001-2026-10",
  "version": 1,
  "observedAt": "2026-10-28T12:00:00Z",
  "member": true,
  "used": true,
  "costMinor": 1000000,
  "spendingMinor": 1000000,
  "currency": "ARS"
}
```

```bash
curl --fail-with-body --max-time 8 \
  -H "Authorization: Bearer $AUTH_GYM_TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary @evidencia.json \
  "$CAR_CRUDS_URL/v1/employees/membership-evidence"
```

La primera aplicación responde `{"applied":true}`. El reenvío idéntico responde `{"applied":false}`. Ante una corrección, mantener la referencia, aumentar la versión y enviar la fotografía completa. Los valores omitidos o `null` representan hechos desconocidos. `false` y `0` representan valores comprobados. El mes calendario y ARS en centavos son el perfil propuesto para el intercambio. Queda acordar la imputación de meses parciales y el redondeo. [Acuerdo del perfil mensual](../ACUERDO-GIMNASIO.md)

## Errores, reintentos e idempotencia

| Resultado | Acción |
| --- | --- |
| 400 / 415 | Corregir campos obligatorios, formato o `Content-Type`. |
| 401 / 403 | Revisar vigencia, valor de la credencial y permisos. |
| 404 | Revisar la ruta o el ID. En `/status`, el ID es desconocido. |
| 409 | Conciliar la referencia y la versión con el emisor. Un cambio de contenido en una versión registrada genera conflicto. |
| 503, otros 5xx transitorios o timeout | Reenviar la misma fotografía con espera exponencial y variación aleatoria, hasta tres reintentos. Si persiste, conservarla pendiente y registrar el `X-Request-ID` cuando esté disponible. |

Usar un plazo inicial de 8 segundos por solicitud y enviar un registro a la vez durante la prueba. Una respuesta perdida puede haber ocurrido después del commit. Reintentar con los mismos campos, incluido `observedAt`. PostgreSQL serializa las operaciones por empleado y período con un bloqueo transaccional y conserva una fila vigente por empleado y período y una fila histórica por versión. La escritura vigente y el historial comparten la transacción. Una versión anterior puede quedar en el historial y devuelve `applied:false` sin sustituir la vigente.

## Prueba conjunta

1. Car Cruds prepara un entorno de ensayo con datos ficticios. `SEED_DEMO=true` sirve para una base nueva de ensayo. Si el ID de ejemplo ya existe o está inactivo, elegir otro ID ficticio creado por el operador. Acordar un período sin evidencia para ese empleado.
2. Desde la red del gimnasio, cargar la credencial y ejecutar `go run ./cmd/checkshared -url "$CAR_CRUDS_URL" -contract docs/contracts/employees-openapi.json -employee employee-001`. El comando solo lee y comprueba TLS, autenticación, proyección, paginación, aislamiento administrativo, estado y coincidencia del contrato. Si el ID de prueba es otro, pasar ese ID.
3. El gimnasio consulta al empleado y lo vincula con un socio ficticio en su sistema. Envía una fotografía parcial y reenvía exactamente el mismo cuerpo. Verificar `applied:true` seguido de `applied:false`.
4. Enviar la corrección con una versión superior y reintentar la primera. El operador de Car Cruds consulta `GET /v1/employees/reports/memberships?period=2026-10` con rol `admin` o `service`. Registrar la versión vigente y comprobar que los totales por moneda reflejan una sola fotografía por empleado y período.
5. Si se incluye la prueba de bajas, el operador da de baja al empleado de ensayo y el gimnasio comprueba `active:false` mediante `/status`. Registrar fecha, revisión desplegada, códigos HTTP y correlación. Mantener las credenciales fuera de la evidencia de la prueba.

## Verificación del lado de Car Cruds

El 10/11/2026 se ejecutó `go test -race -count=1 ./...` con `TEST_DATABASE_URL` apuntando a PostgreSQL desechable y se validaron los contratos y ejemplos con `scripts/check-api-docs.py`. La prueba HTTP usa el ejemplo de este OpenAPI y verifica credenciales ausentes, inválidas y vencidas, permisos, datos obligatorios, repetición concurrente, conflicto y respuesta 503 al perder acceso a persistencia.

En una red Docker aislada, `checkshared` comprobó la ruta Caddy → gateway → Empleados por TLS con una autoridad local. En esa misma red, ocho envíos concurrentes dejaron una sola aplicación. Una corrección con versión mayor reemplazó el costo y el informe conservó un único registro para ambos importes. La prueba pública requiere el origen HTTPS definitivo y la ejecución desde el backend del gimnasio.

# Comunicación con el gimnasio de Pablo

Estado al 22 de octubre de 2026: implementado del lado de Car Cruds en `entrega-2/cierre`. La lista y la recepción están disponibles al ejecutar el conjunto propio. El gimnasio todavía debe aceptar la devolución propuesta e implementar su consumidor y productor. No se acredita una conexión real entre ambos despliegues.

## Dirección y alcance

1. **Gimnasio → Car Cruds:** consulta `GET /v1/employees?page=1&pageSize=20`, con `Authorization: Bearer <AUTH_GYM_TOKEN>`. Obtiene activos con `employeeId` estable y `displayName`; sin registros ajenos a empleados activos. `pageSize` máximo 100.
2. **Gimnasio → Car Cruds:** entrega una instantánea por empleado y período mediante `POST /v1/employees/membership-evidence`, con el mismo token y `Content-Type: application/json`. Car Cruds almacena afiliación, uso, costo y gasto informados por el gimnasio.
3. **Administración de Car Cruds:** consulta `GET /v1/employees/reports/memberships?period=2026-10&page=1&pageSize=20` con token administrativo o de servicio. Ve hechos individuales, faltantes, origen y totales por moneda. El gimnasio no puede leer este informe ni administrar empleados.

Se retiró el envío desde Car Cruds a `POST /api/v1/integraciones/empleados` del gimnasio: ese alta no pertenece al diseño original solicitado. No se necesitan `GYM_BASE_URL` ni credenciales para llamar al gimnasio. No se modificó su repositorio. El flujo del hospital se documenta por separado en [COMUNICACION-HOSPITAL](COMUNICACION-HOSPITAL.md).

## Contrato propio de devolución — pendiente de aceptación

El diseño original exige recibir evidencia y evitar duplicados, pero deja abierto el transporte. Se ofrece una entrada HTTP **de Car Cruds**; no se atribuye este endpoint al gimnasio ni se presenta como acuerdo bilateral. OpenAPI 3.0.3, versión 0.3.0 (la lista y las membresías conservan su formato y comportamiento): [employees-openapi.json](contracts/employees-openapi.json). La operación de lista mantiene su formato anterior.

Ejemplo exclusivamente ficticio de cuerpo:

```json
{
  "employeeId": "employee-001",
  "period": "2026-10",
  "sourceReference": "evidencia-gym-001-octubre",
  "version": 1,
  "observedAt": "2026-10-22T12:00:00Z",
  "member": true,
  "used": false,
  "costMinor": 250000,
  "spendingMinor": 0,
  "currency": "ARS"
}
```

- `employeeId` debe existir como empleado. Se admite evidencia tardía de inactivos para conservar gastos históricos. Un identificador que no corresponde a un empleado obtiene 404; no crea personas ni asociaciones automáticamente.
- `period` y `sourceReference` son identificadores opacos de 1–100 caracteres, sin controles ni espacios exteriores. `2026-10` es solo un ejemplo; la delimitación y vigencia del período deben acordarse con Pablo. La referencia permanece estable para ese empleado y período.
- `member`, `used`, `costMinor` y `spendingMinor` son independientes. Ausencia o `null` significa desconocido; `false` y `0` son hechos explícitos. Un afiliado puede no haber usado el gimnasio y tener costo. No se inventan tarifas, asistencia, compras, pagos ni facturas.
- Los importes son enteros no negativos en unidades menores de la moneda acordada, hasta el máximo `int64`. Se exige moneda de tres letras mayúsculas cuando hay importes. Este contrato admite una moneda por empleado y período; no convierte monedas. `costMinor` es el costo de membresía y `spendingMinor` el gasto individual del período: se muestran y suman por separado, nunca se suman uno al otro.
- `version` es entero positivo creciente por empleado/período. Se envía una instantánea completa; omitir un hecho en una corrección vuelve a dejarlo desconocido. `observedAt` es fecha RFC3339, normalizada a UTC y microsegundos; no determina precedencia. La versión mayor reemplaza a la anterior.
- El origen proviene del token (`gimnasio-pablo`), no de datos elegidos por el emisor en el cuerpo.

Respuesta 200: `{"applied":true}` cuando cambia la evidencia vigente; `{"applied":false}` cuando ya se había recibido o llega atrasada. Una versión atrasada nueva se conserva en historia y no altera el informe. Una repetición con distinto contenido, otro origen o referencia obtiene 409 y exige revisión; no se sobrescribe silenciosamente. Cada versión aceptada queda en una historia persistente. Un reintento idéntico tras un timeout no duplica gasto.

También se devuelven 400 para datos inválidos, 401 por token ausente/inválido, 403 por rol insuficiente, 415 por tipo de contenido y 503 por indisponibilidad. Ante 503 o respuesta perdida se reintenta el mismo mensaje; ante 400/404/409 se corrige o revisa la causa. No reenviar con una versión artificialmente nueva para eludir un conflicto.

## Interpretación del informe

Incluye todos los empleados registrados actualmente, incluso inactivos; no dispone de historia de períodos laborales y no certifica la población facturable. `evidence: null` indica ausencia de devolución para ese período. `missingEvidence` cuenta empleados sin instantánea; `incompleteEvidence` incluye los anteriores y aquellos con algún hecho desconocido.

Los contadores y `totals` abarcan todo el registro, independientemente de la página. Cada moneda informa costo y gasto separados y sus cantidades de registros conocidos (`costRecords`, `spendingRecords`). Sin valores conocidos, el total es `null`, nunca cero inferido. Los totales son parciales cuando faltan evidencias; no se convierten monedas. Ante desbordamiento se responde 503, sin publicar una suma incorrecta. Una única consulta SQL mantiene coherentes filas y totales de cada respuesta; páginas consultadas en distintos momentos pueden diferir.

## Qué falta para la prueba entre equipos

Pablo debe confirmar la lista mínima, el identificador compartido y este contrato de devolución; implementar en su proyecto la consulta y el envío de datos reales. Deben acordarse significado de uso, períodos, referencias, correcciones, moneda/unidad, imputación de gastos y bajas. Ambos equipos deben disponer de URL accesible y compartir el token por privado. Despliegue público y hospital quedan pendientes conforme a lo solicitado.

La prueba conjunta debe mostrar: consulta autenticada, correspondencia de un empleado, envío y reintento idéntico, corrección, evidencia faltante y lectura del informe administrativo. Las pruebas propias con datos ficticios no sustituyen esa aceptación.

## Verificación propia

`go test -race ./services/employees/...` verifica reglas, permisos y formato HTTP. Con `TEST_DATABASE_URL` configurada incluye persistencia PostgreSQL, correcciones, mensajes atrasados, historia, reinicio, monedas y reintentos concurrentes. Sin esa variable las pruebas de base se omiten explícitamente. El workflow de GitHub Actions ejecuta dichas pruebas con PostgreSQL real y rechaza omisiones en la suite del conjunto.

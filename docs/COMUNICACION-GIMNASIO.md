# Comunicación con el gimnasio de Pablo

Actualización del 23 de octubre de 2026: el grupo del gimnasio comunicó que el profesor confirmó el flujo lista → vinculación manual → devolución de evidencia. La implementación anterior está integrada a `main`; los ajustes de interoperabilidad se revisan en `entrega-2/cierre`. [ACUERDO-GIMNASIO](ACUERDO-GIMNASIO.md) responde punto por punto: identidad estable, semántica mensual propuesta, datos parciales, costos por meses iguales elegidos por Car Cruds, consulta explícita de bajas y prueba conjunta. Definiciones y acceso aún requieren confirmación del gimnasio; no se acredita conexión real entre despliegues.

## Dirección y alcance

1. **Gimnasio → Car Cruds:** consulta `GET /v1/employees?page=1&pageSize=20`, con `Authorization: Bearer <AUTH_GYM_TOKEN>`. Obtiene activos con `employeeId` estable y `displayName`; candidatos excluidos. `pageSize` máximo 100.
2. **Gimnasio → Car Cruds:** entrega una instantánea por empleado y período mediante `POST /v1/employees/membership-evidence`, con el mismo token y `Content-Type: application/json`. Car Cruds almacena afiliación, uso, costo y gasto informados por el gimnasio.
3. **Administración de Car Cruds:** consulta `GET /v1/employees/reports/memberships?period=2026-10&page=1&pageSize=20` con token administrativo o de servicio. Ve hechos individuales, faltantes, origen y totales por moneda. El gimnasio no puede leer este informe ni administrar empleados.

Se retiró el envío desde Car Cruds a `POST /api/v1/integraciones/empleados` del gimnasio: ese alta no pertenece al diseño original solicitado. No se necesitan `GYM_BASE_URL` ni credenciales para llamar al gimnasio. No se modificó su repositorio. El flujo del hospital se documenta por separado en [COMUNICACION-HOSPITAL](COMUNICACION-HOSPITAL.md).

## Contrato propio de devolución — pendiente de aceptación

El transporte de tres pasos coincide con lo confirmado por el profesor según el mensaje del gimnasio. OpenAPI 3.0.3, versión 0.3.0: [employees-openapi.json](contracts/employees-openapi.json). Agrega `GET /v1/employees/{id}/status`; la lista y el POST de evidencia 0.2.0 conservan sus campos y comportamiento. La semántica detallada propuesta está en [ACUERDO-GIMNASIO](ACUERDO-GIMNASIO.md), pendiente de confirmación bilateral.

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

- `employeeId` debe existir como empleado. Se admite evidencia tardía de inactivos para conservar gastos históricos. Un candidato o identificador desconocido obtiene 404; no crea personas ni asociaciones automáticamente.
- `period` y `sourceReference` admiten 1–100 caracteres sin controles ni espacios exteriores. El perfil propuesto para Pablo utiliza mes calendario `AAAA-MM` en `America/Argentina/Buenos_Aires`; la API genérica conserva períodos opacos por compatibilidad. La referencia permanece estable para ese empleado y período.
- `member`, `used`, `costMinor` y `spendingMinor` son independientes. Ausencia o `null` significa desconocido; `false` y `0` son hechos explícitos. Un afiliado puede no haber usado el gimnasio y tener costo. No se inventan tarifas, asistencia, compras, pagos ni facturas.
- Los importes son enteros no negativos hasta el máximo `int64`, con moneda obligatoria cuando hay importes. Para Pablo se propone ARS en centavos: `costMinor` corresponde al costo neto imputable al mes completo, por meses iguales; meses parciales quedan en `null` hasta acordar la regla. `spendingMinor` corresponde a pagos efectivos del mes. Se muestran por separado y no se suman entre sí. La API genérica conserva monedas separadas por compatibilidad.
- `version` es entero positivo creciente por empleado/período. Se envía una instantánea completa; omitir un hecho en una corrección vuelve a dejarlo desconocido. `observedAt` es fecha RFC3339, normalizada a UTC y microsegundos; no determina precedencia. La versión mayor reemplaza a la anterior.
- El origen proviene del token (`gimnasio-pablo`), no de datos elegidos por el emisor en el cuerpo.

Respuesta 200: `{"applied":true}` cuando cambia la evidencia vigente; `{"applied":false}` cuando ya se había recibido o llega atrasada. Una versión atrasada nueva se conserva en historia y no altera el informe. Una repetición con distinto contenido, otro origen o referencia obtiene 409 y exige revisión; no se sobrescribe silenciosamente. Cada versión aceptada queda en una historia persistente. Un reintento idéntico tras un timeout no duplica gasto.

También se devuelven 400 para datos inválidos, 401 por token ausente/inválido, 403 por rol insuficiente, 415 por tipo de contenido y 503 por indisponibilidad. Ante 503 o respuesta perdida se reintenta el mismo mensaje; ante 400/404/409 se corrige o revisa la causa. No reenviar con una versión artificialmente nueva para eludir un conflicto.

## Interpretación del informe

Incluye todos los empleados registrados actualmente, incluso inactivos; no dispone de historia de períodos laborales y no certifica la población facturable. `evidence: null` indica ausencia de devolución para ese período. `missingEvidence` cuenta empleados sin instantánea; `incompleteEvidence` incluye los anteriores y aquellos con algún hecho desconocido.

Los contadores y `totals` abarcan todo el registro, independientemente de la página. Cada moneda informa costo y gasto separados y sus cantidades de registros conocidos (`costRecords`, `spendingRecords`). Sin valores conocidos, el total es `null`, nunca cero inferido. Los totales son parciales cuando faltan evidencias; no se convierten monedas. Ante desbordamiento se responde 503, sin publicar una suma incorrecta. Una única consulta SQL mantiene coherentes filas y totales de cada respuesta; páginas consultadas en distintos momentos pueden diferir.

## Qué falta para la prueba entre equipos

Pablo debe confirmar las definiciones propuestas en [ACUERDO-GIMNASIO](ACUERDO-GIMNASIO.md) e implementar consulta, vinculación manual y envío. `used` se propone como ingreso efectivo, no reserva de clase; `member`, vigencia en algún tramo del mes hasta el corte. Se aceptan `null`, meses en curso y correcciones históricas. Para bajas se ofrece `/status`; no se cancela automáticamente la membresía. Falta host accesible, token por privado y prueba conjunta. El hospital tiene su guía independiente.

La prueba conjunta debe mostrar: consulta autenticada, correspondencia de un empleado, envío y reintento idéntico, corrección, evidencia faltante y lectura del informe administrativo. Las pruebas propias con datos ficticios no sustituyen esa aceptación.

## Verificación propia

`go test -race ./services/employees/...` verifica reglas, permisos y formato HTTP. Con `TEST_DATABASE_URL` configurada incluye persistencia PostgreSQL, correcciones, mensajes atrasados, historia, reinicio, monedas y reintentos concurrentes. Sin esa variable las pruebas de base se omiten explícitamente. El workflow de GitHub Actions ejecuta dichas pruebas con PostgreSQL real y rechaza omisiones en la suite del conjunto.

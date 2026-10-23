# Cierre de interoperabilidad con el gimnasio

Estado: propuesta de Car Cruds a partir del mensaje recibido del grupo de Pablo y de las elecciones del responsable del proyecto. El flujo de tres pasos fue confirmado por el profesor según lo comunicado. Las definiciones propuestas aquí todavía deben recibir confirmación del gimnasio; no se registra aceptación inexistente.

## 1. Flujo y vinculación

1. Gimnasio consulta `GET /v1/employees?page=1&pageSize=100` con Bearer de rol `gym` y recorre las páginas.
2. Su administrador vincula manualmente nuestro `employeeId` con un socio. Nunca se vincula solamente por nombre. DNI y email se completan en el gimnasio; Car Cruds no incorpora esos campos a la lista ni realiza un alta automática de socios.
3. Gimnasio envía la evidencia a `POST /v1/employees/membership-evidence`. La vinculación no activa una membresía: plan, pago y habilitación siguen bajo sus reglas.

Car Cruds garantiza por su API un `employeeId` estable: se genera al registrar, no cambia al editar el nombre, dar de baja o reactivar, y no se asigna a otra persona. La baja es lógica; la fila permanece con su clave primaria. Otra persona con el mismo nombre recibe otro ID. No se debe reconstruir la base de un entorno compartido como si fuera un laboratorio vacío durante la evaluación.

## 2. Perfil mensual propuesto

La estructura de la evidencia 0.2.0 se conserva. OpenAPI 0.3.0 agrega únicamente la lectura puntual del estado laboral; no cambia los campos del POST. Este perfil concreta el significado que proponemos para el intercambio con Pablo. La API genérica conserva períodos opacos y monedas separadas: todavía no rechaza otros períodos/monedas válidos, y no calcula ni verifica tarifas del gimnasio.

| Campo | Definición propuesta para este intercambio |
| --- | --- |
| `period` | Mes calendario `AAAA-MM`, desde el primer día a las 00:00 hasta el primer día del mes siguiente, extremo final excluido, en `America/Argentina/Buenos_Aires`. |
| `member` | Tuvo una membresía vigente en algún momento de ese mes hasta el corte informado. No es necesariamente el estado actual de afiliación. Una solicitud de socio pendiente no prueba vigencia. |
| `used` | Existe al menos un ingreso efectivo registrado dentro del período y hasta el corte. Una reserva de clase no demuestra asistencia. |
| `costMinor` | Costo neto del plan imputable al mes completo: importe neto tras descuentos dividido por los meses contratados. No es el pago recibido ese mes. |
| `spendingMinor` | Total de pagos efectivamente registrados al gimnasio para esa persona durante el mes y hasta el corte, según fecha del pago en la zona acordada. No se vuelve a repartir un pago anual entre meses. |
| `currency` | `ARS`, con importes en centavos enteros no negativos, para este perfil. |
| `observedAt` | Instante real de elaboración/comprobación de la fotografía, RFC3339 con zona; identifica el corte de la información, no el orden de correcciones. |

Ejemplo económico ficticio: un plan anual neto de **$120.000** corresponde a **$10.000 por mes completo** (`costMinor: 1000000`). Si se paga todo en octubre, `spendingMinor` de octubre es `12000000`; otro mes solo tendrá gasto cero si el gimnasio verificó que no hubo pagos. La ausencia de registros fiables significa `null`.

**Elección expresa del responsable de Car Cruds:** imputación por meses iguales, no por días de cobertura. El tratamiento de meses parciales permanece abierto. Hasta acordarlo, enviar `costMinor: null` para esos casos. Si la división deja fracciones de centavo, proponemos cociente entero en los primeros meses y residuo en el último; el gimnasio debe confirmar esta regla antes de utilizarla. Si no hay certeza del costo neto, vigencia, descuento o imputación, enviar `null`.

Reembolsos o ajustes que requieran importes negativos necesitan un acuerdo adicional: el contrato actual no los admite. No convertir un saldo negativo o no representable en cero. Car Cruds presenta costo y pagos por separado, sin sumarlos como si fueran conceptos independientes de gasto total ni generar facturas.

## 3. Información parcial y meses anteriores

Se acepta desde el inicio:

- `null` u omisión de un hecho: desconocido. Se admite una fotografía con todos los hechos desconocidos.
- `false`: negativo verificado dentro del período y hasta el corte, con registros suficientes.
- `0`: importe cero verificado, no ausencia de implementación del registro.
- Mes en curso: fotografía provisional hasta `observedAt`. Un `false` o cero de hoy puede cambiar mañana; no equivale a cerrar el mes completo.
- Meses anteriores: correcciones con la misma referencia y mayor versión, incluso si el empleado está actualmente inactivo.

Un informe con todos los campos presentes no certifica cierre temporal. `incompleteEvidence` cuenta hechos faltantes, no meses abiertos. Revisar `observedAt` y el período; Car Cruds no inventa un cierre contable ni completa datos pendientes. Una consulta fallida del productor no debe borrar hechos conocidos: conservar su fotografía pendiente y enviar una nueva solo con la evidencia que corresponda según el acuerdo.

## 4. Reintentos, versiones e historia

La interpretación del gimnasio coincide con la implementación: referencia estable por empleado/período, versión entera positiva creciente y fotografía completa. En un reintento se conservan **todos los valores, incluido `observedAt`**. No generar una nueva fecha o versión por cada intento de entrega.

- Misma versión y mismo contenido: 200 con `applied: false`.
- Misma versión y contenido distinto: 409, revisar; no cambiar la clave para eludirlo.
- Mayor versión: reemplaza la evidencia vigente; no acumula nuevamente los importes.
- Versión atrasada: se conserva sin retroceder el informe, 200 con `applied: false`.
- Cambiar referencia u origen para el mismo empleado/período: 409.

La versión gobierna la precedencia, no la fecha. Una corrección debe enviar la fotografía completa; campos omitidos pasan a desconocidos, no conservan implícitamente el valor anterior. La historia permanece en PostgreSQL.

## 5. Bajas: comprobación explícita

Se agrega `GET /v1/employees/{employeeId}/status`, con el mismo Bearer. Respuesta mínima:

```json
{"employeeId":"employee-001","active":false}
```

Permite consultar un empleado conocido aunque ya no aparezca en la lista activa. Devuelve solamente identidad y estado laboral: no expone nombre, DNI, email, candidatos, informes ni información clínica. `404` significa que el identificador no corresponde a un empleado conocido; `503`, un problema técnico. Ninguno autoriza inferir una baja ni reutilizar el ID.

El gimnasio puede comprobar periódicamente los IDs que ya tiene vinculados, además de recorrer la lista. `active:false` informa una baja laboral; **no cancela ni desvincula automáticamente al socio o su membresía**. Esas decisiones corresponden a su administración. Una ausencia en cualquier página o incluso en recorridos concurrentes no sustituye esta consulta.

## 6. Conexión y condiciones operativas

- **URL:** pendiente de un host alcanzable desde el backend del gimnasio. No se publica una URL inventada ni se presenta `localhost` de Car Cruds como acceso remoto. [DESPLIEGUE](DESPLIEGUE.md) describe la configuración HTTPS preparada; VM/dominio siguen pendientes de provisión.
- **Credencial:** `AUTH_GYM_TOKEN` del entorno compartido, rol `gym`, por canal privado. Nunca enviar `.env` completo ni subir el token al repositorio. Solo puede listar empleados, consultar su estado y enviar evidencia.
- **Paginación:** hasta 100 empleados por página; orden estable por ID, instantánea por página, no entre todas las páginas.
- **Capacidad:** el gateway admite por defecto hasta 64 solicitudes simultáneas globales, compartidas con el resto del sistema y configurables. No es una cuota de solicitudes por segundo ni una reserva para el gimnasio. Hoy no existe un límite por minuto implementado ni un SLA de disponibilidad acordado.
- **Consumo propuesto:** una solicitud a la vez para recorrer/vincular y entregar fotografías pendientes. Ante 503 o respuesta perdida, reintentar el mismo mensaje con espera creciente y variación aleatoria; tras intentos acotados conservarlo pendiente para el próximo ciclo. 400/404/409 requieren revisión; 401/403 requieren corregir credencial/permisos. No generar tráfico de carga sin coordinación.
- **Datos ficticios:** con `SEED_DEMO=true`, `employee-001` y `employee-002` son empleados ficticios; `candidate-001` queda excluido. La siembra no reactiva filas existentes. Antes de la prueba comprobar la lista/estado y, si hace falta, crear otro empleado ficticio por el rol administrativo, conservando el ID devuelto.

La disponibilidad durante la evaluación debe coordinarse y mantenerse una vez publicado el servicio. La configuración preparada no acredita que ya exista un servidor compartido.

## 7. Prueba conjunta de aceptación

1. Verificar acceso desde el backend del gimnasio: 401 sin token, 200 con su token y 403 para administración/candidatos/informes.
2. Leer el empleado ficticio y vincularlo manualmente con un socio del gimnasio, sin emparejar por nombre ni activar una membresía automáticamente.
3. Enviar una fotografía mensual parcial con campos `null`; verificar estado desconocido en el informe administrativo de Car Cruds.
4. Repetir exactamente esa versión y comprobar `applied:false`, sin doble importe.
5. Enviar una corrección completa con mayor versión; verificar reemplazo, origen, fecha y totales separados de costo/pagos.
6. Comprobar una corrección del mes anterior y un mensaje atrasado; el atraso no debe pisar la versión nueva.
7. Dar de baja al empleado ficticio desde Car Cruds, consultar `/status` desde Gimnasio y verificar que su sistema no cancele la membresía por sí solo.

Queda por confirmar con Pablo: este perfil semántico y la regla económica; meses parciales/centavos y eventuales reembolsos; host, token privado, horario y responsables de la prueba. El formato de tres pasos y la vinculación manual ya coinciden con lo comunicado por su equipo.

# Personal en el panel del operador — F3-03

Fecha: 9 de noviembre de 2026. Estado: implementado en `feature/entrega-3-pendientes`, sobre `feature/entrega-3`. Verificado con pruebas automatizadas, contra el servicio Empleados real y en navegador. La conexión con el gimnasio y con el hospital reales sigue pendiente de acuerdos externos.

## Acceso y navegación

Se usa el mismo acceso administrativo del [panel del operador](PANEL-OPERADOR.md) (`GET /v1/session` con `role=admin`). La sección **Personal** del menú lateral agrupa tres pestañas.

| Ruta web | Funcionalidad | API consumida |
| --- | --- | --- |
| `/operador/personal` | Lista paginada de altas y bajas, y consulta por ID | `GET /v1/admin/employees`, `GET /v1/employees/{id}` |
| `/operador/personal/nuevo` | Alta con nombre administrativo y estado | `POST /v1/employees` |
| `/operador/personal/:id` | Edición de nombre y estado, y baja lógica con confirmación | `PUT` y `DELETE /v1/employees/{id}` |
| `/operador/personal/membresias` | Informe por período: contadores, totales por moneda y evidencia por empleado | `GET /v1/employees/reports/memberships` |
| `/operador/personal/turnos` | Nueva solicitud de turno e informe de la última solicitud por empleado | `POST /v1/employees/clinic/patient-links`, `POST /v1/employees/clinic/requests`, `GET /v1/employees/reports/clinic` |
| `/operador/personal/turnos/:requestId` | Estado, clasificación y evidencia; envío y conciliación con el hospital | `GET`, `POST …/submit` y `POST …/refresh` |

No se agregaron endpoints: la interfaz usa los contratos [employees-openapi.json](contracts/employees-openapi.json) 0.3.0 y [clinic-integration-openapi.json](contracts/clinic-integration-openapi.json) 0.3.0. El gimnasio y el hospital nunca se llaman desde el navegador: Empleados es el único que habla con el hospital.

> **Hospital publicado — 10/11/2026:** en Turnos médicos el médico se elige del catálogo del hospital (`GET /v1/employees/clinic/doctors`); si el hospital no responde, se puede ingresar a mano. La sección **Facturación del hospital** muestra, por mes UTC, el precio vigente, el total y cada cargo (`GET /v1/employees/clinic/usage`). Una cancelación sigue cobrada.

## Garantías de la interfaz

- **Alta de empleado.** El identificador lo genera el servicio y el POST no es idempotente. Si la respuesta se pierde, la interfaz bloquea otro envío y pide revisar la lista antes de repetir.
- **Edición.** El PUT reemplaza nombre y estado y no lleva versión, así que gana el último que escribe (deuda registrada en [ARCHITECTURE](ARCHITECTURE.md#9-limitaciones-conocidas-y-deuda-técnica-aceptada)). La ficha muestra siempre la última respuesta del servidor.
- **Baja.** Exige un diálogo de confirmación. Es lógica e idempotente, y no cancela turnos ni membresías.
- **Membresías.** El período es un identificador opaco (por defecto, el mes actual de Buenos Aires). Un dato ausente se muestra como «Sin dato» o «Sin evidencia», nunca como cero. Los totales se informan por moneda, sin conversión.
- **Nueva solicitud de turno.** Vincula empleado y paciente (`204`) y guarda la intención con una referencia local estable (`turno-…`). La referencia se guarda en `sessionStorage` antes de la red, sin la credencial. Ante una respuesta perdida, repetir con la misma referencia recupera la intención original en vez de duplicarla; se comprobó contra el servicio real.
- **Fecha del turno.** Se ingresa en hora de Buenos Aires (UTC−3, sin horario de verano) y se envía en UTC con precisión de segundos, como exige el dominio.
- **Envío.** Es una acción explícita con diálogo. Cada acción hace como máximo una llamada al hospital. Se respeta `nextAttemptAt` y no se ofrece envío para solicitudes terminales o con turno conocido.
- **Conciliación.** `refresh` requiere un envío previo. Si no hay turno conocido, pide el ID de turno del hospital.
- **Resultado.** Un `200` no se presenta como turno confirmado: se muestra la clasificación (`confirmed`, `pending_verification`, `without_confirmed_appointment`).
- **Hospital no configurado.** `503 NOT_CONFIGURED` se informa como «no se envió nada» y no como resultado incierto, porque el servicio responde antes de llamar al hospital.

## Verificaciones ejecutadas

- `src/operator/Staff.test.tsx`: siete casos. Cubren las claves opacas y la conversión horaria; el alta sin reintento; la edición y la baja con diálogo (detectó y corrigió un botón que además enviaba el formulario); el informe sin convertir faltantes en cero; el flujo vínculo, intención y envío; la recuperación de una intención incierta con el mismo cuerpo tras volver a entrar; y `NOT_CONFIGURED` sin falsa incertidumbre.
- Contra el servicio real, por la entrada web: alta de empleado ficticio, vínculo `204`, intención creada, repetición con la misma referencia que devuelve el mismo `requestId`, envío con `503 NOT_CONFIGURED` (sin hospital configurado) e informes de membresías y turnos.
- Navegador real (Edge headless detrás de Caddy con HTTPS): las seis pantallas de personal, en escritorio y móvil, sin violaciones WCAG 2.1 AA de axe, sin scroll horizontal y sin errores de consola. Ver [EVIDENCIA-FRONTEND](EVIDENCIA-FRONTEND.md#6-revisión-en-navegador-real--9-de-noviembre).

## Límites

- No se envió ninguna solicitud a un hospital real: falta la URL y la clave del grupo del hospital. El recorrido completo contra el código del proveedor está cubierto por la prueba de contrato de la CI (`integration/cliniccontract`).
- No hay evidencia real del gimnasio en el informe: la recepción solo acepta el rol `gym`, y el operador no la simula.
- Los datos de prueba creados (empleado «Persona ficticia E3 UI», paciente `pac-ficticio-e3`) quedan en la base local de laboratorio. La API no ofrece borrado físico.

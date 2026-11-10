# Pendientes de cierre y aportes necesarios

Fecha de revisión: 8 de noviembre de 2026. Base técnica: `main`, `19c4e06`. El código propio y el contrato hospitalario aprobaron los ensayos locales de la [revisión](REVISION-E2-2026-11-08.md). Los puntos siguientes siguen abiertos; no se sustituyen por datos ficticios.

## 1. Bloqueos para acreditar el cierre

| Pendiente | Qué falta o debe aportarse | Responsable de coordinación | Evidencia de cierre |
| --- | --- | --- | --- |
| CI de `main` | Log interno de MongoDB del runner, diagnóstico del arranque y corrección o repetición justificada | Equipo Car Cruds | Trabajo Compose y pasos posteriores aprobados sobre la revisión corregida |
| Acceso del gimnasio | Entorno accesible de Car Cruds, URL estable, disponibilidad durante evaluación y entrega privada de su token | Car Cruds; infraestructura/cuenta y acceso del equipo | `checkshared` desde fuera del servidor y consumo real del gimnasio |
| Contrato y población del gimnasio | Aceptación de employees 0.4.0 (incluida la consulta `/status`), identificadores, empleados autorizados, semántica de período/moneda/correcciones y mecanismo de devolución | Car Cruds y grupo de Pablo | Lista consumida y evidencia enviada por el gimnasio; informe sin duplicaciones con totales acordados |
| Conexión al hospital | URL HTTPS que incluya `/v1`, clave de consumidor Car Cruds y condiciones de uso; no publicar secretos | Grupo de Octi y Car Cruds | Configuración privada y ensayo desde Empleados al entorno compartido |
| Datos de turnos | Procedimiento de alta/habilitación y `patientId`, `doctorId`, horarios controlados, pertenencia de turnos antiguos y condiciones económicas aceptadas | Hospital y responsable de personal | Vínculo, solicitud, repetición idempotente, consulta e informe con datos autorizados |
| Aceptación del flujo | Revisión del equipo y confirmación docente de la relevancia del flujo administrativo | Equipo y cátedra | Acuerdo registrado sin atribuir aprobaciones por la mera existencia de código |

No hay evidencia de una VM disponible ni de acceso a una cuenta cloud en esta revisión; no se contrató infraestructura. El contrato oficial del hospital **ya existe**, el adaptador **ya está implementado** y la prueba aislada **ya aprobó**. Lo pendiente es la operación conjunta, no volver a desarrollar esas piezas.

## 2. Funcionalidad que depende de otro equipo

La indicación proveniente del hospital de qué empleado necesita consulta (RF-19/HU-23, EXT-15) requiere que el hospital publique una capacidad o acuerde un mecanismo con identidad, vigencia y correcciones. Turnos para socios 1.0.0 no ofrece esa indicación. Actualmente se puede iniciar la necesidad mediante una decisión administrativa local; no se implementa una consulta externa inexistente.

El gimnasio debe aceptar y producir la devolución de membresías, uso y gastos. Car Cruds ya dispone de recepción, deduplicación, historia e informe propios; esos componentes no permiten inventar tarifas, asistencia ni gastos sin la fuente externa.

## 3. Presentación final y ampliaciones operativas

El frontend completo y la publicación cloud con URL pública pertenecen al listado de presentación grupal del enunciado. Continúan pendientes de implementación/despliegue; no se consideran realizados mediante Swagger. El acceso del consumidor a la primera capacidad operativa sí debe resolverse conforme a 4.1.

También siguen pendientes tablero y alertas, propagación de trazas por mensajería, retención/respaldos, ampliación de carga y comparación de caché fría/sin dependencia, medición de frescura desde commit, ensayos externos y validación de escalado. Son trabajo técnico adicional; no todos dependen de aportes externos y no todos son exigencias específicas de 6.3. No se implementaron durante esta consolidación documental.

## 4. Incidencia local de Windows

Windows reserva el intervalo TCP 58975–59074 y bloquea el puerto estándar 58983 de Solr en esta computadora. La revisión usó una superposición local en 18983, sin cambiar Compose versionado. Las instrucciones reproducibles están en [la revisión](REVISION-E2-2026-11-08.md#reproducción-local). Esto es independiente del fallo de MongoDB en GitHub Actions.

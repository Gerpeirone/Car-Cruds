# ADR-018 — Empleados sin candidatos

> **Complemento posterior del 23/10/2026:** [ADR-019](ADR-019-consumo-turnos-socios.md) adapta el consumo a Turnos para socios 1.0.0 y reemplaza D9 anterior. Mantiene la conservación histórica y exige empleado activo también para reintentar un envío incierto que pueda crear un turno. Las afirmaciones siguientes sobre la migración pendiente corresponden al corte original de este ADR.

| Campo | Valor |
| --- | --- |
| Decisiones afectadas | D1: límites; D3: persistencia; revisión de alcance de D5, D8 y D9 |
| Fecha | 23 de octubre de 2026 |
| Estado | Cambio implementado en rama de propuesta; pendiente de revisión del equipo |
| Sustituye en el alcance de personal | [ADR-014](ADR-014-limites-servicios-v2.md) y [ADR-015](ADR-015-persistencia-v2.md) |
| Complementa | [ADR-016](ADR-016-comunicacion-v2.md), [ADR-017](ADR-017-contrato-empleados.md) y [ADR-009](ADR-009-consumo-clinica.md) |

## 1. Contexto y justificación

La gestión de candidatos era una ampliación interna para posibles incorporaciones y revisiones previas al alta laboral. El alcance actual necesita turnos para empleados registrados y activos; mantener una población adicional introduce CRUD, permisos y una transición candidato–empleado que no forma parte del flujo requerido. Además, un vínculo clínico histórico no puede reasignarse silenciosamente al crear un empleado nuevo.

Se simplifica el modelo activo conservando la gestión administrativa del personal, membresías y consultas médicas. La clínica sigue trabajando con pacientes y el gimnasio con empleados activos; ninguno debe incorporar nuestro proceso de selección. El control laboral pertenece a Empleados y no se delega en la credencial del hospital.

## 2. Decisión de dominio y límites — D1

Se retira candidatos del dominio, los casos de uso, los puertos y el CRUD de Empleados. Las rutas `/v1/candidates` y `/v1/candidates/{id}` dejan de estar publicadas y responden 404 con autenticación válida; el middleware conserva 401 sin autenticación. No se trasladan a Clientes ni se crea otro servicio.

Los cuatro propietarios permanecen: Clientes administra conductores; Flota, vehículos y tarifas; Alquileres, agenda y ciclo del alquiler; Empleados, empleados, evidencia de membresías y seguimiento administrativo de turnos. Alquileres continúa independiente de gimnasio y hospital. Se conserva la arquitectura hexagonal y las tecnologías existentes.

El alta laboral exige una acción administrativa autorizada y un estado `active` explícito. Nuevos vínculos con paciente, nuevas intenciones clínicas y el reclamo del primer envío requieren empleado existente y activo. Un empleado inexistente devuelve 404; uno inactivo, `409 EMPLOYEE_INACTIVE`. La elegibilidad y la escritura local se coordinan en la misma transacción, sin mantenerla durante la llamada HTTP.

Una baja posterior conserva registros y evidencia. La consulta de solicitudes, el informe y `refresh` continúan disponibles para empleados inactivos; no se cancelan turnos ni membresías automáticamente. Repetir una intención ya registrada o un envío ya reclamado recupera su resultado sin crear una nueva solicitud.

## 3. Persistencia y conservación — D3

Empleados mantiene PostgreSQL propio, con credenciales separadas y sin acceso a bases ajenas. Clientes y Alquileres conservan PostgreSQL; Flota, MongoDB; Redis y Solr continúan como derivados. No cambia la elección de motores de ADR-015.

Se conservan las migraciones históricas, la tabla `candidates`, sus filas y los vínculos/solicitudes previos. No se borran ni se convierten automáticamente en empleados. La API activa filtra esos registros y no permite consultar, enviar o actualizar solicitudes de candidatos, incluso si se conoce su identificador. Se elimina su siembra de demostración; la siembra nueva solo crea empleados y no modifica filas existentes.

La conservación evita romper bases ya migradas y perder trazabilidad. Como consecuencia, una identidad clínica ya vinculada a un candidato histórico continúa ocupada: cualquier migración o reasignación posterior requerirá una decisión explícita y verificable. Esta tarea no incluye esa operación.

## 4. Contratos y comunicación — D5, D8 y D9

- El contrato propio de personal/gimnasio pasa a versión **0.3.0**, retirando esquemas y operaciones de candidatos. Lista mínima, autenticación, membresías e informe económico conservan su comportamiento.
- El contrato administrativo clínico pasa a **0.2.0**. Mantiene los campos `personKind`/`personId` por compatibilidad del formato; `employee` es el único tipo válido. El informe admite omitir el selector o enviar `personKind=employee`; rechaza `candidate`.
- No cambian transportes, eventos, credenciales ni operaciones del proveedor. La llamada clínica sigue saliendo directamente de Empleados.
- El cliente y la prueba de contrato conservan la API antigua observada en `5e4c2103e8c3b0ffaccd1e02a94d1dcf98f2f96d`. El hospital publicó posteriormente Turnos para socios 1.0.0 en `c0b65f45867f884e0bdc8f7c23eb0f344f646bb2`; su migración con autenticación, idempotencia y recuperación queda para la siguiente etapa. Esta revisión no acredita compatibilidad con ese contrato nuevo.

Se retiran **HU-10 y RF-10** sin reutilizar identificadores ni renumerar los siguientes. RN-13 pasa a alta laboral y elegibilidad activa. Se actualizan HU-11/HU-15/HU-17, RF-11/RF-15/RF-17, contratos, diagramas y guías. Los ADR anteriores y las evidencias conservan su cuerpo histórico con una nota de vigencia.

## 5. Alternativas y consecuencias

| Alternativa | Evaluación |
| --- | --- |
| Mantener candidatos y completar la transición laboral | Requiere un proceso adicional de identidad, historial y permisos sin necesidad actual |
| Convertir o borrar los datos previos al retirar la API | Puede perder evidencia o atribuir vínculos/turnos a otra persona; no se adopta |
| Limitar el dominio activo a empleados y conservar historia | Reduce superficie y reglas, mantiene los beneficios y preserva trazabilidad; exige mantener almacenamiento histórico fuera de la API |

La retirada del CRUD y de `candidate` es incompatible para consumidores administrativos anteriores y se refleja en las versiones propias. El consumidor del gimnasio mantiene la misma lista. El empleado no obtiene por este cambio una cuenta personal en el portal: Administración continúa solicitando desde nuestro backend.

## 6. Validación y límites de esta revisión

Verificación local comunicada el 23 de octubre: Go 1.26.4 en Windows, sin detector de carreras por ausencia de GCC. `go test -count=1 -json -timeout 120s ./...` finalizó con salida 0: **107 pruebas/subpruebas aprobadas, 20 omitidas y ninguna fallida**. Las omisiones corresponden a integración sin variables `TEST_*` o entorno del proveedor. `go vet ./...` terminó con salida 0. El mock histórico también aprobó sus pruebas y análisis estático. La comprobación de contratos resolvió 107 referencias y tres ejemplos.

Estos resultados verifican los casos ejecutados localmente; no acreditan persistencia, concurrencia SQL, recorrido Compose ni compatibilidad nueva con la clínica. La CI existente con detector de carreras, dependencias reales, smoke y contrato antiguo del hospital debe ejecutarse sobre la rama de propuesta; su resultado se registrará en el pull request. Las evidencias del 13/14/22 de octubre conservan sus revisiones y no acreditan automáticamente este cambio.

## 7. Referencias

[SPEC](../../SPEC.md), [BACKLOG](../BACKLOG.md), [REQUERIMIENTOS](../REQUERIMIENTOS.md), [ARCHITECTURE](../ARCHITECTURE.md), [servicio Empleados](../../services/employees/README.md) y [guía del hospital](../COMUNICACION-HOSPITAL.md). Material consultado en `clases.zip`: teoría 02 (propiedad y conservación de datos), teoría 08 (modelo, límites y puertos/adaptadores) e índice de prácticas (Repository e interfaces).

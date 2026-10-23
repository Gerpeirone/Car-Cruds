# ADR-009 — Consumo de la capacidad de Clínica

## Implementación del 22 de octubre de 2026

Se implementa el consumo de Turnos observado en `feature/entrega-2` del hospital de Octi, commit `5e4c2103e8c3b0ffaccd1e02a94d1dcf98f2f96d`. Su `main` conserva una revisión anterior. Se mantiene la decisión original: Empleados solicita turnos directamente al proveedor y presenta evidencia administrativa; no se sustituye por Notificaciones ni se modifica el otro repositorio.

El adaptador consume POST/GET de `appointments`, con timeout, límite de concurrencia, trazas y redirecciones deshabilitadas. La identidad paciente/persona se vincula mediante una acción administrativa explícita. PostgreSQL conserva intención y marca de envío antes de la llamada HTTP, sin mantener transacciones durante la red. Dado que el proveedor no ofrece idempotencia de creación, una intención se envía como máximo una vez y una respuesta perdida se concilia por ID externo verificado. Estado desconocido no equivale a ausencia de turno. Se preservan historia, versiones, cancelaciones y evidencia previa ante fallos; los informes permanecen privados y separados de candidatos/empleados.

[COMUNICACION-HOSPITAL](../COMUNICACION-HOSPITAL.md) documenta rutas, operación, pruebas y límites. La especificación observada es del consumidor y no reemplaza un contrato formal aprobado por Octi. URL, autenticación externa, pacientes reales, indicaciones del hospital y reprogramación siguen pendientes de capacidades/acuerdos del proveedor. Las secciones siguientes conservan el contexto histórico.

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D9: consumo del proveedor |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Puertos implementados; consumo externo pendiente de contrato |
| Hito | Entrega 2 — 23 de octubre de 2026 |

## 1. Contexto

La Clínica es el proveedor asignado para el flujo administrativo de candidatos y empleados. Se necesita utilizar su capacidad de revisiones o turnos y producir un informe con/sin turno confirmado. Su contrato, credenciales, identidad de personas y estados todavía no se han recibido ni acordado.

## 2. Decisión

Empleados coordina la integración mediante un puerto de salida formulado en términos del caso de uso. Un adaptador traduce el contrato clínico al modelo administrativo propio, con verificación de respuestas, errores y correlación. La solicitud se origina en Empleados directamente hacia Clínica.

La integración tiene una consecuencia de negocio: la evidencia clínica recibida alimenta un informe administrativo sobre turnos de personal. No se limita a una llamada de prueba. Se conserva la referencia y evidencia suficiente para interpretar fecha/período, origen y vigencia, con campos concretos por definir.

| Evidencia administrativa | Interpretación prevista |
| --- | --- |
| Confirmación suficiente según contrato | Incluir en el informe con turno confirmado |
| Respuesta suficiente que permite establecer ausencia de confirmación | Incluir sin turno confirmado, conservando el alcance de la consulta |
| Falta de respuesta, error, identidad no vinculada o datos insuficientes | Mostrar pendiente o indeterminado; no convertirlo en ausencia confirmada |

La confirmación de turno no significa revisión realizada, aptitud, diagnóstico ni contratación. Empleados no almacena diagnósticos ni decide contratación automáticamente. El empleado/candidato se modela separado del cliente conductor.

### Acuerdos requeridos

Acordar operaciones disponibles, formato/protocolo, esquema procesable y versión; vinculación de identidad; autenticación y permisos; estados interpretables; períodos, vigencia y errores; idempotencia o consulta de estado para comandos; límites de uso y acceso al entorno. No se inventan endpoints, campos o garantías del proveedor.

Para solicitudes con efecto, registrar una intención local durable antes de enviar la solicitud a Clínica. Si ese registro falla, no se envía la solicitud. Una consulta sin efecto no requiere ese paso.

Ante un timeout de una operación con efecto posible, conservar la referencia local de la intención y presentar resultado desconocido. La recuperación se realizará mediante el mecanismo que ofrezca el contrato; sin esa garantía, requiere revisión y no reenvío ciego.

## 3. Alternativas consideradas

Consumir desde frontend expone acoplamiento y elude el servicio dueño. Consumir a través de nuestro gateway confunde entrada pública con dependencia saliente. Colocar Clínica en Alquileres introduce reglas administrativas ajenas. Importar modelos del proveedor al núcleo dificulta cambiar contrato o traducir estados.

## 4. Consecuencias y validación

Se agregan puerto, adaptador, trazabilidad y estados administrativos honestos. La preparación documental no demuestra integración. Una vez publicado el contrato, crear un test de contrato que detecte incompatibilidades; unitarios de interpretación con falsos; integración con el entorno proveedor y ensayo de caída/lentitud. Comprobar que la falla clínica no impida reservas de autos ni consultas administrativas ya disponibles.

## Avance de implementación — 14 de octubre de 2026

Empleados define puertos de Clínica y membresías en términos propios, sin endpoints ni adaptadores HTTP externos inventados. El endpoint de estado devuelve 503 NOT_CONFIGURED a identidades autorizadas. No registra turnos ficticios, ausencia confirmada ni costos cero. RF-15 a RF-17/RF-19 y RN-17 a RN-22 mantienen sus obligaciones; contrato, persistencia de intenciones externas y pruebas reales del proveedor continúan pendientes.

## 5. Referencias e historial

Material en `clases.zip`: `teoria/08-estilos-arquitectura-backend.md` y `teoria/07-resiliencia.md`. Enunciado, integración entre grupos. [D1](ADR-014-limites-servicios-v2.md), [D2](ADR-002-arquitectura-interna.md), [D10](ADR-010-resiliencia.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera propuesta de consumo clínico desde Empleados |
| 0.3 | 14 de octubre de 2026 | Registra implementación propia, evidencia local y pendientes externos |


## Revisión de alcance — 11 de octubre de 2026

Se amplía la propuesta sin cambiar las fronteras ni las tecnologías: Gimnasio corresponde al grupo de Pablo y Clínica al hospital de Octi. El gimnasio consume empleados y administra membresías/costos; Empleados incorporará los datos económicos individuales y totales por período, con deduplicación, correcciones y separación por moneda. Uso y gasto se tratan como evidencias distintas.

El flujo del hospital comprende empleados registrados, además del flujo previo de candidatos: información de quién necesita consulta, solicitud de turno desde Empleados y seguimiento de modificaciones/cancelaciones/reprogramaciones administradas por el proveedor. Persistirá la relación entre empleado, necesidad, intención local, turno externo y evidencia de estado, sin interpretar un timeout como ausencia ni repetir comandos sin recuperación acordada.

El alcance de membresías/costos y turnos continúa sin implementar ni acordar con los equipos externos. No se modifica el borrador de lista para inventar contratos económicos o médicos. Consultar INTEGRACIONES, RF-19/HU-23 y RN-21/RN-22 para los acuerdos y verificaciones pendientes. La revisión anterior del registro se conserva en Git.

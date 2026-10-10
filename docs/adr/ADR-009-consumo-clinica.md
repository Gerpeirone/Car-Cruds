# ADR-009 — Consumo de la capacidad de Clínica

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D9: consumo del proveedor |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Hito | Preparación documental de la Entrega 2 — 23 de octubre de 2026 |

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

## 5. Referencias e historial

Material en `clases.zip`: `teoria/08-estilos-arquitectura-backend.md` y `teoria/07-resiliencia.md`. Enunciado, integración entre grupos. [D1](ADR-014-limites-servicios-v2.md), [D2](ADR-002-arquitectura-interna.md), [D10](ADR-010-resiliencia.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera propuesta de consumo clínico desde Empleados |

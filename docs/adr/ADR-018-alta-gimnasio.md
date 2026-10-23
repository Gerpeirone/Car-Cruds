# ADR-018 — Consumo del alta de empleados del gimnasio

| Campo | Valor |
| --- | --- |
| Fecha | 22 de octubre de 2026 |
| Decisiones | Ampliación D9; consecuencias D3/D4/D5/D10 |
| Estado | Implementado en Car Cruds; configuración y validación conjunta pendientes |

## Contexto

El repositorio del gimnasio aportado por el usuario publica el contrato 1.0.1 para recibir empleados, en lugar de consumir nuestra lista mínima. El proveedor requiere DNI y email que no forman parte de esa lista; un alta solo crea socio y solicitud pendiente, sin activar membresía. No hay contrato para uso/gastos ni credenciales/URL acordadas. El hospital conserva su diseño independiente en ADR-009.

## Decisión

Empleados implementa un puerto de alta y un adaptador HTTP saliente hacia el gateway del gimnasio. La operación propia es administrativa y explícita: no se envía todo el personal automáticamente. Se solicitan DNI/email para ese trámite y se guardan privadamente junto con el nombre/identificador congelados del empleado activo. No se amplía la proyección pública ni se envían candidatos.

Antes del envío se confirma la intención en PostgreSQL. Claves locales por aplicación y huellas impiden cambiar datos de un replay. Una única intención pendiente/desconocida/aceptada por empleado evita reintentos con claves nuevas que dupliquen efectos. Un lease recuperable de 15 segundos limita envíos concurrentes; su token protege contra finalizaciones atrasadas. La llamada HTTP de hasta 2 segundos no retiene conexiones de base de datos y no sigue redirecciones. La respuesta se persiste incluso si se desconectó el cliente, con un presupuesto separado de 2 segundos.

La clave externa deriva del identificador durable de la intención y se conserva al reintentar. Un 201 válido confirma únicamente la solicitud pendiente y membresía deshabilitada. Un rechazo de negocio procesable queda terminal; fallas técnicas/autenticación externa/respuestas inválidas quedan UNKNOWN, sin afirmar que el proveedor no registró el alta. No se registra el cuerpo externo ni datos personales; se exponen solo códigos controlados y referencias. URL/API key se configuran fuera de Git; sin ambas no hay envíos.

## Alternativas y consecuencias

Enviar sin persistir la intención pierde la clave ante una caída. Reintentar con claves nuevas puede crear conflictos o duplicados. Mantener una transacción abierta durante HTTP retiene conexiones. Usar únicamente memoria no sobrevive al reinicio. Agregar DNI/email a la lista compartida revela datos que su contrato no contempla. Se elige un trámite privado separado y recuperable.

Se acepta conservar datos personales en PostgreSQL para recuperación e idempotencia; requiere política de retención y operación antes del uso real. La historia confirmada de alta no es una consulta vigente de membresía. La pérdida de la base o el cambio de instalación/identidad del proveedor requiere conciliación; una clave solo garantiza idempotencia dentro del origen que reconoce el gimnasio.

## Validación

Pruebas de consumidor contra copia del contrato publicado y servidor simulado; regresiones PostgreSQL/HTTP con intención durable previa, pérdida de respuesta, recreación del servicio, concurrencia, leases y permisos. No se acredita acceso al gimnasio real hasta ejecutar una prueba conjunta con su configuración. Ver [guía operativa](../INTEGRACION-GIMNASIO.md).

Referencias de cátedra en `clases.zip`: `teoria/08-estilos-arquitectura-backend.md`, `teoria/07-resiliencia.md` y `teoria/02-datos-en-microservicios.md`. La implementación conserva el núcleo sin HTTP/SQL y transacciones locales, conforme a esos criterios.

# ADR-008 — Contrato de disponibilidad y reservas

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D8: contrato propio |
| Versión | 0.1 |
| Fecha | 7 de octubre de 2026 |
| Estado | Propuesto |
| Hito | Entrega 1 — 9 de octubre de 2026 |

## 1. Contexto

Otro grupo deberá incorporar una capacidad de Car Cruds en un flujo de negocio. La integración requiere un contrato formal y procesable, documentación de errores, versión y ejemplos suficientes para trabajar de manera autónoma.

La capacidad seleccionada debe producir consecuencias sobre el negocio del consumidor. Una consulta de catálogo puede facilitar una selección, pero la gestión de reservas ofrece un resultado operativo concreto: asignar un auto para un período.

## 2. Decisión

Se propone publicar **disponibilidad, precio y gestión de una reserva de auto** mediante API HTTP, descrita con OpenAPI 3.0.3 en `docs/contracts/openapi-v1.json`. La versión inicial de API es `1.0.0` y utiliza rutas `/v1`.

| Operación | Propósito |
| --- | --- |
| `GET /v1/availability` | Consultar disponibilidad y precio actual para un auto y período |
| `POST /v1/reservations` | Confirmar una reserva con importe esperado e idempotencia |
| `GET /v1/reservations/{id}` | Recuperar estado y condiciones de una reserva |
| `DELETE /v1/reservations/{id}` | Cancelar una reserva antes del retiro |

La consulta es informativa y no retiene el auto ni congela su precio. La confirmación revalida importe, cliente y agenda y conserva las condiciones aceptadas. Los importes se expresan en centavos enteros ARS. Las fechas corresponden a días calendario con fin exclusivo. Los rechazos distinguen conflicto de precio, solapamiento y conductor no habilitado.

La capacidad utiliza autenticación de aplicación mediante bearer token. La publicación real utilizará HTTPS y credenciales externas al repositorio que delimiten el acceso por consumidor. El mock local utiliza un token ficticio documentado.

El mock HTTP conserva datos de prueba y reservas en memoria. Permite probar conflictos, consulta, cancelación e idempotencia. La capacidad real se implementará mediante los servicios y la persistencia previstos en la arquitectura.

### Compatibilidad y publicación

El contrato, los ejemplos y sus cambios se versionan en Git. Los cambios aditivos compatibles mantienen `/v1` y aumentan la versión menor o de corrección según corresponda. El consumidor debe tolerar campos nuevos de respuesta que preserven los existentes.

Los cambios de semántica, campos obligatorios de entrada, tipos o estados requieren revisión de compatibilidad. Cuando sean incompatibles, se publicará una versión mayor con una ruta nueva. La transición se acordará con el consumidor y conservará el contrato anterior.

Antes de la publicación operativa se validarán el contrato y el comportamiento del servidor. El consumidor deberá incorporar un test de contrato que detecte cambios incompatibles. La capacidad permanecerá accesible hasta finalizar la evaluación.

## 3. Alternativas consideradas

| Alternativa | Evaluación |
| --- | --- |
| Publicar solo catálogo | Aporta lectura, pero puede quedar como integración sin consecuencias de negocio |
| Publicar solo eventos | Dificulta una solicitud inmediata de reserva y requiere acceso al broker entre grupos |
| Contrato informal en texto | Limita la validación procesable y la integración autónoma |
| Incluir cobro externo en la capacidad inicial | Amplía el alcance y la integración más allá de la gestión de reservas prevista |

## 4. Justificación y consecuencias

La API permite incorporar un auto reservado a una contratación del consumidor. Este debe gestionar cambios de precio, falta de disponibilidad y reintentos. El identificador de cliente referencia un perfil existente en Car Cruds; el mecanismo de alta o vinculación externa debe definirse antes de la integración real.

El contrato publica los datos necesarios de la reserva y excluye los datos personales del conductor de las respuestas. La confirmación conserva el precio leído al reservar y utiliza idempotencia para recuperar resultados sin duplicar operaciones.

## 5. Validación prevista

- Verificar estructura OpenAPI, ejemplos, fechas, tipos y respuestas.
- Demostrar consulta, reserva, repetición idempotente, conflicto, consulta de estado y cancelación.
- Comprobar que otro equipo pueda ejecutar los escenarios utilizando exclusivamente `docs/contracts/README.md`.
- Incorporar pruebas de compatibilidad antes de la publicación operativa.

La versión 0.1 incluye ejecución local del mock. La publicación de la implementación real requiere el entorno y la URL definidos para la integración.

## 6. Aspectos por resolver

- Acuerdo del contrato con el consumidor asignado por la cátedra.
- Alta o vinculación de clientes del consumidor y delimitación de permisos.
- Credenciales reales, URL pública y entorno operativo.
- Pruebas de contrato y procedimiento de transición entre versiones.

## 7. Historial

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.1 | 7 de octubre de 2026 | Selección de capacidad y políticas iniciales de contrato y publicación |

Una decisión sustitutiva deberá identificar este registro como reemplazado y conservar su historia.

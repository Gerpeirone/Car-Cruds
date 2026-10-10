# Capacidad propuesta de Empleados para el gimnasio

| Campo | Valor |
| --- | --- |
| Fecha | 10 de octubre de 2026 |
| Estado | Borrador propio; no acordado, implementado ni publicado |
| Contrato procesable propuesto | [employees-openapi-draft.json](employees-openapi-draft.json) |
| Versión del borrador | 0.1.0; independiente de la API histórica de reservas |

## 1. Alcance comunicado y propuesta

El gimnasio consumirá una lista de empleados de Car Cruds y devolverá información de uso de membresías. La lectura permite vincular esos beneficios a empleados de la empresa; Empleados utilizará la devolución para el informe administrativo.

Se propone una operación `GET /v1/employees`, con autenticación de aplicación y paginación. **La ruta, el formato, los campos, la paginación y la selección de población son propuestas de Car Cruds, no acuerdos externos.** No existe todavía un servidor ni URL pública para esta operación.

El borrador propone identificadores opacos `employeeId` y un nombre administrativo `displayName`, excluye candidatos y devuelve únicamente registros autorizados para esta integración. La selección de activos/inactivos y el alcance del nombre mostrado deben revisarse con el gimnasio. La relación con sus identificadores externos también está pendiente.

## 2. Condiciones propuestas

- Gateway valida identidad y límites de la aplicación; Empleados controla permiso de lectura y población autorizada.
- Comunicación pública mediante HTTPS; credenciales reales suministradas fuera del repositorio. El token de demostración del mock de reservas no es una credencial del gimnasio.
- Propuesta de paginación por `page` y `pageSize`, con máximo inicial de 100 registros por página. Los valores requieren acuerdo y validación; el borrador no garantiza una fotografía consistente mientras cambian datos entre páginas.
- Respuesta con `items`, `page`, `pageSize` y `total`. No contiene datos de clientes, médicos, contraseñas ni tarifas.
- Errores propios propuestos para entrada inválida, ausencia de autenticación, falta de permiso o indisponibilidad. No se atribuyen estos códigos a la clínica o al gimnasio.

La definición de una lista completa, snapshots o sincronización incremental deberá resolverse antes de que el consumidor dependa de su exhaustividad. Mientras se acuerda esa semántica, el borrador no garantiza que una persona ausente en una respuesta deba darse de baja en el gimnasio.

## 3. Devolución del uso

No hay operación de devolución definida en este borrador. El mecanismo, identificadores, período, significado de uso/no uso, deduplicación y precedencia de correcciones se acordarán en EXT-02 a EXT-05 de [INTEGRACIONES.md](../INTEGRACIONES.md). La falta de devolución se registra como información pendiente.

Tampoco hay precios o una fórmula de gastos acordados. El informe de uso precede a cualquier cálculo monetario.

## 4. Publicación y compatibilidad previstas

Antes de implementar y publicar: revisar el contrato conjuntamente, cerrar población/campos/errores, incorporar ejemplos acordados, definir entorno y credenciales, probar paginación y autorización y establecer el test de contrato del consumidor. Se propone conservar versiones incompatibles durante la transición acordada.

El contrato de reservas [openapi-v1.json](openapi-v1.json) sigue describiendo exclusivamente el mock de la entrega 1. No se cambia su significado ni se utiliza para representar empleados.

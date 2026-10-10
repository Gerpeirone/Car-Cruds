# Consumo de la clínica — acuerdos pendientes

| Campo | Valor |
| --- | --- |
| Fecha | 10 de octubre de 2026 |
| Estado | Capacidad identificada; contrato externo no recibido |
| Consumidor interno | Servicio Empleados |

La capacidad comunicada es el servicio de turnos para revisiones de personas consideradas para una incorporación. Todavía no se dispone de su formato procesable, versión, URLs, operaciones, autenticación, datos obligatorios, errores ni política de idempotencia.

No se incluye una especificación OpenAPI ficticia del proveedor. Una vez recibido su contrato se conservará aquí la referencia o copia autorizada, la versión consumida y el mapeo al modelo administrativo de Car Cruds.

## 1. Puerto interno propuesto

Empleados necesita solicitar una revisión y conocer si existe un turno confirmado para una intención/persona. Son necesidades del negocio local; no afirman que la clínica publique operaciones con esos nombres. El adaptador traduce el contrato real y conserva referencia externa, estado administrativo, fecha y correlación cuando estén disponibles.

El responsable puede ver un resultado confirmado, ausencia confirmada de turno o información pendiente de verificar. Una reserva de turno no acredita asistencia ni aptitud médica. La forma de registrar candidatos y su posterior alta como empleados pertenece a Car Cruds.

## 2. Condiciones que bloquean el adaptador real

- EXT-07: contrato/versiones y operaciones efectivamente disponibles.
- EXT-08: reserva directa frente a pasos completados por la persona.
- EXT-09: consulta de estado, cancelación/reprogramación e idempotencia.
- EXT-02 y EXT-10 a EXT-13: identidad, datos mínimos, permisos, entorno, límites y escenarios de prueba.

Los acuerdos se siguen en [INTEGRACIONES.md](../INTEGRACIONES.md). Un mock local futuro deberá declararse simulación y representar solo lo respaldado por un contrato; no prueba que la integración externa funcione.

## 3. Verificación prevista

Test de contrato sobre operaciones consumidas, solicitudes repetidas, respuestas tardías, cambios incompatibles y pérdida de respuesta después de confirmar. Se verificará que el informe conserve incertidumbre y que la falla externa no afecte alquileres.

No se ejecutaron estas verificaciones ni se recibieron tarifas o resultados clínicos. Ver las decisiones [D9](../adr/ADR-009-consumo-clinica.md) y [D10](../adr/ADR-010-resiliencia.md).

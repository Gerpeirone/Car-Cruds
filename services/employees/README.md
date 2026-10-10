# Servicio Empleados

| Campo | Valor |
| --- | --- |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Implementación | Documentación del componente; sin servicio ejecutable |

## Responsabilidad

Administrar candidatos y empleados de la concesionaria; publicar la lista de personal para el Gimnasio; conservar evidencia administrativa de uso/no uso por período y origen; consumir directamente la capacidad de Clínica para revisiones o turnos y preparar el informe con/sin turno confirmado.

Empleado/candidato y cliente conductor son modelos separados. Este servicio no modifica reservas de autos ni determina habilitación de conducción. La confirmación de un turno no determina diagnóstico, aptitud o contratación automática. No se almacenan diagnósticos.

## Datos y límites

Se propone PostgreSQL propio para personal, referencias externas, evidencia por período y origen y estado de integración. Los campos, vinculación de identidad, períodos, vigencia y retención deben acordarse; no constituyen un contrato externo aprobado. No accede a bases de otros servicios.

Un informe clasifica uso/no uso o confirmación/ausencia de confirmación solo con evidencia suficiente para su período. Datos ausentes, error o resultado desconocido permanecen pendientes o indeterminados. Los criterios económicos de beneficios no se calculan sin acuerdo; no se inventan tarifas ni descuentos.

## Integraciones previstas

| Relación | Dirección y estado |
| --- | --- |
| Gimnasio consulta lista | Gimnasio → gateway → Empleados; contrato, protocolo, permisos y paginación por acordar |
| Gimnasio informa uso/no uso | Recepción en Empleados mediante mecanismo por acordar; no se presupone RabbitMQ compartido |
| Clínica ofrece revisiones/turnos | Empleados → adaptador → Clínica; la llamada no sale de frontend ni pasa por el gateway propio |
| Interfaz administrativa | Web → gateway → Empleados; autorización del recurso en el servicio |

## Arquitectura interna prevista

Hexagonal: reglas y casos de uso expresan puertos de entrada y de salida, con Repository para persistencia y Adapter para Clínica. El núcleo no conoce HTTP, SQL, drivers ni modelos del proveedor. Se propone ensamblado por inyección explícita de dependencias.

La estructura futura separará dominio, aplicación, puertos y adaptadores. Esta documentación no crea ni acredita esas capas de código. El contrato clínico se traducirá al modelo propio y se validará con test de contrato e integración real.

Timeout, Circuit Breaker y límites de concurrencia contendrán fallas clínicas. No habrá reintentos automáticos de comandos sin idempotencia o recuperación acordada. El flujo de alquiler continuará independiente de la Clínica.

## Decisiones y validación pendientes

Consultar [arquitectura](../../docs/ARCHITECTURE.md), [D1](../../docs/adr/ADR-014-limites-servicios-v2.md), [D2](../../docs/adr/ADR-002-arquitectura-interna.md), [D3](../../docs/adr/ADR-015-persistencia-v2.md), [D9](../../docs/adr/ADR-009-consumo-clinica.md) y [contratos](../../docs/contracts/README.md).

Antes de implementar, acordar contrato procesable, datos mínimos y estados del informe. Luego probar aislamiento de permisos, duplicados, evidencia incompleta, timeout después de un posible efecto y recuperación del proveedor.

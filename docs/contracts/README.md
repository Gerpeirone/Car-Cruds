# Contratos e integración

| Campo | Valor |
| --- | --- |
| Versión documental | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Preparación documental de la entrega 2 |

La integración comunicada por el grupo publica una lista de empleados al gimnasio y consume turnos de una clínica para revisiones de posibles incorporaciones. El servicio responsable es Empleados. Los acuerdos externos se siguen en [INTEGRACIONES.md](../INTEGRACIONES.md).

| Artefacto | Capacidad | Estado real |
| --- | --- | --- |
| [EMPLOYEES.md](EMPLOYEES.md) y [employees-openapi-draft.json](employees-openapi-draft.json) | Lista propia de empleados para el gimnasio | Propuesta procesable 0.1.0; no acordada, implementada ni publicada |
| Devolución del uso de membresías | Comunicación del gimnasio para el informe de beneficios | Mecanismo y contrato pendientes; no se inventó una API externa |
| [CLINIC.md](CLINIC.md) | Consumo del servicio de turnos | Capacidad identificada; contrato del proveedor no recibido |
| [RESERVAS.md](RESERVAS.md) y [openapi-v1.json](openapi-v1.json) | Disponibilidad y reservas de vehículos | Contrato 1.0.0 y mock histórico de la entrega 1; comportamiento sin modificaciones |

## Condiciones para integrar

Antes de publicar la capacidad nueva se requieren revisión conjunta del contrato, población y campos autorizados, errores, versionado, ejemplos acordados, credenciales y entorno accesible. La devolución del gimnasio necesita identidad, período, significado del uso, deduplicación y precedencia de correcciones. Clínica debe aportar sus operaciones y condiciones reales.

La documentación en texto y el borrador propio no acreditan integración operativa. El test de contrato de la capacidad consumida se elaborará sobre el contrato real de la clínica. Los mocks futuros deben identificarse como simulaciones y no sustituir pruebas contra dependencias reales del enunciado.

## Historia y decisiones

[ADR-017](../adr/ADR-017-contrato-empleados.md) reemplaza D8 de la primera entrega. El contrato de reservas permanece en su archivo original para conservar el mock y sus pruebas; no se renombra como contrato de empleados ni se cambia el significado de sus rutas.

La capacidad propia debe mantenerse operativa y accesible hasta terminar la evaluación una vez publicada. URL, fecha de publicación y resultados de pruebas siguen pendientes.

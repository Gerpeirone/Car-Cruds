# Frontend web

**Equipos externos:** gimnasio del grupo de Pablo (consume nuestra lista de empleados) y hospital del grupo de Octi (Car Cruds consume su API). Las referencias a “Clínica” en el diseño identifican al hospital de Octi. Contratos, entornos y credenciales siguen pendientes de acuerdo.

| Aspecto | Diseño propuesto de entrega 2 |
| --- | --- |
| Estado | Estructura inicial; interfaz pendiente de implementación |
| Usuarios | Cliente, operador y responsable de personal |
| Integración | API Gateway mediante HTTP |
| Tecnología | Framework pendiente de selección |

La interfaz web presenta los flujos de búsqueda, cotización y reserva, además de la operación de alquileres y administración según el rol.

## Flujos previstos

| Usuario | Funcionalidades |
| --- | --- |
| Cliente | Ingreso, catálogo y búsqueda, disponibilidad y precio, reserva, consulta y cancelación |
| Operador | Administración de flota y clientes, bloqueos de agenda, retiro y devolución |
| Responsable de personal | Empleados; solicitudes de revisión y seguimiento de turnos; informe de uso de membresías por período |

`src/` aloja la estructura prevista del código. La interfaz consume el gateway; el acceso a persistencia, mensajería y proveedores corresponde a los servicios de negocio.

El informe de clínica distingue turno confirmado, ausencia confirmada y estado desconocido. El de gimnasio distingue uso, no uso e información faltante. La interfaz mostrará fecha/origen y condiciones pendientes; no tratará una falla externa como un resultado negativo ni generará tarifas o conclusiones médicas. La interfaz de estos flujos aún no tiene implementación. Nuevas vinculaciones, intenciones y primer envío de turnos requieren empleado activo; la baja conserva seguimiento previo. Candidatos se retiró según ADR-018.

## Referencias

- [SPEC.md — actores, alcance y reglas](../SPEC.md).
- [Arquitectura del sistema](../docs/ARCHITECTURE.md).
- [D1 vigente — empleados sin candidatos](../docs/adr/ADR-018-empleados-sin-candidatos.md).
- [D8 vigente — empleados](../docs/adr/ADR-017-contrato-empleados.md).
- [Contrato histórico de reservas](../docs/contracts/RESERVAS.md).
- [Historias y criterios de aceptación](../docs/BACKLOG.md).

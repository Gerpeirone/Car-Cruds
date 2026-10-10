# API Gateway

| Aspecto | Diseño propuesto de entrega 2 |
| --- | --- |
| Estado | Estructura de directorios y responsabilidades; implementación pendiente |
| Responsabilidad | Entrada pública, autenticación, direccionamiento y control de tráfico |
| Destinos | Clientes, Flota a través del Load Balancer, Alquileres y Empleados |
| Tecnología | Pendiente de selección |

El gateway concentra el acceso del frontend y las aplicaciones consumidoras. Propaga identidad y correlación hacia los servicios. Las reglas de reservas y el acceso a datos de negocio corresponden al servicio propietario.

El gimnasio accede a la lista autorizada de Empleados por este punto de entrada. La devolución del uso tendrá el mecanismo acordado con ese equipo; si utiliza HTTP entrante se aplicarán las mismas políticas de identidad y permisos. El balanceador interno selecciona réplicas de Flota; no reemplaza estas políticas del gateway.

## Organización prevista

| Directorio | Responsabilidad |
| --- | --- |
| `src/routes/` | Definición de rutas y destinos |
| `src/middleware/` | Autenticación, permisos de entrada, correlación y límites de tráfico |

La clínica se consume directamente desde Empleados; el gateway propio no realiza ni canaliza esa llamada saliente. La demostración histórica de reservas utiliza un [mock HTTP local](../mocks/rentals/README.md) accesible directamente. Este directorio no implementa todavía un gateway operativo.

## Referencias

- [SPEC.md — actores y permisos](../SPEC.md).
- [Arquitectura del sistema](../docs/ARCHITECTURE.md).
- [D1 vigente — límites](../docs/adr/ADR-014-limites-servicios-v2.md).
- [D5 vigente — comunicación](../docs/adr/ADR-016-comunicacion-v2.md).
- [D8 vigente — capacidad de empleados](../docs/adr/ADR-017-contrato-empleados.md).
- [Load Balancer previsto](../load-balancer/README.md).

# Car Cruds

**Sistema de gestión de alquiler de vehículos**<br/>
Trabajo Práctico Integrador · Arquitectura de Software 2026<br/>
Universidad Católica de Córdoba · Facultad de Ingeniería

| Campo | Valor |
| --- | --- |
| Versión documental | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Preparación documental; implementación de entrega 2 pendiente |
| Hito | Entrega 2: dominio propio de punta a punta |
| Fecha de entrega | Viernes 23 de octubre de 2026 |

## Presentación del proyecto

Car Cruds es un sistema diseñado para administrar el alquiler de vehículos de una concesionaria. Su objetivo es gestionar la selección de un auto, la consulta de disponibilidad, la reserva por un período y el registro de retiro y devolución, conservando las condiciones económicas acordadas.

La operación central consiste en asignar un vehículo a un cliente durante un período. Esta asignación requiere validar al conductor, calcular el importe y resolver solicitudes concurrentes: un mismo auto no puede reservarse para períodos superpuestos. Además, una devolución tardía debe impedir su entrega física a otro cliente.

La revisión de diseño añade Empleados y un Load Balancer según el feedback docente. La gestión de personal publicará una lista al gimnasio, registrará el uso de membresías comunicado por ese sistema y consumirá turnos de una clínica para revisiones de posibles incorporaciones. Los informes apoyarán el control administrativo de beneficios y el seguimiento de turnos. Los acuerdos externos se mantienen explícitamente pendientes.

## Estado del proyecto

| Componente | Resultado disponible |
| --- | --- |
| Especificación y planificación | Alcance, 8 épicas, 22 historias y requerimientos trazables; criterios de aceptación diseñados |
| Arquitectura | Cuatro servicios, gateway y Load Balancer explícitos; diagramas y propiedad de datos |
| Decisiones arquitectónicas | Propuestas para entrega 2 e historial de D1/D3/D5/D8 reemplazados; validación operativa pendiente |
| Integraciones de personal | Gimnasio y clínica identificados; borrador propio de empleados y acuerdos externos pendientes |
| Ejecutable disponible | Mock histórico de reservas en memoria, con contrato 1.0.0 y pruebas de entrega 1 |
| Servicios, infraestructura y frontend reales | Diseño y estructura documental; implementación pendiente |

Esta revisión prepara documentación antes de programar. No acredita la entrega 2: el 23 de octubre se requiere un servicio operativo además de la capacidad compartida, almacenamiento real, logs correlacionados y una primera traza, junto con las decisiones y evidencias correspondientes. Ver [el control de entrega 2](docs/ENTREGA-2.md).

## Flujo principal

1. Buscar autos por marca, categoría, transmisión, plazas y tarifa.
2. Consultar disponibilidad y precio para un período.
3. Crear una reserva con un cliente habilitado y un importe esperado.
4. Registrar el retiro y comenzar el alquiler.
5. Registrar la devolución y finalizar el alquiler.

La consulta de disponibilidad es informativa y no bloquea el vehículo. La creación de la reserva vuelve a validar el período y el importe esperado; los reintentos con la misma clave de idempotencia recuperan el resultado original.

## Servicios de negocio

| Servicio | Responsabilidad principal |
| --- | --- |
| **Clientes** | Administrar perfiles y verificar la habilitación del conductor |
| **Flota** | Administrar el catálogo de autos, sus características y la tarifa diaria vigente |
| **Alquileres** | Controlar reservas, períodos ocupados, precios acordados, retiro y devolución |
| **Empleados** | Gestionar candidatos y empleados, la lista para el gimnasio y los informes administrativos de membresías y turnos de clínica |

Alquileres es la autoridad sobre la disponibilidad temporal. Empleados concentra las integraciones de personal; la clínica se consume directamente desde ese servicio. El API Gateway centraliza acceso y autenticación. El Load Balancer previsto distribuye llamadas a Flota entre dos réplicas; su puesta en marcha y verificación siguen pendientes. La distribución se detalla en [la arquitectura](docs/ARCHITECTURE.md).

## Flujos de personal previstos

1. Mantener empleados; publicar la población autorizada al gimnasio; registrar su devolución de uso/no uso e informar uso, no uso o datos faltantes por período.
2. Registrar posibles incorporaciones; solicitar revisiones conforme al contrato de la clínica; conciliar turnos e informar quién tiene turno confirmado, quién no y qué información falta verificar.

Un candidato no es automáticamente un empleado. Un timeout no confirma “sin turno”; un dato faltante no confirma “no usa la membresía”. No hay tarifas acordadas para calcular gastos monetarios, ni se incorporan diagnósticos, aptitud o contratación automática. Ver [integraciones y acuerdos pendientes](docs/INTEGRACIONES.md).

## Ejecución local del mock histórico

**Requisito:** Go 1.22 o posterior. El mock utiliza la biblioteca estándar y no requiere bases de datos ni servicios externos.

Desde la raíz del proyecto:

```powershell
go -C mocks/rentals run .
```

El mock escucha por defecto en `http://127.0.0.1:8080`. En otra terminal:

```powershell
$headers = @{ Authorization = 'Bearer demo-token' }
Invoke-RestMethod -Headers $headers -Uri 'http://127.0.0.1:8080/v1/availability?vehicleId=auto-001&startsOn=2026-10-15&endsOn=2026-10-18'
```

### Verificación automatizada

```powershell
go -C mocks/rentals test -race ./...
```

Las pruebas verifican concurrencia, repetición idempotente, cancelación, validaciones y respuestas del contrato. Los requisitos adicionales de `-race` en Windows se describen en [la guía del mock](mocks/rentals/README.md).

El servidor conserva datos de demostración en memoria; reiniciarlo restablece su estado. El token local es ficticio. El recorrido completo está en [la guía de demostración](mocks/rentals/README.md), y sus formatos y errores en [el contrato histórico de reservas](docs/contracts/RESERVAS.md). Este mock no implementa Empleados ni las integraciones nuevas.

## Documentación

| Archivo | Contenido |
| --- | --- |
| [Enunciado TP Final.md](Enunciado%20TP%20Final.md) | Requisitos académicos en Markdown compacto, con hitos y decisiones obligatorias |
| [clases.zip](clases.zip) | Material de cátedra: 17 documentos Markdown de teoría y práctica |
| [SPEC.md](SPEC.md) | Alcance, reglas de negocio, estados y criterios de aceptación |
| [docs/BACKLOG.md](docs/BACKLOG.md) | Épicas e historias con prioridades, dependencias y criterios Dado/Cuando/Entonces |
| [docs/REQUERIMIENTOS.md](docs/REQUERIMIENTOS.md) | Requerimientos funcionales y de calidad con trazabilidad |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Contexto, contenedores, servicios, datos y comunicaciones |
| [docs/ENTREGA-2.md](docs/ENTREGA-2.md) | Preparación y evidencia pendiente para el 23 de octubre |
| [docs/INTEGRACIONES.md](docs/INTEGRACIONES.md) | Gimnasio, clínica y acuerdos pendientes EXT-01 a EXT-14 |
| [docs/ENTREGA-1.md](docs/ENTREGA-1.md) | Control y evidencia históricos de la primera entrega |
| [docs/adr/README.md](docs/adr/README.md) | Decisiones propuestas vigentes, patrones e historia de reemplazos |
| [docs/contracts/README.md](docs/contracts/README.md) | Índice de contratos, borradores y estado real |
| [docs/contracts/EMPLOYEES.md](docs/contracts/EMPLOYEES.md) | Propuesta propia de lista de empleados, todavía no acordada |
| [docs/contracts/CLINIC.md](docs/contracts/CLINIC.md) | Condiciones para consumir el contrato real de la clínica |
| [docs/contracts/openapi-v1.json](docs/contracts/openapi-v1.json) | Contrato histórico de reservas 1.0.0, utilizado por el mock |
| [mocks/rentals/README.md](mocks/rentals/README.md) | Ejecución y recorrido de demostración del mock |

## Organización inicial

```text
services/
  customers/     # Clientes y habilitación del conductor
  fleet/         # Catálogo, características y tarifas
  rentals/       # Reservas, agenda, retiro y devolución
  employees/     # Gestión administrativa de personal e integraciones previstas
gateway/         # Entrada única, autenticación y direccionamiento previstos
load-balancer/   # Distribución prevista entre dos réplicas de Flota
frontend/        # Interfaz web prevista
mocks/rentals/   # Mock histórico de reservas; no implementa Empleados
docs/            # Arquitectura, decisiones, contrato y control de entrega
```

Los directorios describen responsabilidades y estructura prevista. Empleados y Load Balancer contienen documentación, sin implementación ejecutable. El código existente sigue en `mocks/rentals/`; Go se utiliza allí para la simulación del contrato. Las tecnologías y versiones definitivas se registrarán antes de implementar los servicios.

## Organización del trabajo

La modalidad prevista utiliza un tablero Kanban, ramas `feature/<tarea>` o `docs/<tarea>` y revisión por otro integrante antes de integrar cambios mediante pull request a `main`. Las modificaciones de comportamiento deben actualizar la especificación, el contrato y los ADR correspondientes. La versión final evaluable se mantendrá en la rama principal.

## Información administrativa y acceso

| Dato | Estado |
| --- | --- |
| Integrantes | Por completar |
| Repositorio público | [Gerpeirone/Car-Cruds](https://github.com/Gerpeirone/Car-Cruds) |
| Feedback de entrega 1 | Incorporado al diseño: Empleados y Load Balancer; documentación antes de código |
| Validación del alcance ampliado y acuerdos externos | Pendiente de registro con cátedra y equipos externos |
| Ejecutable disponible | Mock histórico local en `http://127.0.0.1:8080` |
| Capacidad nueva para el gimnasio | Borrador de lista de empleados; sin servidor ni URL pública |

Una vez publicada, la capacidad real deberá permanecer accesible al consumidor hasta finalizar la evaluación. El avance y las evidencias pendientes se registran en [el control de entrega 2](docs/ENTREGA-2.md).

# Car Cruds

**Sistema de gestión de alquiler de vehículos**<br/>
Trabajo Práctico Integrador · Arquitectura de Software 2026<br/>
Universidad Católica de Córdoba · Facultad de Ingeniería

| Campo | Valor |
| --- | --- |
| Versión documental | 0.1 |
| Fecha | 7 de octubre de 2026 |
| Estado | Diseño preliminar |
| Hito | Entrega 1: diseño y contrato con mock |
| Fecha de presentación | Viernes 9 de octubre de 2026 |

## Presentación del proyecto

Car Cruds es un sistema diseñado para administrar el alquiler de vehículos de una concesionaria. Su objetivo es gestionar la selección de un auto, la consulta de disponibilidad, la reserva por un período y el registro de retiro y devolución, conservando las condiciones económicas acordadas.

La operación central consiste en asignar un vehículo a un cliente durante un período. Esta asignación requiere validar al conductor, calcular el importe y resolver solicitudes concurrentes: un mismo auto no puede reservarse para períodos superpuestos. Además, una devolución tardía debe impedir su entrega física a otro cliente.

## Alcance de la primera entrega

| Componente | Resultado disponible |
| --- | --- |
| Especificación | Alcance, actores, reglas de negocio, estados y criterios de aceptación |
| Arquitectura | Diagramas de contexto y contenedores, responsabilidades y propiedad de los datos |
| Decisiones arquitectónicas | D1 y D8; versiones iniciales de D3 y D5 |
| Contrato de integración | API de disponibilidad y reservas, documentada en OpenAPI 3.0.3 |
| Mock | Servidor HTTP local con datos de demostración y pruebas automatizadas |
| Estructura del proyecto | Directorios iniciales de servicios, gateway y frontend |

El hito comprende el diseño y la simulación del contrato. La implementación de los microservicios, la persistencia, la interfaz web y el despliegue corresponden a las siguientes etapas del proyecto.

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

Alquileres es la autoridad sobre la disponibilidad temporal. El API Gateway centraliza el acceso al sistema. La distribución de componentes y sus dependencias se detalla en [la arquitectura](docs/ARCHITECTURE.md).

## Ejecución local

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

El servidor conserva datos de demostración en memoria; reiniciarlo restablece su estado. El token local es ficticio. El recorrido completo de consulta, reserva y cancelación está en [la guía de demostración](mocks/rentals/README.md), y los formatos y errores se documentan en [el contrato](docs/contracts/README.md).

## Documentación

| Archivo | Contenido |
| --- | --- |
| [Enunciado TP Final.md](Enunciado%20TP%20Final.md) | Requisitos académicos en Markdown compacto, con hitos y decisiones obligatorias |
| [clases.zip](clases.zip) | Material de cátedra: 17 documentos Markdown de teoría y práctica |
| [SPEC.md](SPEC.md) | Alcance, reglas de negocio, estados y criterios de aceptación |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Contexto, contenedores, servicios, datos y comunicaciones |
| [docs/ENTREGA-1.md](docs/ENTREGA-1.md) | Matriz de entregables, evidencia de verificación y control de presentación |
| [docs/adr/](docs/adr/) | D1, D8 y versiones iniciales de D3 y D5 |
| [docs/contracts/README.md](docs/contracts/README.md) | Capacidad ofrecida, ejemplos y errores |
| [docs/contracts/openapi-v1.json](docs/contracts/openapi-v1.json) | Contrato OpenAPI, versión de API 1.0.0 |
| [mocks/rentals/README.md](mocks/rentals/README.md) | Ejecución y recorrido de demostración del mock |

## Organización inicial

```text
services/
  customers/     # Clientes y habilitación del conductor
  fleet/         # Catálogo, características y tarifas
  rentals/       # Reservas, agenda, retiro y devolución
gateway/         # Entrada única, autenticación y direccionamiento previstos
frontend/        # Interfaz web prevista
mocks/rentals/   # Simulación ejecutable de la capacidad compartida
docs/            # Arquitectura, decisiones, contrato y control de entrega
```

Los directorios de servicios contienen su estructura inicial y la descripción de responsabilidades y dependencias. La implementación ejecutable de esta entrega se encuentra en `mocks/rentals/`. Go se utiliza para la simulación del contrato; las tecnologías definitivas de los servicios se registrarán al iniciar su implementación.

## Organización del trabajo

La modalidad prevista utiliza un tablero Kanban, ramas `feature/<tarea>` o `docs/<tarea>` y revisión por otro integrante antes de integrar cambios mediante pull request a `main`. Las modificaciones de comportamiento deben actualizar la especificación, el contrato y los ADR correspondientes. La versión final evaluable se mantendrá en la rama principal.

## Información administrativa y acceso

| Dato | Estado |
| --- | --- |
| Integrantes | Por completar |
| Repositorio público | [Gerpeirone/Car-Cruds](https://github.com/Gerpeirone/Car-Cruds) |
| Aprobación docente del dominio y alcance | Por confirmar |
| Capacidad disponible en esta entrega | Mock local en `http://127.0.0.1:8080` |
| Publicación de la capacidad real | Prevista para una etapa posterior; URL por definir |

La capacidad real deberá permanecer accesible al grupo consumidor hasta finalizar la evaluación. Los aspectos por resolver antes de presentar se registran en [el control de entrega](docs/ENTREGA-1.md).

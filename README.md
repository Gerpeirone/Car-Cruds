# Car Cruds

**Sistema de gestión de alquiler de vehículos**<br/>
Trabajo Práctico Integrador · Arquitectura de Software 2026<br/>
Universidad Católica de Córdoba · Facultad de Ingeniería

| Campo | Valor |
| --- | --- |
| Versión documental | 0.3 |
| Fecha | 14 de octubre de 2026 |
| Estado | Backend propio implementado y verificado localmente; integración externa pendiente |
| Hito | Entrega 2 — viernes 23 de octubre de 2026 |
| Equipo | Alterman, Ivo · Peirone, German · Grbavac, Darko |
| Repositorio | [Gerpeirone/Car-Cruds](https://github.com/Gerpeirone/Car-Cruds) |

## Presentación

Car Cruds administra el alquiler de autos de una concesionaria: catálogo, búsqueda, disponibilidad, reserva, retiro y devolución. Alquileres controla la agenda, congela el precio acordado y evita asignaciones superpuestas. Un vehículo todavía no devuelto impide otro retiro, incluso después del fin planificado.

La ampliación de entrega 2 incorpora Empleados y un balanceador explícito. El gimnasio del grupo de Pablo consumirá nuestra lista de empleados y aportará información de membresías y gastos. El hospital del grupo de Octi, denominado Clínica en el diseño, proveerá consultas y turnos. Esos contratos, accesos y pruebas con los otros equipos permanecen pendientes.

## Estado disponible

| Componente | Implementación y alcance |
| --- | --- |
| Clientes | Servicio Go por capas; perfiles, habilitación e historial derivado en PostgreSQL propio |
| Flota | Servicio Go por capas; catálogo versionado en MongoDB, caché Redis, eventos e índice Solr |
| Alquileres | Servicio Go hexagonal; agenda, reservas, idempotencia durable, mantenimiento, retiro y devolución en PostgreSQL propio |
| Empleados | Servicio Go hexagonal; empleados y candidatos separados, PostgreSQL propio y lista mínima de empleados activos |
| Gateway y balanceador | Gateway Go con autenticación y límites de concurrencia; NGINX interno distribuye a dos réplicas de Flota |
| Infraestructura | Docker Compose con almacenes, RabbitMQ, indexador y Jaeger; imágenes fijadas por digest |
| Interfaz web | Pendiente; la ejecución actual se realiza por API y herramientas de verificación |
| Integraciones externas | Puertos definidos en Empleados; sin adaptadores ni contratos ficticios de gimnasio u hospital |
| Mock de entrega 1 | Conservado en un módulo independiente; no representa la API real completa |

El conjunto propio fue construido y ejecutado con Docker. Las pruebas reales, el recorrido HTTP, la persistencia tras reinicio y una traza distribuida están registrados en [el estado de implementación](docs/ESTADO-IMPLEMENTACION.md). El funcionamiento local de la lista no acredita su publicación ni la aceptación del gimnasio. [ENTREGA-2.md](docs/ENTREGA-2.md) distingue cobertura obtenida y pendientes académicos.

## Ejecución local

Se requiere Go **1.26.1** y Docker con Compose. Desde la raíz:

```powershell
go run ./cmd/devsetup
docker compose up -d --build --wait --wait-timeout 300
go run ./cmd/smoke -history
```

`devsetup` genera credenciales locales aleatorias en `.env`, que no se versiona. El generador no reemplaza una configuración existente. La entrada predeterminada es [http://127.0.0.1:8080](http://127.0.0.1:8080), y Jaeger está en [http://127.0.0.1:16686](http://127.0.0.1:16686). Las API requieren los tokens de aplicación configurados; las credenciales no se incluyen en la documentación.

La [herramienta de verificación](cmd/smoke/README.md) comprueba permisos, lista de empleados, dos réplicas, catálogo/caché/búsqueda, reservas concurrentes, retiro/devolución y publicación de historial. Utiliza registros ficticios identificables y realiza limpieza limitada a sus propios datos. No llama a los proveedores externos.

`docker compose down` conserva los volúmenes. No utilizar `down -v` para una detención habitual, porque elimina los datos locales.

## Pruebas y automatización

```powershell
go test -race -count=1 -v ./cmd/... ./internal/... ./services/...
go vet ./cmd/... ./internal/... ./services/...
go -C mocks/rentals test -race -count=1 ./...
```

Las pruebas de integración requieren variables `TEST_*` explícitas y dependencias reales, descritas en los README de cada servicio. Sin ellas, los casos correspondientes se omiten; una omisión no demuestra persistencia ni integración. El 14 de octubre la suite final de componentes sobre el backend corregido aprobó 60 pruebas con todas las dependencias configuradas, sin omisiones; vet y el nuevo recorrido de 78 solicitudes también finalizaron satisfactoriamente.

El [workflow de CI](.github/workflows/ci.yml) ejecuta pruebas y análisis estático; otro trabajo construye el entorno, ejecuta las pruebas con dependencias reales sin omisiones y realiza `cmd/smoke -history`. Ambos trabajos aprobaron en GitHub Actions sobre la revisión `11d475c` el 14 de octubre de 2026 ([ejecución](https://github.com/Gerpeirone/Car-Cruds/actions/runs/38509003244)).

El despliegue público con HTTPS para la capacidad compartida se describe en [DESPLIEGUE](docs/DESPLIEGUE.md).

Las herramientas opcionales `go run ./cmd/messagecheck` y `pwsh -File ./scripts/verify-resilience.ps1` verifican cola de fallidos y recuperación de componentes propios. `go run ./cmd/load` realiza carga GET local; `pwsh -File ./scripts/measure-cache.ps1 -DurationSeconds 5` compara caché caliente y fallback, restaurando Redis. Sus resultados y condiciones están en [EVIDENCIA-E2](docs/EVIDENCIA-E2.md).

## Servicios y propiedad de datos

| Servicio | Responsabilidad | Guía |
| --- | --- | --- |
| Clientes | Perfil y habilitación de conductor; historial de alquileres completados | [README](services/customers/README.md) |
| Flota | Características, tarifa vigente y proyecciones de lectura | [README](services/fleet/README.md) |
| Alquileres | Disponibilidad temporal, agenda y ciclo de vida | [README](services/rentals/README.md) |
| Empleados | Personal y candidatos; integración administrativa prevista | [README](services/employees/README.md) |

Consultar disponibilidad no bloquea un auto. Reservar vuelve a validar las fechas y el precio autoritativo; repetir la misma clave y solicitud recupera el resultado persistido. Redis y Solr no autorizan la tarifa de confirmación ni la agenda.

`GET /v1/employees` devuelve únicamente `employeeId` y `displayName` de empleados activos. Gimnasio dispone de lectura y no de operaciones administrativas ni candidatos. El [contrato implementado](docs/contracts/employees-openapi.json) es una propuesta propia provisional; el acuerdo de población, identificadores, acceso público y devolución económica sigue pendiente.

## Documentación

| Archivo | Contenido |
| --- | --- |
| [Enunciado TP Final.md](Enunciado%20TP%20Final.md) y [clases.zip](clases.zip) | Requisitos y material de cátedra |
| [SPEC.md](SPEC.md) | Alcance, reglas RN-01 a RN-22 y estados |
| [BACKLOG.md](docs/BACKLOG.md) | 9 épicas y 28 historias, prioridades y cobertura |
| [REQUERIMIENTOS.md](docs/REQUERIMIENTOS.md) | 22 RF y 12 RNF con trazabilidad |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | Servicios, estilos, datos y comunicaciones |
| [Índice ADR](docs/adr/README.md) | D1 a D13, decisiones actuales e historial |
| [ESTADO-IMPLEMENTACION.md](docs/ESTADO-IMPLEMENTACION.md) | Código disponible, pruebas y límites de la evidencia |
| [EVIDENCIA-E2.md](docs/EVIDENCIA-E2.md) y [POSTMORTEM.md](docs/POSTMORTEM.md) | Informes, reproducción y fallas controladas |
| [ENTREGA-2.md](docs/ENTREGA-2.md) | Matriz de cumplimiento para el 23 de octubre |
| [INTEGRACIONES.md](docs/INTEGRACIONES.md) | Acuerdos externos EXT-01 a EXT-15 |
| [Contratos](docs/contracts/README.md) | API propias actuales y artefactos históricos |
| [ENTREGA-1.md](docs/ENTREGA-1.md) | Evidencia histórica de la primera entrega |

## Organización

```text
cmd/             Ejecutables, preparación local y verificaciones
internal/        Autenticación, HTTP, observabilidad y configuración compartida
services/        Clientes, Flota, Alquileres y Empleados
gateway/         Documentación del gateway implementado
load-balancer/   Configuración NGINX y documentación
infra/           Configuración de infraestructura e imágenes
frontend/        Estructura de interfaz prevista
mocks/rentals/   Mock histórico de la entrega 1
docs/            Especificación, ADR, contratos y evidencia
```

Los cambios se presentan en ramas de trabajo y mediante pull request. Su integración en `main` requiere aprobación del equipo. Los archivos de entorno, temporales, ejecutables y logs permanecen fuera de Git.

## Alcance externo pendiente

El informe del gimnasio deberá distinguir afiliación, uso, costo por membresía, gastos por empleado y total por período/moneda. Una respuesta faltante no significa “no uso” ni costo cero. El hospital deberá informar quién necesita consulta, permitir solicitar el turno y comunicar sus cambios; el flujo de candidatos se conserva separado. Un timeout no prueba ausencia de turno.

Estas funciones corresponden a RN-15 a RN-22, RF-13 a RF-17/RF-19 y HU-13 a HU-17/HU-23. No se han implementado endpoints de otros grupos ni se han asignado tarifas o resultados médicos. La lista propia y los puertos permiten avanzar sin presentar acuerdos inexistentes.

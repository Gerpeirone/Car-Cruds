# Car Cruds

> **Cierre de la entrega 3 — 11/11/2026:** `main` `309a443` está desplegado en Azure (<https://57-156-56-201.sslip.io>) y aprobó todos los trabajos de GitHub Actions. El [estado del despliegue](#estado-del-despliegue) distingue lo verificado en Azure de lo probado en CI y laboratorio. La matriz del enunciado está en [ENTREGA-3](docs/ENTREGA-3.md). Los avisos de etapas anteriores se conservan en los documentos fechados y en el historial de Git.

**Sistema de gestión de alquiler de vehículos**<br/>
Trabajo Práctico Integrador · Arquitectura de Software 2026<br/>
Universidad Católica de Córdoba · Facultad de Ingeniería

| Campo | Valor |
| --- | --- |
| Versión documental | 0.5 |
| Fecha de revisión | 11 de noviembre de 2026 |
| Estado | `main` `309a443` desplegado en Azure y con CI aprobada; aceptación desde el backend de Gimnasio pendiente |
| Hito | Presentación grupal (entrega 3) — 11 o 13 de noviembre de 2026 |
| URL pública | <https://57-156-56-201.sslip.io> |
| Equipo | Alterman, Ivo · Peirone, German · Grbavac, Darko |
| Repositorio | [Gerpeirone/Car-Cruds](https://github.com/Gerpeirone/Car-Cruds) |

## Presentación

Car Cruds administra el alquiler de autos de una concesionaria: catálogo, búsqueda, disponibilidad, reserva, retiro y devolución. Alquileres controla la agenda, congela el precio acordado y evita asignaciones superpuestas. Un vehículo todavía no devuelto impide otro retiro, incluso después del fin planificado.

La ampliación de entrega 2 incorpora Empleados y un balanceador explícito. El gimnasio del grupo de Pablo consume nuestra lista de empleados y aporta información de membresías y gastos. El hospital del grupo de Octi, denominado Clínica en el diseño, provee consultas y turnos. Empleados consume el contrato oficial del hospital: en Azure se verificaron las consultas, y un turno real reservado y confirmado el 10/11/2026 conserva su [evidencia](docs/evidence/hospital-live-2026-11-10.json). La evidencia de aceptación desde el backend del Gimnasio sigue pendiente.

Empleados administra exclusivamente personal registrado, membresías y seguimiento administrativo de turnos. Solo empleados activos pueden iniciar vinculaciones, intenciones y envíos que puedan crear turnos; la baja preserva historia y seguimiento previo. [ADR-019](docs/adr/ADR-019-consumo-turnos-socios.md) adapta el flujo existente a Turnos para socios 1.0.0, sin incorporar nuevas funcionalidades de presentación final.

La [revisión del 8 de noviembre](docs/REVISION-E2-2026-11-08.md) y [PENDIENTES-E2](docs/PENDIENTES-E2.md) conservan el estado histórico de la entrega 2, incluida la falla de CI de `19c4e06`, resuelta después con MongoDB 7.0 en el runner.

## Estado disponible

| Componente | Implementación y alcance |
| --- | --- |
| Clientes | Servicio Go por capas; perfiles, habilitación e historial derivado en PostgreSQL propio |
| Flota | Servicio Go por capas; catálogo versionado en MongoDB, caché Redis, eventos e índice Solr |
| Alquileres | Servicio Go hexagonal; agenda, reservas, idempotencia durable, mantenimiento, retiro y devolución en PostgreSQL propio |
| Empleados | Servicio Go hexagonal; gestión de empleados, PostgreSQL propio y lista mínima de empleados activos |
| Gateway y balanceador | Gateway Go con autenticación y límites de concurrencia; NGINX interno distribuye a dos réplicas de Flota y resuelve sus nombres por DNS de Docker ([D12](docs/adr/ADR-012-balanceo-carga.md)) |
| Infraestructura | Docker Compose con almacenes, RabbitMQ, indexador y Jaeger; imágenes fijadas por digest |
| Interfaz web | Frontend del alquiler (F3-01), panel del operador (F3-02) y Personal, Hospital y Gimnasio (F3-03) en [`frontend/`](frontend/README.md). Incluye registro y sesión de clientes para el catálogo, y **Mis alquileres**. Publicada en Azure por Caddy |
| Integraciones externas | Gimnasio: lista, estado y recepción de evidencia publicados; la aceptación desde su backend está pendiente. Hospital: solicitud y seguimiento de turnos desde Empleados, con intención durable e informe; consultas verificadas en Azure |
| Mock de entrega 1 | Conservado en un módulo independiente; no representa la API real completa |

El conjunto propio se ejecuta con Docker Compose, tanto en local como en Azure. Las pruebas reales, el recorrido HTTP, la persistencia tras reinicio y una traza distribuida están registrados en [el estado de implementación](docs/ESTADO-IMPLEMENTACION.md). [ENTREGA-2.md](docs/ENTREGA-2.md) conserva la cobertura de la entrega 2.

## Estado del despliegue

### Verificado en Azure — 11/11/2026

| Aspecto | Resultado |
| --- | --- |
| Revisión | La VM `vm-carcruds` ejecuta `309a443`, el mismo commit que `main` |
| Contenedores | Los 16 contenedores permanentes están en ejecución. `rabbit-init` es una tarea de inicialización que termina |
| Interfaz | <https://57-156-56-201.sslip.io> responde por HTTPS. Registro, inicio de sesión, catálogo y Mis alquileres funcionan |
| Contrato público | <https://57-156-56-201.sslip.io/contracts/employees-openapi.json> publica el OpenAPI de Empleados 0.5.0 en JSON |
| Datos | 19 vehículos en el catálogo y 16 empleados en el panel |
| Búsqueda | Solr indexa los cambios; la búsqueda, los filtros y la paginación responden |
| Integraciones | Las consultas de Hospital y Gimnasio responden |
| Balanceador | NGINX resuelve dinámicamente `fleet-a` y `fleet-b` por DNS; la auditoría del despliegue lo verificó ([D12](docs/adr/ADR-012-balanceo-carga.md)) |

### Verificado en CI y laboratorio

- GitHub Actions sobre `309a443`, 11/11/2026: [backend](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829684) (contratos OpenAPI, pruebas y análisis estático, Docker Compose con recorrido persistente) y [frontend](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829457) (pruebas y build) aprobados.
- Redis (caché caliente, Redis detenido y circuit breaker), RabbitMQ (consumidor idempotente y cola de fallidos), persistencia tras reinicio, balanceo y fallas controladas se probaron en CI o en entornos de laboratorio. Cada resultado conserva su fecha y revisión en [docs/evidence](docs/evidence/) y en los ADR. La evidencia documentada de estos ensayos proviene de CI y laboratorio, no de Azure; en Azure la demostración usa lecturas.

### Pendiente

- Evidencia de aceptación desde el backend del grupo de Gimnasio ([integración](docs/contracts/INTEGRACION_GIMNASIO.md#prueba-conjunta)).
- Aceptación formal del hospital y de la cátedra.

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

## Documentación de endpoints y Swagger

La [referencia completa de API](docs/API.md) explica las operaciones HTTP implementadas (44 en el inventario que valida `scripts/check-api-docs.py` en CI), sus permisos, entradas, respuestas, errores y flujos. Para abrir Swagger UI sin bases de datos ni credenciales:

```powershell
go run ./cmd/apidocs
```

Abrir [http://127.0.0.1:8090/api/](http://127.0.0.1:8090/api/). El menú reúne Clientes, Flota, Alquileres, Empleados/gimnasio, integración clínica y rutas técnicas. Incluye recursos locales y funciona sin conexión una vez disponible el código. Es un visor de lectura; los ejemplos se ejecutan por el gateway con el rol correspondiente. No reemplaza la interfaz de negocio ni acredita aceptación externa. En Azure, el contrato de Empleados se publica en <https://57-156-56-201.sslip.io/contracts/employees-openapi.json>. [Contratos y ejemplos](docs/contracts/README.md).

## Pruebas y automatización

```powershell
go test -race -count=1 -v ./cmd/... ./internal/... ./services/...
go vet ./cmd/... ./internal/... ./services/...
go -C mocks/rentals test -race -count=1 ./...
```

Las pruebas de integración requieren variables `TEST_*` explícitas y dependencias reales, descritas en los README de cada servicio. Sin ellas, los casos correspondientes se omiten; una omisión no demuestra persistencia ni integración. Como antecedente, el 14 de octubre la suite final de componentes sobre el backend corregido aprobó 60 pruebas con todas las dependencias configuradas, sin omisiones; vet y el nuevo recorrido de 78 solicitudes también finalizaron satisfactoriamente.

El [workflow del backend](.github/workflows/ci.yml) tiene tres trabajos. El primero valida contratos OpenAPI, ejemplos y cobertura de rutas. El segundo ejecuta pruebas con detector de carreras y análisis estático. El tercero levanta Docker Compose y ejecuta las pruebas con dependencias reales, la medición de caché y frescura de búsqueda, `cmd/smoke -history`, `checkshared` y el contrato con el código original del hospital. El [workflow del frontend](.github/workflows/frontend.yml) ejecuta pruebas y build. Sobre `309a443` aprobaron los cuatro trabajos el 11 de noviembre de 2026 ([backend](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829684), [frontend](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829457)). La CI usa MongoDB 7.0 por el kernel del runner ([ARCHITECTURE §9](docs/ARCHITECTURE.md#9-limitaciones-conocidas-y-deuda-técnica-aceptada)).

El despliegue en Azure, su verificación y los procedimientos de actualización se describen en [DESPLIEGUE](docs/DESPLIEGUE.md).

La [revisión de cierre de E2 del 15 de octubre](docs/REVISION-E2-2026-10-15.md) registra las correcciones de despliegue y de retención de conexiones en reservas. `go run ./cmd/checkshared -url http://127.0.0.1:8080 -env .env` verifica por lecturas la autenticación, proyección mínima, paginación y permisos de la capacidad compartida. Para una URL externa exige HTTPS y un destino explícito; no sigue redirecciones ni acredita aceptación del gimnasio.

Las herramientas opcionales `go run ./cmd/messagecheck` y `pwsh -File ./scripts/verify-resilience.ps1` verifican cola de fallidos y recuperación de componentes propios. `go run ./cmd/load` realiza carga GET local; `pwsh -File ./scripts/measure-cache.ps1 -DurationSeconds 5` compara caché caliente y fallback, restaurando Redis. Sus resultados y condiciones están en [EVIDENCIA-E2](docs/EVIDENCIA-E2.md).

## Servicios y propiedad de datos

| Servicio | Responsabilidad | Guía |
| --- | --- | --- |
| Clientes | Perfil y habilitación de conductor; historial de alquileres completados | [README](services/customers/README.md) |
| Flota | Características, tarifa vigente y proyecciones de lectura | [README](services/fleet/README.md) |
| Alquileres | Disponibilidad temporal, agenda y ciclo de vida | [README](services/rentals/README.md) |
| Empleados | Empleados, membresías y seguimiento administrativo de turnos | [README](services/employees/README.md) |

Consultar disponibilidad no bloquea un auto. Reservar vuelve a validar las fechas y el precio autoritativo; repetir la misma clave y solicitud recupera el resultado persistido. Redis y Solr no autorizan la tarifa de confirmación ni la agenda.

`GET /v1/employees` devuelve únicamente `employeeId` y `displayName` de empleados activos. El ID es estable y no se reasigna a otra persona. Gimnasio vincula manualmente al socio, completa DNI/email de su lado y envía evidencia a `POST /v1/employees/membership-evidence`. Puede verificar bajas mediante `GET /v1/employees/{id}/status`, que devuelve solo ID y estado laboral y no cancela membresías. No puede administrar empleados ni consultar informes internos.

El [contrato implementado](docs/contracts/employees-openapi.json), versión 0.5.0, suma historial administrativo y conserva lista, estado y evidencia para Gimnasio. [COMUNICACION-GIMNASIO](docs/COMUNICACION-GIMNASIO.md) explica direcciones, cuerpo, correcciones, informe y trabajo pendiente del equipo de Pablo; [ACUERDO-GIMNASIO](docs/ACUERDO-GIMNASIO.md) responde sus preguntas y propone el perfil mensual: ingresos efectivos, pagos del mes y costo neto por meses iguales. Meses parciales quedan en `null` hasta acordar su tratamiento. Se aceptan fotografías provisionales y correcciones históricas. En Azure, el contrato 0.5.0 se publica en JSON y las consultas del Gimnasio responden. La confirmación semántica del perfil mensual y la evidencia de aceptación desde el backend del Gimnasio siguen pendientes.

## Documentación

Para presentar y reproducir el sistema, consultar [Arquitectura](docs/ARCHITECTURE.md), [operación en Azure](docs/DESPLIEGUE.md), [matriz del enunciado](docs/ENTREGA-3.md#trazabilidad-del-enunciado) y [guion de defensa](docs/GUION-PRESENTACION.md).

| Archivo | Contenido |
| --- | --- |
| [Enunciado TP Final.md](Enunciado%20TP%20Final.md) y [clases.zip](clases.zip) | Requisitos y material de cátedra |
| [SPEC.md](SPEC.md) | Alcance, reglas RN-01 a RN-22 y estados |
| [BACKLOG.md](docs/BACKLOG.md) | 9 épicas; identificadores HU-01 a HU-28 conservados, HU-10 retirada |
| [REQUERIMIENTOS.md](docs/REQUERIMIENTOS.md) | Identificadores RF-01 a RF-22 y 12 RNF; RF-10 retirado |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | Servicios, estilos, datos y comunicaciones |
| [Índice ADR](docs/adr/README.md) | D1 a D13, decisiones actuales e historial |
| [ESTADO-IMPLEMENTACION.md](docs/ESTADO-IMPLEMENTACION.md) | Código disponible, pruebas y límites de la evidencia |
| [EVIDENCIA-E2.md](docs/EVIDENCIA-E2.md) y [POSTMORTEM.md](docs/POSTMORTEM.md) | Informes, reproducción y fallas controladas |
| [ENTREGA-3.md](docs/ENTREGA-3.md) y [GUION-PRESENTACION.md](docs/GUION-PRESENTACION.md) | Matriz del enunciado para la presentación grupal y guion de defensa |
| [DESPLIEGUE.md](docs/DESPLIEGUE.md) | Despliegue en Azure, verificación, actualización y vuelta atrás |
| [ENTREGA-2.md](docs/ENTREGA-2.md) | Matriz de cumplimiento para el 23 de octubre |
| [INTEGRACIONES.md](docs/INTEGRACIONES.md) | Acuerdos externos EXT-01 a EXT-15 |
| [Contratos](docs/contracts/README.md) | API propias actuales y artefactos históricos |
| [Referencia de endpoints y Swagger](docs/API.md) | Operaciones HTTP: permisos, entradas, respuestas, errores, ejemplos y visor local |
| [ENTREGA-1.md](docs/ENTREGA-1.md) | Evidencia histórica de la primera entrega |

## Organización

```text
cmd/             Ejecutables, preparación local y verificaciones
internal/        Autenticación, HTTP, observabilidad y configuración compartida
services/        Clientes, Flota, Alquileres y Empleados
gateway/         Documentación del gateway implementado
load-balancer/   Configuración NGINX y documentación
infra/           Configuración de infraestructura e imágenes
frontend/        Interfaz React del cliente, del operador y de personal
mocks/rentals/   Mock histórico de la entrega 1
docs/            Especificación, ADR, contratos y evidencia
```

Los cambios se presentan en ramas de trabajo y mediante pull request. Su integración en `main` requiere aprobación del equipo. Los archivos de entorno, temporales, ejecutables y logs permanecen fuera de Git.

## Alcance externo pendiente

El informe propio `GET /v1/employees/reports/memberships?period=2026-10` distingue afiliación, uso, costo por membresía, gastos por empleado y totales por período/moneda; solo administración y servicio pueden consultarlo. Una respuesta faltante no significa “no uso” ni costo cero.

La [comunicación con el hospital de Octi](docs/COMUNICACION-HOSPITAL.md) mantiene vínculo, intención durable, solicitud y consulta por el contrato oficial Turnos para socios 1.0.0 fijado a `c0b65f4`. `GET /v1/employees/reports/clinic` muestra la última intención por empleado. Configurar `CLINIC_BASE_URL` terminada en `/v1` y `CLINIC_API_KEY` fuera de Git; sin URL el envío permanece deshabilitado. Un timeout conserva estado desconocido y permite recuperación explícita con la misma clave/datos, tras la espera durable. Solicitudes antiguas enviadas sin procedencia idempotente nunca se reenvían como POST. En Azure la integración está configurada y sus consultas se verificaron. Pacientes reales, condiciones económicas y aceptación formal siguen pendientes.

Estas funciones corresponden a RN-15 a RN-22, RF-13 a RF-17/RF-19 y HU-13 a HU-17/HU-23. No se han implementado endpoints de otros grupos ni se han asignado tarifas o resultados médicos. La lista propia y los puertos permiten avanzar sin presentar acuerdos inexistentes.

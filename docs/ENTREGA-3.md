# Entrega 3 — preparación de la presentación grupal

## Trazabilidad del enunciado

Corte del 11/11/2026 sobre `main` `309a443`, el mismo commit desplegado en Azure, con CI aprobada ([backend](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829684), [frontend](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829457)). La evidencia histórica conserva su revisión. Fuente académica: [transcripción del enunciado](../Enunciado%20TP%20Final.md), §2, §4.1 y Presentación Grupal, contrastada con el PDF original, páginas 8–9. «Cumplido» indica evidencia técnica suficiente en el alcance indicado. La columna de evidencia separa lo verificado en **Azure** de lo probado en **CI o laboratorio**. Una aceptación externa abierta se indica en el estado.

| Requisito | Implementación | Evidencia | Estado |
| --- | --- | --- | --- |
| Frontend de punta a punta | React: registro y sesión de clientes, catálogo, reserva, Mis alquileres, panel del operador, Personal, Hospital y Gimnasio | **Azure:** registro, inicio de sesión, catálogo y Mis alquileres ([verificación](DESPLIEGUE.md#verificación-en-azure-del-11112026)). **Laboratorio:** [pruebas de interfaz](EVIDENCIA-FRONTEND.md) y [panel de Personal](PERSONAL-OPERADOR.md) | Cumplido |
| Entrada única | Caddy, gateway, Bearer y límite de admisión | **Azure:** HTTPS, `/v1/*` con 401 sin credencial, `/internal/*` cerrado ([verificación](DESPLIEGUE.md#verificación-en-azure-del-11112026)). **CI:** [pruebas del gateway](../cmd/gateway/main_test.go) | Cumplido |
| Tres servicios operativos | Clientes, Flota, Alquileres y Empleados | **Azure:** 16 contenedores en ejecución sobre `309a443`. **CI:** [Compose y recorrido persistente](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829684) | Cumplido |
| Capacidad propia pública | Empleados, OpenAPI 0.5.0, Caddy | **Azure:** OpenAPI 0.5.0 publicado en JSON y consultas de Gimnasio verificadas ([integración](contracts/INTEGRACION_GIMNASIO.md)) | Cumplido del lado de Car Cruds; **aceptación desde el backend de Gimnasio pendiente** |
| Proveedor en un flujo importante | Hospital desde Empleados y el panel | **Azure:** consultas al Hospital verificadas. **Histórico:** [turno confirmado e informe](evidence/hospital-live-2026-11-10.json). **CI:** [test contra el proveedor original](../integration/cliniccontract/contract_test.go) | Cumplido |
| Dos tipos de almacenamiento | PostgreSQL y MongoDB | **Azure:** ambos en ejecución con los datos del despliegue. **Laboratorio y CI:** [persistencia y reinicios](evidence/runtime-persistence.json), [pruebas por servicio](ARCHITECTURE.md#responsabilidades-contratos-y-operación) | Cumplido |
| Caché con comparación | Redis para ficha de Flota | **Laboratorio:** [D7](adr/ADR-007-cache.md) registra lecturas y latencia, con límites del ensayo | Cumplido con el alcance medido |
| Dos estilos internos | Clientes por capas, Alquileres hexagonal | [Arquitectura interna](ARCHITECTURE.md#4-arquitectura-interna-y-tecnologías) | Cumplido |
| Evento, consumidor idempotente y fallidos | `RentalCompleted.v1`, outbox, inbox y DLQ | **Laboratorio:** [mensaje rechazado](evidence/message-failure.json), [D5](adr/ADR-016-comunicacion-v2.md) | Cumplido |
| Búsqueda indexada | Solr, filtros, orden, paginación y actualización por eventos | **Azure:** indexación, búsqueda, filtros y paginación sobre 19 vehículos. **CI:** [D6](adr/ADR-006-busqueda.md), [pruebas reales de Solr](../services/fleet/solr_integration_test.go) | Cumplido en operación normal |
| Decisiones con evidencia | D1–D13 y verificaciones por revisión | [Índice ADR](adr/README.md), [evidencias](evidence/). D12 incluye la corrección DNS verificada en Azure ([ADR-012](adr/ADR-012-balanceo-carga.md)) | Cumplido (CI, laboratorio y auditoría de Azure) |
| Limitaciones y deuda | Sección 9 de Arquitectura | [Estado y mitigaciones](ARCHITECTURE.md#9-limitaciones-conocidas-y-deuda-técnica-aceptada) | Cumplido |

La evidencia de aceptación desde el backend del grupo de Gimnasio sigue pendiente. Para la defensa individual, el [postmortem](POSTMORTEM.md), [D11](adr/ADR-011-observabilidad.md), [D12](adr/ADR-012-balanceo-carga.md) y [D13](adr/ADR-013-capacidad-costos.md) reúnen ensayos de fallas, observabilidad, balanceo y capacidad. Los resultados de laboratorio conservan sus límites y fechas.

| Campo | Valor |
| --- | --- |
| Fecha | 11 de noviembre de 2026 |
| Rama | `main` |
| Base | `309a443`, desplegado en Azure |
| Estado | F3-01 a F3-05 implementados; Azure publicado y verificado; aceptación desde el backend de Gimnasio pendiente |
| Referencia académica | [Enunciado](../Enunciado%20TP%20Final.md), presentación grupal |

## 1. Objetivo y alcance del hito

Se utiliza «entrega 3» como nombre de trabajo para la presentación grupal del 11 o 13 de noviembre, según la fecha asignada por la cátedra. El enunciado la denomina presentación grupal. Este documento organiza el avance y no declara cumplimiento de la presentación completa.

La presentación exige frontend completo, entrada única autenticada, servicios y almacenes operativos, capacidad propia publicada en cloud, consumo externo integrado y evidencia de caché, búsqueda, mensajería, patrones y limitaciones. El backend dispone de una base implementada; cada ampliación deberá verificarse sobre su revisión, sin trasladar automáticamente resultados históricos.

## 2. Etapas propuestas

| Etapa | Resultado | Estado |
| --- | --- | --- |
| F3-01: frontend del alquiler | Acceso de laboratorio, búsqueda, ficha, disponibilidad, reserva, consulta por código y cancelación | Implementado; [evidencia](EVIDENCIA-FRONTEND.md) y [revisión en navegador](EVIDENCIA-FRONTEND.md#6-revisión-en-navegador-real--9-de-noviembre) sin violaciones WCAG AA |
| F3-02: operación administrativa | Flota, alta/consulta de clientes, reservas por código, retiro/devolución y mantenimiento | [Implementado y probado con servicios reales](PANEL-OPERADOR.md); revisado en navegador |
| F3-03: personal e integraciones | Empleados registrados, informes de membresías y seguimiento administrativo de turnos | [Implementado](PERSONAL-OPERADOR.md) y probado contra Empleados real; gimnasio y hospital reales pendientes de acuerdos |
| F3-04: publicación y ensayo conjunto | Entrada HTTPS, interfaz publicada y acceso real de gimnasio/hospital | Publicado en Azure el 11/11/2026 sobre `309a443` ([verificación](DESPLIEGUE.md#verificación-en-azure-del-11112026)). Hospital consultado desde Empleados; falta la aceptación desde el backend de Gimnasio |
| F3-05: evidencia final | Recorrido completo, mediciones comparativas y limitaciones actualizadas | [Comparación de caché](evidence/cache-comparison-2026-11-09.json) y [limitaciones y deuda técnica](ARCHITECTURE.md#9-limitaciones-conocidas-y-deuda-técnica-aceptada) actualizadas; recorrido de registro, inicio de sesión, catálogo y Mis alquileres verificado en Azure |

F3-01 incluye ahora código de interfaz, dependencias fijadas, build, pruebas y una entrada estática local con NGINX. Se conservan los endpoints de negocio existentes. No se incorporan pagos ni funciones ajenas al alquiler documentado.

## 3. Cierre esperado de F3-01

Un cliente de laboratorio podrá entrar con la credencial asignada, buscar un auto, seleccionar fechas, obtener disponibilidad y total desde Alquileres, confirmar una única reserva y consultar o cancelar una reserva propia por código. El recorrido tendrá estados de carga, vacío, conflicto, error y resultado desconocido; será utilizable con teclado y en pantalla móvil.

La [especificación funcional](FRONTEND-ALQUILER.md) fija contratos, criterios y orden de implementación. [ADR-020](adr/ADR-020-frontend-alquiler.md) propone la tecnología, los límites de autenticación y la distribución. La implementación y sus pruebas actuales se registran en [EVIDENCIA-FRONTEND](EVIDENCIA-FRONTEND.md); la presentación completa aún requiere las otras etapas.

## 4. Dependencias y pendientes

- El cliente de laboratorio debe existir, tener credencial propia y estar vinculado por `AUTH_CLIENT_CUSTOMER_ID`; la credencial se entrega por un canal privado. Las cuentas personales creadas en `/registro` solo habilitan el catálogo; no otorgan la credencial de alquiler ni reemplazan la identidad del backend.
- Deben existir vehículos activos, un cliente habilitado y fechas vigentes para demostrar reservas. Los fixtures de pruebas se identificarán y limpiarán según su propietario.
- El fallo de arranque de MongoDB en la [CI de la base](https://github.com/Gerpeirone/Car-Cruds/actions/runs/42516259957) quedó diagnosticado y mitigado en esta entrega (kernel del runner y MongoDB 8.x; ver «CI en verde» más abajo y la sección 9 de [ARCHITECTURE](ARCHITECTURE.md)).
- El acceso cloud y los acuerdos externos se coordinan en [DESPLIEGUE](DESPLIEGUE.md) e [INTEGRACIONES](INTEGRACIONES.md). No bloquean la documentación ni el desarrollo local de F3-01.
- La propuesta de consolidación documental de E2 ([PR #7](https://github.com/Gerpeirone/Car-Cruds/pull/7)) se cerró sin integrarse el 10/11/2026.

## 5. Estado y verificación de esta etapa

El corte documental inicial se conserva en el historial. El incremento actual implementa F3-01: build, 24 pruebas ordinarias, prueba React contra API real y servidor estático Docker verificados. La herramienta de navegador se bloqueó por no poder verificar la URL en Windows; no se declara aprobado el layout ni el recorrido visual/teclado en navegador. Ver [evidencia y pendientes](EVIDENCIA-FRONTEND.md).

## 6. Cierre del 9 de noviembre

- **F3-03.** La sección Personal del panel cubre empleados (lista, alta, edición, baja), el informe de membresías por período y las solicitudes de turno (vínculo, intención con referencia estable, envío explícito, conciliación e informe). Tiene siete pruebas nuevas y se ejercitó contra el servicio real. Ver [PERSONAL-OPERADOR](PERSONAL-OPERADOR.md).
- **Revisión visual.** Se hizo en navegador real sobre 33 pantallas en escritorio y móvil, con axe WCAG 2.1 AA, scroll horizontal y teclado. Se corrigió el contraste de la paleta y el foco en las tablas desplazables; terminó con 0 violaciones.
- **F3-04.** Se agregó el servicio `web` a `compose.deploy.yaml`, y el Caddyfile ahora enruta la interfaz y la API. También se corrigió que la regla de `/internal/*` nunca se alcanzaba. Se validó con `check-deploy-config.py`, `caddy validate`, `test-deploy-vm.sh` y un despliegue local con HTTPS.
- **F3-05.** Se agregó `scripts/measure-cache.sh` (no necesita PowerShell 7) y se hicieron dos corridas, con caché caliente y con Redis detenido. Las limitaciones y la deuda técnica están en ARCHITECTURE §9.
- **CI.** El trabajo Compose vuelca diagnóstico de los contenedores y los logs de MongoDB cuando falla el arranque. La primera ejecución mostró la causa: el runner usa kernel 7.0 y MongoDB 8.x se niega a iniciar en kernels 6.19 o superiores (ver ARCHITECTURE §9). Elegir la versión de MongoDB queda para el equipo, porque afecta los volúmenes existentes.

Siguen pendientes y dependen de terceros: la VM con URL pública (cuenta cloud), el token y consumo del gimnasio, y la URL y clave del hospital. *Actualización del 11/11: la VM, la URL pública y la configuración del hospital quedaron resueltas; ver sección 10.*

## 7. Ampliación del 9 de noviembre (noche)

- **CI en verde.** La CI usa MongoDB 7.0 solo en el runner ([compose.ci.yaml](../compose.ci.yaml)) y los tres trabajos aprobaron, incluido el contrato con el hospital. Local y VM siguen con MongoDB 8.
- **Circuit breaker de caché (HU-27).** Con Redis detenido, la ficha pasó de p50 155 ms a 1,6 ms con concurrencia 1, y el breaker se cerró solo al volver Redis ([evidencia](evidence/cache-breaker-2026-11-09.json)).
- **Tablero y alertas (HU-20, D11).** [compose.observability.yaml](../compose.observability.yaml) agrega Prometheus y Grafana con cinco alertas; hay [simulacro con captura](evidence/alert-drill-2026-11-09.json).
- **Revisión de interfaz reproducible.** [scripts/ui-review](../scripts/ui-review/review.mjs) recorre 33 pantallas contra cualquier URL y falla si encuentra problemas.
- **Guion.** [GUION-PRESENTACION](GUION-PRESENTACION.md) incluye el recorrido, los requisitos con su evidencia, preguntas probables y plan B.

Siguen pendientes la VM con URL pública y el acceso real del gimnasio y del hospital. *Actualización del 11/11: ver sección 10.*

## 8. Frescura de búsqueda — 9 de noviembre de 2026

El recorrido `cmd/smoke` ahora mide alta, actualización de tarifa/versión y baja lógica desde antes de cada escritura hasta que la búsqueda refleja el cambio. En la [CI de la revisión `2dc9c61`](https://github.com/Gerpeirone/Car-Cruds/actions/runs/42745574484/job/129597135342) las tres transiciones cumplieron la cota de 5 s. El método, sus límites y la evidencia figuran en [D6](adr/ADR-006-busqueda.md#medición-de-operación-normal--9-de-noviembre-de-2026). Falta ampliar a carga sostenida y recuperación.

## 9. Criterio de cierre de evidencia del sistema

El requisito «decisiones verificadas mediante evidencia del sistema» se considera cumplido en una revisión únicamente cuando coinciden estos cuatro controles:

1. contratos OpenAPI, ejemplos, inventario de `API.md` y rutas implementadas sin diferencias;
2. pruebas y análisis estático del backend sin fallos ni omisiones de las integraciones configuradas;
3. Compose con dependencias reales, recorrido persistente, caché fría/caliente/fallback y frescura de Solr bajo carga satisfactorios;
4. pruebas, build y revisión reproducible del frontend satisfactorios.

La evidencia histórica conserva su revisión y alcance. No se traslada automáticamente a un commit nuevo: la ejecución de CI de esa revisión debe quedar enlazada aquí antes de declarar el punto cerrado. La publicación cloud y las pruebas con equipos externos siguen siendo requisitos separados.

## 10. Cierre del 11 de noviembre de 2026

Revisión evaluada: `main` `309a443`, desplegada en Azure en `https://57-156-56-201.sslip.io`.

### Controles de la sección 9

| Control | Resultado sobre `309a443` |
| --- | --- |
| 1. Contratos, ejemplos, `API.md` y rutas | Trabajo «Contratos OpenAPI y documentación de endpoints» aprobado ([ejecución](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829684)) |
| 2. Pruebas y análisis estático del backend | Trabajo «Pruebas y análisis estático» aprobado ([ejecución](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829684)) |
| 3. Compose con dependencias reales, recorrido persistente, caché y frescura de Solr | Trabajo «Docker Compose y recorrido persistente» aprobado ([ejecución](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829684)) |
| 4. Pruebas y build del frontend | Trabajo «frontend» aprobado ([ejecución](https://github.com/Gerpeirone/Car-Cruds/actions/runs/43055829457)). La revisión reproducible en navegador (`scripts/ui-review`) se registró el 9/11 sobre una revisión anterior ([informe](evidence/ui-review-2026-11-09.json)) |

### Verificado en Azure

- La VM ejecuta `309a443` con 16 contenedores en ejecución.
- La interfaz pública responde por HTTPS. Registro, inicio de sesión, catálogo y Mis alquileres funcionan.
- El OpenAPI de Empleados 0.5.0 se publica en JSON.
- El catálogo tiene 19 vehículos y el panel 16 empleados. Solr indexa, busca, filtra y pagina.
- Las consultas de Hospital y Gimnasio responden.
- La auditoría verificó la resolución DNS dinámica del balanceador ([D12](adr/ADR-012-balanceo-carga.md)).

### Probado en CI o laboratorio

Caché y circuit breaker de Redis, mensajería con RabbitMQ y cola de fallidos, persistencia tras reinicio, balanceo y fallas controladas. Cada resultado conserva su fecha y revisión en [evidence](evidence/) y en los ADR. La evidencia documentada de estos ensayos proviene de CI y laboratorio, no de Azure.

### Pendiente

- Evidencia de aceptación desde el backend del grupo de Gimnasio ([prueba conjunta](contracts/INTEGRACION_GIMNASIO.md#prueba-conjunta)).
- Aceptación formal del hospital y de la cátedra.

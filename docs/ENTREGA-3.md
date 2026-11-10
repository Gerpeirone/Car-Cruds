# Entrega 3 — preparación de la presentación grupal

| Campo | Valor |
| --- | --- |
| Fecha | 9 de noviembre de 2026 |
| Rama | `feature/entrega-3-pendientes`, sobre `feature/entrega-3` |
| Base | `main`, `19c4e06` |
| Estado | F3-01 a F3-03 implementados y revisados en navegador; entrada pública verificada con HTTPS local; falta la VM pública y los accesos externos |
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
| F3-04: publicación y ensayo conjunto | Entrada HTTPS, interfaz publicada y acceso real de gimnasio/hospital | Caddy sirve la interfaz y deriva `/v1/*` al gateway; [verificado con HTTPS local](DESPLIEGUE.md). Falta crear la VM, el DNS y los accesos externos |
| F3-05: evidencia final | Recorrido completo, mediciones comparativas y limitaciones actualizadas | [Comparación de caché](evidence/cache-comparison-2026-11-09.json) y [limitaciones y deuda técnica](ARCHITECTURE.md#9-limitaciones-conocidas-y-deuda-técnica-aceptada) actualizadas; repetir el recorrido sobre la URL pública |

F3-01 incluye ahora código de interfaz, dependencias fijadas, build, pruebas y una entrada estática local con NGINX. Se conservan los endpoints de negocio existentes. No se incorporan pagos ni funciones ajenas al alquiler documentado.

## 3. Cierre esperado de F3-01

Un cliente de laboratorio podrá entrar con la credencial asignada, buscar un auto, seleccionar fechas, obtener disponibilidad y total desde Alquileres, confirmar una única reserva y consultar o cancelar una reserva propia por código. El recorrido tendrá estados de carga, vacío, conflicto, error y resultado desconocido; será utilizable con teclado y en pantalla móvil.

La [especificación funcional](FRONTEND-ALQUILER.md) fija contratos, criterios y orden de implementación. [ADR-020](adr/ADR-020-frontend-alquiler.md) propone la tecnología, los límites de autenticación y la distribución. La implementación y sus pruebas actuales se registran en [EVIDENCIA-FRONTEND](EVIDENCIA-FRONTEND.md); la presentación completa aún requiere las otras etapas.

## 4. Dependencias y pendientes

- El cliente de laboratorio debe existir, tener credencial propia y estar vinculado por `AUTH_CLIENT_CUSTOMER_ID`; la credencial se entrega por un canal privado. La interfaz no crea cuentas ni reemplaza la identidad del backend.
- Deben existir vehículos activos, un cliente habilitado y fechas vigentes para demostrar reservas. Los fixtures de pruebas se identificarán y limpiarán según su propietario.
- El fallo de arranque de MongoDB en la [CI de la base](https://github.com/Gerpeirone/Car-Cruds/actions/runs/42516259957) quedó diagnosticado y mitigado en esta entrega (kernel del runner y MongoDB 8.x; ver «CI en verde» más abajo y la sección 9 de [ARCHITECTURE](ARCHITECTURE.md)).
- El acceso cloud y los acuerdos externos se coordinan en [DESPLIEGUE](DESPLIEGUE.md) e [INTEGRACIONES](INTEGRACIONES.md). No bloquean la documentación ni el desarrollo local de F3-01.
- La consolidación documental de E2 está propuesta en [PR #7](https://github.com/Gerpeirone/Car-Cruds/pull/7). No se integra esa propuesta en `main` ni se presupone su aceptación por iniciar esta rama.

## 5. Estado y verificación de esta etapa

El corte documental inicial se conserva en el historial. El incremento actual implementa F3-01: build, 24 pruebas ordinarias, prueba React contra API real y servidor estático Docker verificados. La herramienta de navegador se bloqueó por no poder verificar la URL en Windows; no se declara aprobado el layout ni el recorrido visual/teclado en navegador. Ver [evidencia y pendientes](EVIDENCIA-FRONTEND.md).

## 6. Cierre del 9 de noviembre

- **F3-03.** La sección Personal del panel cubre empleados (lista, alta, edición, baja), el informe de membresías por período y las solicitudes de turno (vínculo, intención con referencia estable, envío explícito, conciliación e informe). Tiene siete pruebas nuevas y se ejercitó contra el servicio real. Ver [PERSONAL-OPERADOR](PERSONAL-OPERADOR.md).
- **Revisión visual.** Se hizo en navegador real sobre 33 pantallas en escritorio y móvil, con axe WCAG 2.1 AA, scroll horizontal y teclado. Se corrigió el contraste de la paleta y el foco en las tablas desplazables; terminó con 0 violaciones.
- **F3-04.** Se agregó el servicio `web` a `compose.deploy.yaml`, y el Caddyfile ahora enruta la interfaz y la API. También se corrigió que la regla de `/internal/*` nunca se alcanzaba. Se validó con `check-deploy-config.py`, `caddy validate`, `test-deploy-vm.sh` y un despliegue local con HTTPS.
- **F3-05.** Se agregó `scripts/measure-cache.sh` (no necesita PowerShell 7) y se hicieron dos corridas, con caché caliente y con Redis detenido. Las limitaciones y la deuda técnica están en ARCHITECTURE §9.
- **CI.** El trabajo Compose vuelca diagnóstico de los contenedores y los logs de MongoDB cuando falla el arranque. La primera ejecución mostró la causa: el runner usa kernel 7.0 y MongoDB 8.x se niega a iniciar en kernels 6.19 o superiores (ver ARCHITECTURE §9). Elegir la versión de MongoDB queda para el equipo, porque afecta los volúmenes existentes.

Siguen pendientes y dependen de terceros: la VM con URL pública (cuenta cloud), el token y consumo del gimnasio, y la URL y clave del hospital.

## 7. Ampliación del 9 de noviembre (noche)

- **CI en verde.** La CI usa MongoDB 7.0 solo en el runner ([compose.ci.yaml](../compose.ci.yaml)) y los tres trabajos aprobaron, incluido el contrato con el hospital. Local y VM siguen con MongoDB 8.
- **Circuit breaker de caché (HU-27).** Con Redis detenido, la ficha pasó de p50 155 ms a 1,6 ms con concurrencia 1, y el breaker se cerró solo al volver Redis ([evidencia](evidence/cache-breaker-2026-11-09.json)).
- **Tablero y alertas (HU-20, D11).** [compose.observability.yaml](../compose.observability.yaml) agrega Prometheus y Grafana con cinco alertas; hay [simulacro con captura](evidence/alert-drill-2026-11-09.json).
- **Revisión de interfaz reproducible.** [scripts/ui-review](../scripts/ui-review/review.mjs) recorre 33 pantallas contra cualquier URL y falla si encuentra problemas.
- **Guion.** [GUION-PRESENTACION](GUION-PRESENTACION.md) incluye el recorrido, los requisitos con su evidencia, preguntas probables y plan B.

Siguen pendientes la VM con URL pública y el acceso real del gimnasio y del hospital.

## 8. Frescura de búsqueda — 9 de noviembre de 2026

El recorrido `cmd/smoke` ahora mide alta, actualización de tarifa/versión y baja lógica desde antes de cada escritura hasta que la búsqueda refleja el cambio. En la [CI de la revisión `2dc9c61`](https://github.com/Gerpeirone/Car-Cruds/actions/runs/42745574484/job/129597135342) las tres transiciones cumplieron la cota de 5 s. El método, sus límites y la evidencia figuran en [D6](adr/ADR-006-busqueda.md#medición-de-operación-normal--9-de-noviembre-de-2026). Falta ampliar a carga sostenida y recuperación.

## 9. Criterio de cierre de evidencia del sistema

El requisito «decisiones verificadas mediante evidencia del sistema» se considera cumplido en una revisión únicamente cuando coinciden estos cuatro controles:

1. contratos OpenAPI, ejemplos, inventario de `API.md` y rutas implementadas sin diferencias;
2. pruebas y análisis estático del backend sin fallos ni omisiones de las integraciones configuradas;
3. Compose con dependencias reales, recorrido persistente, caché fría/caliente/fallback y frescura de Solr bajo carga satisfactorios;
4. pruebas, build y revisión reproducible del frontend satisfactorios.

La evidencia histórica conserva su revisión y alcance. No se traslada automáticamente a un commit nuevo: la ejecución de CI de esa revisión debe quedar enlazada aquí antes de declarar el punto cerrado. La publicación cloud y las pruebas con equipos externos siguen siendo requisitos separados.

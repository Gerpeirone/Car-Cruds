# Preparación de la entrega 2 — dominio propio de punta a punta

| Campo | Valor |
| --- | --- |
| Proyecto | Car Cruds |
| Versión documental | 0.2 |
| Fecha de revisión | 10 de octubre de 2026 |
| Fecha de entrega | Viernes 23 de octubre de 2026 |
| Estado | Documentación de diseño; implementación de entrega 2 pendiente |
| Referencia | [Enunciado](../Enunciado%20TP%20Final.md), sección 6.3 y requisitos del sistema |

## 1. Cambios a partir del feedback

Se conserva el negocio de alquiler de autos y se incorpora el microservicio de Empleados para gestión administrativa de personal. El Load Balancer aparece como componente explícito, con dos réplicas de Flota previstas. El gimnasio consumirá empleados y devolverá uso de membresías; Car Cruds consumirá turnos de una clínica para revisiones de posibles incorporaciones.

Se documentan épicas, historias, requerimientos y patrones antes de programar. Este trabajo actual es documental. El único ejecutable sigue siendo el mock de reservas de la entrega 1; no hay nuevos servicios, persistencia, Load Balancer o integraciones operativas.

## 2. Documentación preparada

| Artefacto | Finalidad |
| --- | --- |
| [SPEC.md](../SPEC.md) | Alcance actualizado, actores y RN-01 a RN-20 |
| [BACKLOG.md](BACKLOG.md) | Épicas, historias, prioridades y criterios de aceptación |
| [REQUERIMIENTOS.md](REQUERIMIENTOS.md) | RF/RNF verificables y trazabilidad |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Contexto C4, contenedores, propiedad de datos y distribución prevista |
| [ADR](adr/README.md) | Estilos, patrones, alternativas y consecuencias; historia de reemplazos |
| [INTEGRACIONES.md](INTEGRACIONES.md) | Alcance comunicado, propuestas locales y EXT-01 a EXT-14 pendientes |
| [Contratos](contracts/README.md) | Borrador propio de lista de empleados y condiciones que faltan del proveedor |

## 3. Correspondencia con lo exigido el 23 de octubre

| Exigencia del enunciado | Preparación actual | Evidencia pendiente para acreditar el hito |
| --- | --- | --- |
| Al menos un servicio operativo, además de la funcionalidad compartida funcionando | Servicios y casos de uso definidos; mock histórico de reservas | Flujo propio real con servicios necesarios y capacidad de Empleados disponible al gimnasio |
| Al menos un almacenamiento integrado al flujo | Persistencia propuesta en D3 | Operación contra una base real y recuperación tras reinicio |
| Logs estructurados y correlacionados, primera traza distribuida | Estrategia inicial D11 | Logs/traza de una operación real entre componentes; no salida simulada |
| D2, D6, D7, D9, D10 y D12 | Propuestas documentadas | Aplicación y verificación de decisiones según el avance real |
| D13: capacidad y costos | Protocolo de medición | Carga ejecutada, límite identificado, alternativa de escalado y estimación de costos con fuente |
| Validación de D1, D3 y D5 | Registros revisados con Empleados e integraciones | Evidencia de límites, almacenamiento y comunicaciones implementados |
| Primera versión de D11 | Diseño de observabilidad | Instrumentación inicial del flujo principal |
| Actualización de D8 si cambió contrato | ADR-017 y borrador propio de empleados | Acuerdo, contrato revisado, servidor accesible y validación con el consumidor |

Un borrador procesable, una historia escrita o un directorio no equivalen a un servicio operativo. Los resultados de búsqueda, caché, carga, balanceo e integración permanecen pendientes; no se atribuyen al mock histórico ni a las demostraciones de las clases.

## 4. Orden de trabajo propuesto

1. Revisar alcance y prioridades de épicas/historias con el grupo y la cátedra; asignar responsables.
2. Cerrar los acuerdos indispensables EXT-01 a EXT-05 y EXT-07 a EXT-13 para las integraciones. EXT-06 solo bloquea montos; el informe de uso puede avanzar sin tarifas.
3. Validar patrones y dependencias de los cuatro servicios, el balanceador y la propiedad de datos.
4. Implementar un flujo propio de alquiler con persistencia y sus dependencias necesarias; implementar Empleados y la lista revisada para el consumidor.
5. Instrumentar logs correlacionados y una traza del flujo, agregar pruebas de integración y contrato sobre dependencias reales.
6. Implementar y demostrar turnos e informes conforme a los acuerdos, así como comunicación de uso del gimnasio. Registrar incertidumbre, fallas y duplicados.
7. Ejecutar las mediciones previstas; actualizar ADR, estado de historias y esta matriz con evidencia obtenida.

El orden es una propuesta de coordinación, no un cronograma de resultados ya alcanzados. Las historias del sistema completo incluyen funciones de presentación grupal y defensa que no se declaran obligatoriamente implementadas en este trabajo documental.

## 5. Criterio para comenzar una historia

Debe tener actor y valor claros, requerimientos vinculados, criterios de aceptación verificables, reglas y propietario de datos, dependencias identificadas y acuerdo de contrato si utiliza una API externa. Una duda que cambia el resultado esperado se conserva como pendiente y bloquea solo la parte dependiente.

## 6. Criterio para acreditar una implementación

- Código y comportamiento acordes con historia, requerimientos y ADR, revisados por el grupo.
- Pruebas pertinentes ejecutadas; las de integración utilizan una dependencia real y las de contrato la versión efectivamente consumida.
- Persistencia, errores y recuperación comprobados; logs/traza con evidencia del flujo.
- Instrucciones reproducibles, configuraciones sin secretos y documentación actualizada.
- Para la capacidad compartida: servidor accesible, contrato versionado y prueba con el consumidor, mantenido operativo hasta finalizar la evaluación.

Estos criterios no están cumplidos por la preparación documental actual. No se agrega un informe de postmortem ficticio ni resultados de carga: se producirán a partir de ensayos reales.

## 7. Referencia a la primera entrega

[ENTREGA-1.md](ENTREGA-1.md) conserva el control histórico. El contrato [openapi-v1.json](contracts/openapi-v1.json), el código y las pruebas del mock de alquileres conservan su comportamiento. La nueva integración no presenta ese mock como una implementación de Empleados.

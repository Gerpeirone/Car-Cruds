# Enunciado - Práctico Integrador 2026

**Arquitectura de Software - Facultad de Ingeniería.**

Versión Markdown compacta del enunciado académico, preparada el 8 de octubre de 2026 con el formato de las clases: títulos, listas y tablas. Se conserva la numeración y el contenido de los requisitos académicos; se eliminan encabezados repetidos, espacios de maquetación y saltos de página. Las notas de conversión se identifican al final.

## Propósito

- Diseñar, construir y poner en funcionamiento un sistema de **microservicios sobre un dominio nuevo**: testeado, observable, tolerante a fallas e integrado efectivamente con otro grupo.
- El trabajo recorre los contenidos de la materia. El enunciado establece **qué condiciones cumplir**, sin determinar cómo implementarlas. Cada grupo decide diseño, arquitectura y tecnologías, y debe **documentar, justificar y defender** sus decisiones en las evaluaciones.

## 1. Dominio

- Elegir un dominio distinto del utilizado en el TP de Desarrollo de Software y del elegido por los demás grupos. La propuesta requiere **aprobación de los profesores**.
- El sistema debe superar las operaciones CRUD: la acción principal involucra **restricciones, estados, validaciones o consecuencias relevantes para el negocio**.

| Dominio de ejemplo | Entidad principal | Acción principal | Característica relevante |
| --- | --- | --- | --- |
| Turnos médicos | Profesional / turno | Reservar un turno | Dos personas no pueden tomar el mismo turno; picos de demanda |

## 2. Requisitos de Arquitectura y Desarrollo

| Apartado | Requisitos |
| --- | --- |
| **2.1 Arquitectura de servicios** | Backend con **al menos tres microservicios y un API Gateway**. Al menos dos servicios utilizan **patrones de arquitectura diferentes**, trabajados durante la materia. |
| **2.2 Frontend** | Interfaz web que centralice la interacción del usuario y dé soporte a **todas las funcionalidades** del proyecto. |
| **2.3 Balanceo de carga** | Al menos un servicio con **dos o más instancias**, balanceo y detección de instancias no disponibles. La distribución de solicitudes y el estado de las instancias deben comprobarse mediante observabilidad. |
| **2.5 Comunicación síncrona y asíncrona** | Implementar ambas. Síncrona: definir **timeout, manejo de errores, respuestas tardías o ausentes y política de reintentos**. Asíncrona: mensajería con **publicación y consumo de un evento de dominio** y tratamiento de mensajes fallidos. |
| **2.6 Búsqueda** | Una entidad con búsqueda mediante un **motor de búsqueda**: paginación, filtros de dominio y al menos un ordenamiento. Mantener el índice sincronizado con los cambios y definir el **retraso máximo tolerable** desde la modificación de la fuente hasta su aparición en resultados. |
| **2.7 Caché** | Caché en al menos un flujo de lectura relevante, justificada por una **necesidad observable**, no solo por cumplimiento formal. Demostrar su impacto mediante **métricas**. |
| **2.8 Consistencia** | Elegir almacenamiento por servicio según accesos, volumen, consultas y garantías de consistencia. Usar **al menos dos tipos: relacional y no relacional**. Al menos una operación debe impedir **duplicación o pérdida de información** mediante garantías de consistencia. |
| **2.9 Resiliencia** | Identificar dependencias y comportamiento ante caída o lentitud. Implementar **al menos un mecanismo de protección frente a fallas**. Demostrarlo con una caída provocada y registrarlo en `POSTMORTEM.md`. |
| **2.10 Observabilidad** | Comprender el estado de una operación sin inspeccionar manualmente cada componente. Mínimo: **logs estructurados, métricas y tablero** de estado general. |
| **2.11 Testing** | **Unitarios** sobre reglas de negocio relevantes, superando el porcentaje que represente calidad; **integración por servicio** contra una dependencia real (almacenamiento o mensajería); **carga**, con resultados documentados y analizados. |
| **2.12 Puesta en marcha** | Código y documentación en un **único repositorio público**; versión final evaluable en la **rama principal**. Estrategia de ramas, revisiones e integración acorde al grupo. Arranque local **único, automatizado y documentado**, que cree/inicie dependencias, sin configuraciones manuales exclusivas de las computadoras de los integrantes. **Secretos y datos sensibles fuera del repositorio**. |

## 3. Documentación

La documentación forma parte del sistema y se evalúa junto al código. Debe mantenerse **actualizada durante el desarrollo**, no elaborarse únicamente al final.

| Archivo o artefacto | Contenido mínimo |
| --- | --- |
| `README.md` | Dominio, objetivo y flujo principal; ejecución local, acceso a la parte desplegada y ubicación de la documentación. |
| `SPEC.md` **o equivalente** | Funcionalidades, reglas de negocio y criterios de aceptación. Formato y ubicación según la metodología: especificación, historias de usuario, casos de uso, backlog u otros artefactos. **No se exige SPEC.md** si la documentación de la metodología cumple esa finalidad. |
| `docs/ARCHITECTURE.md` | Arquitectura con texto y diagramas: servicios, responsabilidades, datos propios, comunicaciones y distribución de componentes. |
| `docs/adr/ADR-XXX.md` | Decisiones obligatorias y otras relevantes, cada una en un archivo independiente, según el formato que el enunciado remite a la sección 8. |
| `docs/contracts/README.md` | Capacidad publicada, contrato versionado, versiones, ejemplos de uso, posibles errores e información suficiente para el consumidor. |
| `docs/POSTMORTEM.md` | Caída provocada: impacto, línea temporal, detección, respuesta, causa y acciones de mejora. |

## 4. Integración entre grupos

### 4.1 Organización de la integración

- Cada grupo debe **publicar y documentar una capacidad propia** y **consumir una capacidad de otro grupo**. Los profesores asignan las relaciones proveedor-consumidor.
- Publicar implica ofrecer una forma definida de uso, suficiente para integrar **sin explicaciones privadas**. La primera versión operativa debe estar en un **entorno accesible** y mantenerse operativa hasta finalizar la evaluación.
- Contrato **formal, versionado, estándar y procesable**, adecuado a la comunicación elegida, en `docs/contracts/`. Puede describir **API, eventos o ambos**.
- Contrato y documentación deben incluir, según corresponda: operaciones/eventos; entradas y resultados; errores e interpretación; condiciones de **autenticación, idempotencia y uso**; versión vigente; ejemplos suficientes para implementar y probar.
- La capacidad externa debe intervenir en un **flujo importante del negocio**. No son válidas una llamada aislada, una pantalla de prueba o una integración sin consecuencias de negocio.
- La llamada al proveedor sale del **microservicio responsable** de la funcionalidad: no del frontend ni a través del punto de entrada propio del sistema.

El consumidor trabaja a partir del contrato y la documentación publicados y debe:

1. Implementar la integración respetando el contrato.
2. Incluir un **test de contrato** que detecte cambios incompatibles.
3. Definir tiempos de espera y tratamiento de errores.
4. Decidir qué observa el usuario si el proveedor falla.
5. Demostrar ese comportamiento durante la jornada de fallas controladas.

La integración exige trabajar con un sistema que el grupo no controla y producir documentación utilizable por personas ajenas a las decisiones internas.

## 5. Registro y justificación de decisiones arquitectónicas

- Los **ADR** explican por qué se eligió una alternativa, qué otras se evaluaron y qué consecuencias se aceptaron.
- Registrarlos durante diseño e implementación y mantenerlos actualizados en el hito correspondiente. En principio, una decisión obligatoria por archivo; pueden agregarse otras relevantes.

| Decisión | Tema | Aspectos que deben justificarse |
| --- | --- | --- |
| **D1** | Límites de los servicios | Separación de responsabilidades, relaciones y propiedad de los datos. |
| **D2** | Arquitectura interna | Estilos/patrones, dependencias internas y adecuación al problema de cada servicio. |
| **D3** | Persistencia | Almacenamiento por servicio, patrones de acceso y limitaciones aceptadas. |
| **D4** | Consistencia y concurrencia | Operaciones con garantías especiales; concurrencia, duplicación, pérdida y fallas parciales. |
| **D5** | Comunicación | Interacciones síncronas/asíncronas, tiempos de espera, reintentos, eventos e idempotencia. |
| **D6** | Búsqueda | Datos indexados, consultas, actualización/reconstrucción del índice y retraso aceptable. |
| **D7** | Caché | Información temporal, invalidación, vigencia, comportamiento ante fallas y medición. |
| **D8** | Contrato propio | Diseño, publicación, compatibilidad y estrategia de versionado de la capacidad ofrecida. |
| **D9** | Consumo del proveedor | Incorporación de la capacidad, adaptación al contrato y comportamiento ante errores, cambios o indisponibilidad. |
| **D10** | Resiliencia | Protección ante fallas, limitación de su propagación y degradación controlada. |
| **D11** | Observabilidad | Logs, métricas, trazas y tableros; **objetivo de servicio y criterios de alerta**. |
| **D12** | Balanceo de carga | Servicio, distribución, disponibilidad y comportamiento ante caída de una instancia. |
| **D13** | Capacidad y costos | Resultados de carga, límite principal, alternativa de escalado y estimación de costos. |

Si una decisión cambia, **conservar la historia**: marcar el ADR anterior como reemplazado y registrar en el nuevo cuál sustituye y por qué se revisó.

## 6. Cronograma y Entregables

Todas las fechas corresponden a **2026**.

### 6.1 Viernes 2 de octubre - Instancia Inicial

- Integrantes confirmados.
- Dominio y alcance preliminar aprobados.
- Repositorio creado y accesible a la cátedra.

### 6.2 Viernes 9 de octubre - Entrega 1: Diseño y contrato con mock

- Primera versión de **README**, documentación de alcance según metodología y **docs/ARCHITECTURE.md**.
- Diagramas de **contexto y contenedores**.
- Límites preliminares de servicios, responsabilidades y propiedad de datos.
- Capacidad propia seleccionada y documentada.
- Estructura inicial de servicios y dependencias.
- **D1 y D8**, más versiones iniciales de **D3 y D5**.

### 6.3 Viernes 23 de octubre - Entrega 2: Dominio propio de punta a punta

- **Al menos un servicio operativo**, además de la funcionalidad compartida funcionando.
- **Al menos un tipo de almacenamiento** integrado al flujo.
- **Logs estructurados y correlacionados** y una primera **traza distribuida** del flujo principal.
- **D2, D6, D7, D9, D10, D12 y D13**; validación de **D1, D3 y D5**; primera versión de **D11**; actualizar **D8** si cambió el contrato.

### Miércoles 11 o viernes 13 de noviembre - Presentación Grupal

La fecha de cada grupo se **sorteará y comunicará el 6 de noviembre**. Incluirá:

- Frontend completo, desde el ingreso hasta la funcionalidad compartida.
- Entrada única con **autenticación, direccionamiento y control de tráfico**.
- **Al menos tres servicios operativos**.
- Capacidad propia **real**, desplegada en la **nube con URL pública**.
- Capacidad del proveedor incorporada al **flujo principal**.
- **Al menos dos tipos de almacenamiento** integrados.
- Caché en lectura relevante, con **comparación antes/después**.
- **Dos estilos o patrones internos** aplicados de forma reconocible.
- Evento de dominio publicado/consumido por mensajería; consumidor **idempotente** y tratamiento de mensajes no procesables.
- Búsqueda indexada con paginación, filtros, ordenamiento y actualización/reconstrucción.
- Decisiones verificadas mediante **evidencia del sistema**.
- **Limitaciones conocidas y deuda técnica aceptada**, actualizadas en `docs/ARCHITECTURE.md`.

### Defensa Individual - Examen Final

- Operación crítica con garantías de **consistencia y concurrencia**.
- Tests unitarios y un test de integración contra una **dependencia real**.
- **Test de contrato** de la capacidad consumida.
- Balanceo operativo con **dos o más instancias**, verificación de disponibilidad y evidencia de distribución.
- Simulación de caída de dependencia propia, proveedor externo o ambos.
- Test de carga ejecutado: resultados, análisis de capacidad y límite principal.
- Comportamiento documentado ante caída de cada dependencia relevante y **al menos una protección implementada**.
- **Logs, métricas y trazas completos**, tablero y alerta para la presentación.
- Ensayo de caída controlada e informe **`docs/POSTMORTEM.md`**.

## 7. Bonus

- **Hasta dos desafíos**, con **hasta un punto por desafío**.
- Anunciarlos en la **Entrega 2** e incorporarlos al alcance documentado.
- Para obtener puntaje deben estar **integrados, documentados, testeados y defendidos**.
- Alternativas: agente que ejecute operaciones mediante un protocolo estándar; actualizaciones en tiempo real; capa de agregación adaptada a la interfaz; orquestación de contenedores; tests E2E; escalado automático.

## Notas de conversión

- Se conserva el salto de **2.3 a 2.5**: el enunciado recibido no contiene 2.4.
- El enunciado remite al formato ADR de una **sección 8 inexistente** en el documento recibido, que termina en la sección 7.
- No establece un porcentaje numérico de calidad/cobertura unitaria, una fecha para la defensa individual ni un formato concreto del mock de la Entrega 1.
- La obligación de integración **por servicio** de 2.11 permanece vigente, aunque el listado de defensa mencione un test en singular.
- Los nombres de herramientas, patrones y reglas de Car Cruds pertenecen a las decisiones del proyecto; este enunciado no los impone.

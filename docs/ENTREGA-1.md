# Entrega 1: diseño y contrato con mock

| Campo | Valor |
| --- | --- |
| Proyecto | Car Cruds |
| Versión documental | 0.1 |
| Fecha | 7 de octubre de 2026 |
| Estado | Documentación y mock disponibles para revisión |
| Presentación | Viernes 9 de octubre de 2026 |
| Referencia | Enunciado del TP Integrador, secciones 6.1 y 6.2, páginas 7 y 8 |

## 1. Matriz de entregables

| Requisito | Material preparado | Estado |
| --- | --- | --- |
| Primera versión del README | [Presentación del proyecto](../README.md) | Documentada |
| Documentación de alcance | [Especificación](../SPEC.md); organización del trabajo en README | Documentada |
| Descripción de arquitectura | [Arquitectura](ARCHITECTURE.md) | Documentada |
| Diagramas de contexto y contenedores | Figuras de arquitectura | Documentados |
| Límites, responsabilidades y propiedad de datos | Arquitectura y [D1](adr/ADR-001-limites-de-servicios.md) | Diseño preliminar registrado |
| Capacidad propia seleccionada | [Disponibilidad y reservas](contracts/README.md) | Contrato v1 documentado |
| Estructura inicial y dependencias | `services/`, `gateway/`, `frontend/` | Estructura y dependencias documentadas |
| Decisiones D1 y D8 | [D1](adr/ADR-001-limites-de-servicios.md) y [D8](adr/ADR-008-contrato-propio.md) | Registradas en ADR independientes |
| Versiones iniciales D3 y D5 | [D3](adr/ADR-003-persistencia.md) y [D5](adr/ADR-005-comunicacion.md) | Versiones iniciales registradas |
| Contrato y mock | [OpenAPI v1](contracts/openapi-v1.json) y [mock HTTP](../mocks/rentals/README.md) | Verificados localmente |

La capacidad compartida se presenta mediante un contrato HTTP procesable y un mock ejecutable. Esta combinación permite verificar entradas, respuestas, conflictos e idempotencia antes de implementar la persistencia y las dependencias reales.

## 2. Evidencia de verificación local

Fecha de ejecución: **7 de octubre de 2026**.

- `go -C mocks/rentals test -race ./...`: aprobado en Windows. Incluye 24 solicitudes concurrentes para un mismo auto/período, con una reserva ganadora y 23 conflictos; repetición idempotente, claves reutilizadas, cancelación, liberación, fechas, precio, autenticación y respuestas del contrato.
- Contrato validado formalmente con `openapi-spec-validator`: OpenAPI 3.0.3 y versión de API 1.0.0. Sus ejemplos también se comprobaron contra los esquemas correspondientes.
- Enlaces internos de la documentación revisados, sin destinos faltantes.
- Presentación de los documentos revisada en vista renderizada: tablas legibles y diagramas de contexto, contenedores y comunicaciones sin recortes ni superposición de etiquetas.
- Recorrido de demostración ejecutado en Windows PowerShell 5.1 y PowerShell 7: la repetición conserva el identificador, una reserva superpuesta devuelve `409 VEHICLE_UNAVAILABLE` y la cancelación libera el período.

La evidencia corresponde al mock local de la primera entrega. Las garantías con persistencia, múltiples instancias e integración externa requieren pruebas adicionales en los hitos de implementación.

## 3. Control previo a la presentación

- [ ] Incorporar nombres de integrantes.
- [x] Vincular el proyecto al repositorio público [Gerpeirone/Car-Cruds](https://github.com/Gerpeirone/Car-Cruds).
- [ ] Confirmar que dominio y alcance fueron aprobados por los profesores, como pide la instancia inicial del 2 de octubre.
- [ ] Revisar las convenciones del dominio: fechas por días, tarifa, duración y cancelación.
- [ ] Confirmar tecnologías y patrones internos vistos en la materia.
- [ ] Revisar y acordar D1/D8; conservar D3/D5 como versiones iniciales que se validarán al implementar.
- [ ] Probar el mock desde una copia del repositorio y demostrar los escenarios siguientes.
- [ ] Subir documentación, contrato, estructura y mock al repositorio accesible a la cátedra.
- [ ] Verificar las condiciones de acceso al mock que solicite la cátedra para este hito.

Los ADR utilizan una estructura uniforme de contexto, decisión, alternativas, consecuencias y validación. El enunciado remite a una plantilla en la sección 8 que no aparece en el documento disponible; corresponde verificar si la cátedra dispone de un formato complementario.

## 4. Recorrido de demostración

Duración de referencia: **5 minutos**. Las solicitudes están disponibles en [la guía del mock](../mocks/rentals/README.md).

| Paso | Tiempo | Contenido y resultado esperado |
| --- | --- | --- |
| 1 | 60 s | Presentar el dominio, la operación central y las restricciones de asignación |
| 2 | 60 s | Exponer contexto, contenedores, propietarios de datos y decisiones D1/D8 |
| 3 | 30 s | Consultar disponibilidad y comprobar el cálculo de tarifa por cantidad de días |
| 4 | 60 s | Crear una reserva y repetir con la misma clave: se conserva el mismo identificador |
| 5 | 45 s | Crear otra reserva superpuesta con clave distinta: se obtiene un conflicto |
| 6 | 45 s | Cancelar, repetir la cancelación y comprobar que el período queda libre |

## 5. Continuidad del proyecto

Los siguientes hitos incorporan persistencia real, interfaz web, integración con el proveedor asignado, publicación de la capacidad, pruebas contra dependencias reales, búsqueda sincronizada, mediciones de caché, balanceo, observabilidad, pruebas de carga y postmortem. La planificación se describe en [la especificación](../SPEC.md) y [la arquitectura](ARCHITECTURE.md).

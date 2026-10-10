# ADR-002 — Arquitectura interna y patrones de diseño

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D2: arquitectura interna |
| Versión | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto — no validado |
| Hito | Preparación documental de la Entrega 2 — 23 de octubre de 2026 |

## 1. Contexto

El teórico 8 distingue estilo arquitectónico, patrón de diseño, modelo de dominio y framework. La organización debe proteger las reglas de reserva y el modelo administrativo de los detalles de persistencia y proveedores. Clientes y Flota presentan principalmente gestión y lectura de perfiles o fichas.

## 2. Decisión

| Servicio | Estilo propuesto | Dependencias previstas y justificación |
| --- | --- | --- |
| Alquileres | Hexagonal | Casos de uso y reglas de período, precio y estados expresan puertos de repositorio, Clientes, Flota y eventos; adaptadores HTTP, persistencia y mensajería dependen de esos puertos |
| Empleados | Hexagonal | Casos de uso de personal e informes expresan puertos de repositorio y proveedor clínico; la traducción del contrato externo queda en un adaptador |
| Clientes | Capas | Controlador HTTP → servicio de aplicación → repositorio; separa validación de conductor y acceso a datos |
| Flota | Capas | Controlador → servicio → repositorios de catálogo/búsqueda; adaptadores de índice y caché encapsulan detalles técnicos |

En los servicios hexagonales, el núcleo no importa framework HTTP, driver de base ni tipos del proveedor. Un handler es un adaptador de entrada; una interfaz de persistencia o de Clínica es un puerto de salida. Alquileres no conserva un puerto de proveedor clínico: esa responsabilidad pertenece a Empleados.

### Patrones concretos

| Patrón | Aplicación prevista | Variación o problema que protege |
| --- | --- | --- |
| Repository | Persistencia por servicio; interfaz de catálogo en Flota y puertos de repositorio en Alquileres/Empleados | Sustituir almacén o usar un falso en unitarios sin exponer consultas del motor |
| Adapter | HTTP de entrada, almacenamiento, Solr y traducción de Clínica | Evitar que protocolos y modelos externos determinen las reglas internas |
| Inyección de dependencias | Ensamblado explícito al iniciar cada servicio, mediante constructores | Seleccionar adaptadores reales/de prueba; mantener dependencias visibles |
| Transactional Outbox | Cambios de Flota y finalización de alquiler | Conservar la intención de publicar junto al cambio durable |
| Idempotent Consumer | Historial de Clientes e indexador | Evitar repetir efectos de mensajes duplicados o desordenados |
| Cache-aside | Fichas de Flota en Redis | Optimizar lecturas repetidas conservando MongoDB como fuente |
| Circuit Breaker | Operaciones dependientes de Clínica y validaciones HTTP | Limitar espera y propagación de una caída, junto con timeout |

Estos patrones no constituyen siete estilos arquitectónicos. Capas y Hexagonal son los dos estilos diferentes propuestos; su aplicación reconocible se verificará en las dependencias del código, no en nombres de carpetas.

## 3. Alternativas consideradas

Capas para todo reduce estructura, pero facilita filtrar detalles de Clínica a informes y reglas de reserva. Hexagonal para todo agrega interfaces y mapeos donde las capas ya resultan claras. CQRS completo, Event Sourcing y programación reactiva no se proponen inicialmente: no hay mediciones ni necesidad de reconstrucción que justifiquen sus costos. Un índice derivado por eventos no implica Event Sourcing.

## 4. Consecuencias y validación

Se aceptan interfaces y conversiones en las fronteras que cambian. DI no garantiza por sí sola el aislamiento. Revisar imports/dependencias, mantener reglas fuera de controladores y comprobar casos de uso con falsos; probar adaptadores contra dependencias reales y contrato clínico una vez publicado. No hay implementación de estos estilos en esta preparación documental.

## 5. Referencias e historial

Material en `clases.zip`: `teoria/08-estilos-arquitectura-backend.md`, `practica/clase-2-mongodb-repository-pattern.md`, `practica/clase-3-cache.md` y `practica/clase-5-solr.md`. [D1 vigente](ADR-014-limites-servicios-v2.md).

| Versión | Fecha | Cambio |
| --- | --- | --- |
| 0.2 | 10 de octubre de 2026 | Primera propuesta de estilos y patrones para cuatro servicios |

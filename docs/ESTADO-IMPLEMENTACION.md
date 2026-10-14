# Estado de implementación de la entrega 2

Fecha de corte: 13 de octubre de 2026. Rama de trabajo: `features`.

La implementación se interrumpió a solicitud del integrante responsable para guardar el avance. Este estado es parcial y requiere completar las verificaciones antes de proponer su integración en `main`.

## Avance registrado

- Servicios propios de Clientes, Flota, Alquileres y Empleados, con persistencia y pruebas por componente.
- Gateway con autenticación por roles; balanceador NGINX para dos instancias de Flota.
- Infraestructura Docker Compose con PostgreSQL, MongoDB, Redis, RabbitMQ, Solr y Jaeger.
- Reservas con control de concurrencia e idempotencia; publicación mediante outbox y consumidores con deduplicación.
- Separación de empleados y candidatos. Lista propia de empleados activos y contrato OpenAPI provisional.
- Instrumentación de registros JSON, métricas y trazas; su recorrido completo todavía no fue comprobado.
- Generación local de credenciales con `go run ./cmd/devsetup`. El archivo `.env` permanece fuera de Git.

## Verificaciones realizadas

Se ejecutaron pruebas de persistencia y concurrencia contra PostgreSQL y MongoDB, de coherencia de caché contra Redis y de actualización de búsqueda contra Solr. Las pruebas dirigidas por componente con detector de carreras y las verificaciones de publicación AMQP con escritura bloqueada resultaron satisfactorias.

Las imágenes de los servicios se construyeron, pero esa construcción precedió a las últimas correcciones del transporte AMQP. Deben reconstruirse antes de validar el estado guardado. No se presenta este avance como un sistema completo validado ni como evidencia de aceptación académica.

## Trabajo pendiente al retomar

1. Rechazar respuestas de dependencias de Alquileres que contengan JSON seguido de datos adicionales; comprobar también el límite de tamaño del cuerpo.
2. Reconstruir las imágenes y verificar el recorrido por gateway y balanceador, los roles, las reservas y el procesamiento de eventos entre servicios.
3. Verificar trazas distribuidas reales, correlación de registros y métricas; completar las pruebas de recuperación y persistencia del conjunto.
4. Incorporar pruebas reproducibles del conjunto y resultados iniciales de carga, límites observados, alternativa de escalamiento y estimación de costos con fuentes oficiales.
5. Completar el contrato de la API implementada de Alquileres, la documentación de ejecución y la matriz de cobertura académica. Actualizar SPEC, arquitectura, backlog y ADR según evidencia real.
6. Fijar también la imagen de NGINX por digest y completar la revisión de reproducibilidad del despliegue.
7. Presentar la propuesta al equipo mediante pull request y obtener aprobación antes de integrar en `main`.

Los contratos y accesos externos de clínica y gimnasio permanecen pendientes por indicación del equipo. No se implementaron integraciones reales ni se atribuyeron resultados a esos proveedores.

## Ejecución prevista para la próxima sesión

Se requiere Go 1.26 y Docker Desktop en funcionamiento. Generar `.env` con `go run ./cmd/devsetup` si todavía no existe y ejecutar `docker compose up -d --build --wait`. La entrada local prevista es `http://localhost:8080` y la interfaz de Jaeger es `http://localhost:16686`.

Los datos locales se almacenan en volúmenes Docker. Conservarlos al detener o reiniciar los servicios. Las pruebas de integración utilizan variables `TEST_*` explícitas y recursos aislados; una ejecución sin esas variables no demuestra integración con bases de datos reales.

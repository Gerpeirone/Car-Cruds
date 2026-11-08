# Visor Swagger de las API propias

La guía de uso, el inventario completo y los flujos están en [API.md](../API.md). Desde la raíz, ejecutar `go run ./cmd/apidocs` y abrir `http://127.0.0.1:8090/api/`.

`catalog.json` selecciona los seis contratos actuales. `index.html` y `swagger-initializer.js` configuran Swagger UI como visor de lectura: `supportedSubmitMethods=[]`, validación remota deshabilitada, autorización sin persistencia y configuración por query deshabilitada. El proceso no consulta `.env`, no alcanza el gateway y solo publica una lista explícita de recursos documentales. No debe confundirse con el frontend del negocio ni con una publicación de la capacidad externa.

Los archivos de `vendor/` provienen sin modificaciones de `swagger-ui-dist` **5.34.0**, distribución oficial publicada en npm. [provenance.json](vendor/provenance.json) registra URL, integridad del paquete y hashes SHA-256. Se conservan [LICENSE](vendor/LICENSE) y [NOTICE](vendor/NOTICE). Para actualizar: obtener una versión explícita del paquete oficial, verificar su integridad npm, copiar únicamente los tres recursos de UI y los avisos legales, renovar la procedencia y ejecutar la validación documental y la inspección del visor. No editar los archivos de terceros para cambiar la presentación propia.

Los requisitos Python se usan exclusivamente para validar los contratos y los ejemplos; no son necesarios para abrir el visor. CI crea un entorno temporal, instala [requirements.txt](requirements.txt) y ejecuta [check-api-docs.py](../../scripts/check-api-docs.py). Las pruebas Go verifican que el catálogo y los recursos se sirven y que no se publican rutas de negocio, fuentes, borradores o directorios.

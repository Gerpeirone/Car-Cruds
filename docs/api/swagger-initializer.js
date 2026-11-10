"use strict";

window.addEventListener("load", async () => {
  try {
    const response = await fetch("./catalog.json");
    if (!response.ok) throw new Error("No se pudo cargar el catálogo de contratos");
    const catalog = await response.json();
    window.ui = SwaggerUIBundle({
      urls: catalog.map(entry => ({ name: entry.name, url: "../contracts/" + entry.file })),
      dom_id: "#swagger-ui",
      deepLinking: true,
      filter: true,
      displayOperationId: true,
      defaultModelsExpandDepth: 1,
      docExpansion: "list",
      supportedSubmitMethods: [],
      validatorUrl: null,
      persistAuthorization: false,
      queryConfigEnabled: false,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
      layout: "StandaloneLayout"
    });
  } catch (error) {
    const notice = document.createElement("p");
    notice.className = "status";
    notice.textContent = "No se pudo iniciar Swagger UI. Inicie el visor con go run ./cmd/apidocs y recargue esta página. " + error.message;
    document.getElementById("swagger-ui").appendChild(notice);
  }
});

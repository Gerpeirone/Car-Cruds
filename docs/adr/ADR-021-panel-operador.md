# ADR-021 — Panel del operador y proyección de acceso

Fecha: 8 de noviembre de 2026. Estado: implementado en la entrega 3 (PR #8); aceptación del equipo y revisión visual pendientes. Complementa D2, D5, D8 y D11; conserva ADR-020 y los patrones internos del backend.

## Contexto

F3-02 requiere operar flota, clientes y alquileres desde la web. El backend usa credenciales opacas de aplicación, sin un login personal. El perfil de cliente no prueba un rol administrativo; una lista de clientes admite también service. Validar el panel con una consulta de personal traería información ajena a su función.

## Decisión

Agregar `GET /v1/session` exclusivamente en el gateway, detrás de la autenticación existente. Devuelve el rol establecido por el servidor y el ID de cliente cuando aplica, con `Cache-Control: no-store`, sin tokens ni perfil personal. Es una ampliación compatible del contrato técnico 0.2.0, no una nueva autoridad de identidad ni una sesión persistente. El panel admite únicamente `admin`; los servicios mantienen sus permisos de recursos.

Separar la interfaz en `frontend/src/operator/` con contexto y token de memoria propios, rutas y estilos del operador, flota, clientes, alquileres y componentes compartidos. El frontend del cliente conserva su recorrido. Entrar a operador descarta el contexto de cliente y la salida descarta el token administrativo.

No agregar endpoints de listado de reservas ni CRUD de clientes que el backend no ofrece. Los comandos pasan por el gateway existente. Los conflictos de versión conservan el borrador y requieren decisión explícita; los POST no se reintentan automáticamente. La creación de mantenimiento carece de idempotencia/consulta: su referencia local incierta permanece bloqueada tras recarga hasta revisión del equipo.

## Alternativas y consecuencias

| Alternativa | Resultado |
| --- | --- |
| Confiar en un selector «operador» de la UI | No acredita identidad; descartada |
| Probar el acceso mediante recursos de personal | Acoplamiento y exposición innecesaria; descartada |
| Crear autenticación personal completa | Excede el acceso de laboratorio existente; requiere diseño propio posterior |
| Proyección mínima del acceso autenticado | Elegida: no amplía privilegios y permite verificar el rol administrativo sin leer datos de negocio |

Se incorpora una operación HTTP técnica y su inventario/contrato/pruebas. Recargar exige autenticarse nuevamente. La gestión de credenciales y el aislamiento del entorno continúan siendo de laboratorio; no se presenta una identidad personal de producción. Permanecen las limitaciones de consulta/mantenimiento de las API actuales.

## Evidencia y material consultado

[PANEL-OPERADOR](../PANEL-OPERADOR.md) registra verificaciones y límites. Pruebas con carreras de gateway, contrato OpenAPI, 36 casos ordinarios de UI y recorrido del panel contra servicios reales aprobados. La inspección visual en navegador no está acreditada.

Material de `clases.zip`: `teoria/06-gateway-lb-discovery.md` (identidad y autorización distribuida) y `teoria/08-estilos-arquitectura-backend.md` (separación de reglas y adaptadores); [arquitectura](../ARCHITECTURE.md), [backlog](../BACKLOG.md) y [requerimientos](../REQUERIMIENTOS.md). No se modifica la autoridad transaccional de los servicios.

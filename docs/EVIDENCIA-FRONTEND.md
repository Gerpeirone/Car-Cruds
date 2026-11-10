# Frontend del alquiler — implementación y evidencia

Fecha del informe: 8 de noviembre de 2026. Rama: `feature/entrega-3`. Alcance: F3-01 de [ENTREGA-3](ENTREGA-3.md).

[Resumen estructurado de esta verificación](evidence/frontend-validation-2026-11-08.json).

## 1. Implementado

Interfaz React/TypeScript/Vite con acceso de laboratorio, catálogo/búsqueda, filtros/paginación, ficha, fechas y disponibilidad, resumen, reserva idempotente, recuperación de resultado incierto después de recargar, consulta por código y cancelación con diálogo. El token se conserva exclusivamente en memoria; sessionStorage guarda solo la operación pendiente necesaria para recuperarla.

Se agregaron estados de carga, vacío, errores y conflictos. La navegación protege el acceso y conserva enlaces directos después del ingreso. Se descartan respuestas de búsquedas/fechas anteriores. El total se toma de Alquileres, se verifica como entero seguro y se invalida al modificar vehículo o fechas. Una reserva recuperada se consulta por GET para mostrar su estado actual.

Diseño con fondo claro, verdes y acento lima, ilustración vectorial neutra propia y tipografías Manrope/DM Sans servidas localmente mediante Fontsource. No se incluyen fotos ficticias de vehículos ni datos comerciales inventados. Las reglas CSS adaptan navegación, formularios, tarjetas y resumen a móvil; se implementan etiquetas, foco, diálogo nativo, mensajes accesibles y preferencia de movimiento reducido.

## 2. Verificaciones ejecutadas

| Comprobación | Resultado y límite |
| --- | --- |
| Tipos y build | `npm run build` satisfactorio; salida estática en `frontend/dist`, ignorada por Git |
| Pruebas automatizadas | 24 pruebas de fechas/importes, transporte y comportamiento React aprobadas con jsdom; no son una inspección visual de navegador |
| Prueba React contra servicios reales | `live.test.tsx` aprobó con gateway, PostgreSQL, MongoDB, Redis, Solr y servicios propios. Creó un vehículo ficticio exclusivo, reservó, provocó pérdida de respuesta después del commit, recuperó tras volver a montar la aplicación y entrar, comprobó el mismo ID y canceló. El fixture quedó inactivo y la reserva cancelada |
| Idempotencia real | Dos POST con exactamente el mismo cuerpo/clave recuperaron la misma reserva persistida; se verificó su estado antes y después de cancelar |
| Docker | Build multietapa del frontend y NGINX saludables con `compose.frontend.yaml` |
| Entrada estática y API | Ruta web `/autos`: 200; API sin credencial: 401; `/internal/ready`: 404; asset inexistente: 404. Se verificó CSP del servidor estático |
| Contratos | La UI usa las rutas existentes de clientes, flota y alquileres; no necesita nuevos endpoints de negocio |

La suite normal omite intencionalmente una prueba adicional de integración que necesita entorno/credenciales explícitos; esa prueba se ejecutó por separado contra el backend. Los 24 casos ordinarios y el caso real se registran por separado. Las claves de fixtures de laboratorio no constituyen credenciales de producción. No se contactó a gimnasio ni hospital.

## 3. Cobertura de aceptación

| Criterio | Evidencia |
| --- | --- |
| FE-A01 | Acceso y salida con perfil; el montaje nuevo exige volver a ingresar; almacenamiento sin token |
| FE-A02 | Búsqueda, descarte de respuesta anterior y alternativa explícita de catálogo cuando falla Solr en prueba controlada |
| FE-A03 | Pruebas de fechas límite/inválidas y de invalidación de cotización al editar fechas |
| FE-A04 | Creación/cancelación con contrato correcto y doble clic con un único POST en prueba controlada |
| FE-A05 | Pérdida de respuesta y recuperación tras recarga en prueba controlada y contra persistencia real |
| FE-A06 | Cambio de precio impide confirmar con la cotización anterior; conflicto de disponibilidad tiene tratamiento específico; su inspección visual sigue pendiente |
| FE-A07 | Consulta por código y ausencia de exposición de una reserva ajena con respuesta 404 controlada; autorización real sigue a cargo del backend |
| FE-A08 | Diálogo explícito y cancelación; estados ACTIVE y COMPLETED sin acción de cancelación; recorrido real hasta CANCELLED |
| FE-A09 | Pruebas de red/respuesta inválida/correlación; errores diferenciados de vacío y de escritura incierta |
| FE-A10 | Estilos adaptables y mecanismos de teclado/foco implementados; **revisión visual móvil, contraste y recorrido de teclado en navegador pendientes** |

## 4. Límite de la revisión visual

La herramienta de Computer Use no pudo adjuntar el navegador integrado y luego detuvo el acceso a Chrome por no poder determinar con confianza la URL actual en Windows. No se eludió ese bloqueo. No se dispone de capturas verificadas ni se declara aprobado el recorrido visual en navegador. jsdom valida comportamiento de componentes y API, no layout ni renderizado real.

## 5. Ejecución y pendientes

[frontend/README](../frontend/README.md) contiene instalación, ejecución y prueba real. El workflow `frontend.yml` ejecuta instalación reproducible, pruebas ordinarias y build; su resultado remoto debe consultarse sobre el commit publicado.

Continúan pendientes las pantallas administrativas y de personal, la publicación cloud y la identidad personal de producción. El Caddyfile público actual todavía no publica la SPA: la superposición `compose.frontend.yaml` ofrece una entrada local independiente con API del mismo origen. Este corte no modifica servicios de negocio, acuerdos externos ni el fallo histórico de MongoDB en CI.

## Aclaración del acceso

Se distinguió la referencia de soporte de los errores HTTP de un código de reserva y se agregó ayuda para pegar la credencial sin prefijos. Un caso de regresión 401 comprueba que esa referencia no se presente como una reserva. La suite actual aprueba 25 pruebas ordinarias; la prueba real previamente registrada conserva su alcance. Build satisfactorio.

## 6. Revisión en navegador real — 9 de noviembre

Se cerró el pendiente de la sección 4 con un navegador renderizado: Microsoft Edge en modo headless, controlado con `playwright-core`, contra la entrada pública local (Caddy con HTTPS interno, `PUBLIC_HOST=localhost`), que deriva `/v1/*` al gateway y el resto a la interfaz. Se recorrieron 33 pantallas: cliente (acceso, catálogo, ficha, reservas), operador (resumen, flota, clientes, reservas, mantenimiento) y personal (lista, alta, ficha, membresías, turnos, detalle de turno), en escritorio de 1366×900 y en móvil de 390×844, incluido el menú lateral abierto en móvil.

| Control | Resultado inicial | Resultado final |
| --- | --- | --- |
| axe-core 4.11, reglas WCAG 2.0/2.1 A y AA | Contraste insuficiente en todas las pantallas (unos 300 nodos) y tablas desplazables sin foco en móvil | **0 violaciones** en las 33 pantallas |
| Scroll horizontal (`scrollWidth − innerWidth`) | 0 px | 0 px |
| Errores de consola | Ninguno | Ninguno |
| Recorrido con Tab (acceso de cliente, acceso y resumen del operador) | Orden lógico, enlace «Saltar al contenido» y foco visible (contorno de 3 px) en cada elemento | Sin cambios |

Correcciones aplicadas:

- Los colores de texto secundario que fallaban se oscurecieron sin cambiar el tono, hasta llegar a 4,5:1 contra el fondo más claro donde aparecen (o 3:1 en texto grande). Incluye la variable `--muted` (`#738075` → `#677369`). Solo se tocaron propiedades `color`, no fondos ni bordes.
- La etiqueta de la ilustración en la ficha medía 6 px en móvil: pasó a 9 px con mayor contraste.
- Los contenedores de tablas con desplazamiento horizontal son enfocables (`tabIndex=0`, `role="region"` y etiqueta), para recorrerlos con teclado.

El informe resumido está en [ui-review-2026-11-09.json](evidence/ui-review-2026-11-09.json). Las credenciales se pasaron por variables de entorno y no figuran en el informe. Para inyectar axe, la auditoría desactivó la CSP solo en el navegador de prueba. La política del sitio no se modificó.

Límites: axe detecta en forma automática alrededor de la mitad de los problemas de accesibilidad. No se probó con lector de pantalla ni en Safari o Firefox. El selector nativo de fecha y hora se muestra según el idioma del navegador.

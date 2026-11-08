# Entrega 3 — preparación de la presentación grupal

| Campo | Valor |
| --- | --- |
| Fecha | 8 de noviembre de 2026 |
| Rama | `feature/entrega-3` |
| Base | `main`, `19c4e06` |
| Estado | F3-01 y F3-02 implementados y probados localmente; revisión visual y publicación pendientes |
| Referencia académica | [Enunciado](../Enunciado%20TP%20Final.md), presentación grupal |

## 1. Objetivo y alcance del hito

Se utiliza «entrega 3» como nombre de trabajo para la presentación grupal del 11 o 13 de noviembre, según la fecha asignada por la cátedra. El enunciado la denomina presentación grupal. Este documento organiza el avance y no declara cumplimiento de la presentación completa.

La presentación exige frontend completo, entrada única autenticada, servicios y almacenes operativos, capacidad propia publicada en cloud, consumo externo integrado y evidencia de caché, búsqueda, mensajería, patrones y limitaciones. El backend dispone de una base implementada; cada ampliación deberá verificarse sobre su revisión, sin trasladar automáticamente resultados históricos.

## 2. Etapas propuestas

| Etapa | Resultado | Estado |
| --- | --- | --- |
| F3-01: frontend del alquiler | Acceso de laboratorio, búsqueda, ficha, disponibilidad, reserva, consulta por código y cancelación | Implementado; [evidencia](EVIDENCIA-FRONTEND.md), con revisión visual pendiente |
| F3-02: operación administrativa | Flota, alta/consulta de clientes, reservas por código, retiro/devolución y mantenimiento | [Implementado y probado con servicios reales](PANEL-OPERADOR.md); revisión visual pendiente |
| F3-03: personal e integraciones | Empleados registrados, informes de membresías y seguimiento administrativo de turnos | Pendiente de especificación detallada y acuerdos operativos |
| F3-04: publicación y ensayo conjunto | Entrada HTTPS, interfaz publicada y acceso real de gimnasio/hospital | Requiere infraestructura, credenciales y coordinación |
| F3-05: evidencia final | Recorrido completo, mediciones comparativas y limitaciones actualizadas | Pendiente de ejecución sobre el conjunto final |

F3-01 incluye ahora código de interfaz, dependencias fijadas, build, pruebas y una entrada estática local con NGINX. Se conservan los endpoints de negocio existentes. No se incorporan pagos ni funciones ajenas al alquiler documentado.

## 3. Cierre esperado de F3-01

Un cliente de laboratorio podrá entrar con la credencial asignada, buscar un auto, seleccionar fechas, obtener disponibilidad y total desde Alquileres, confirmar una única reserva y consultar o cancelar una reserva propia por código. El recorrido tendrá estados de carga, vacío, conflicto, error y resultado desconocido; será utilizable con teclado y en pantalla móvil.

La [especificación funcional](FRONTEND-ALQUILER.md) fija contratos, criterios y orden de implementación. [ADR-020](adr/ADR-020-frontend-alquiler.md) propone la tecnología, los límites de autenticación y la distribución. La implementación y sus pruebas actuales se registran en [EVIDENCIA-FRONTEND](EVIDENCIA-FRONTEND.md); la presentación completa aún requiere las otras etapas.

## 4. Dependencias y pendientes

- El cliente de laboratorio debe existir, tener credencial propia y estar vinculado por `AUTH_CLIENT_CUSTOMER_ID`; la credencial se entrega por un canal privado. La interfaz no crea cuentas ni reemplaza la identidad del backend.
- Deben existir vehículos activos, un cliente habilitado y fechas vigentes para demostrar reservas. Los fixtures de pruebas se identificarán y limpiarán según su propietario.
- El fallo de arranque de MongoDB en la [CI de la base](https://github.com/Gerpeirone/Car-Cruds/actions/runs/42516259957) continúa pendiente de diagnóstico. La publicación de documentación no equivale a corregirlo.
- El acceso cloud y los acuerdos externos se coordinan en [DESPLIEGUE](DESPLIEGUE.md) e [INTEGRACIONES](INTEGRACIONES.md). No bloquean la documentación ni el desarrollo local de F3-01.
- La consolidación documental de E2 está propuesta en [PR #7](https://github.com/Gerpeirone/Car-Cruds/pull/7). No se integra esa propuesta en `main` ni se presupone su aceptación por iniciar esta rama.

## 5. Estado y verificación de esta etapa

El corte documental inicial se conserva en el historial. El incremento actual implementa F3-01: build, 24 pruebas ordinarias, prueba React contra API real y servidor estático Docker verificados. La herramienta de navegador se bloqueó por no poder verificar la URL en Windows; no se declara aprobado el layout ni el recorrido visual/teclado en navegador. Ver [evidencia y pendientes](EVIDENCIA-FRONTEND.md).

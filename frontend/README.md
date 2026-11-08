# Frontend web de Car Cruds

| Aspecto | Estado al 8 de noviembre de 2026 |
| --- | --- |
| Hito | Entrega 3: preparación de la presentación grupal |
| Primera etapa | F3-01: frontend del alquiler |
| Estado | Documentación previa; interfaz pendiente de implementación |
| Actor inicial | Cliente de laboratorio vinculado a un conductor existente |
| Tecnología propuesta | React, TypeScript, Vite y React Router; sin paquetes instalados en esta etapa |
| API | Rutas relativas `/v1/*`, siempre mediante el gateway |

## Documentación para empezar

- [ENTREGA-3](../docs/ENTREGA-3.md): etapas, dependencias y alcance del hito.
- [FRONTEND-ALQUILER](../docs/FRONTEND-ALQUILER.md): pantallas, navegación, contratos, errores y criterios FE-A01 a FE-A10.
- [ADR-020](../docs/adr/ADR-020-frontend-alquiler.md): decisión técnica, autenticación de laboratorio y distribución propuesta.
- [API vigente](../docs/API.md): endpoints, permisos y límites del backend.

## Primer recorrido previsto

Acceso de laboratorio → catálogo y búsqueda → ficha y fechas → disponibilidad y total → resumen → confirmación → consulta por código → cancelación antes del retiro.

El acceso actual usa una credencial de aplicación y consulta el perfil autorizado; no hay login de usuario/contraseña ni registro público. La API no lista reservas: la primera versión permite consultar una propia por su código. El cliente no usa credenciales administrativas, no llama al endpoint de cotización restringido y no conserva tokens en almacenamiento del navegador.

El precio y la disponibilidad provienen de Alquileres. Un timeout al confirmar deja un resultado incierto y se recupera con la misma clave y cuerpo. Cambiar fechas o auto invalida la cotización; una doble pulsación no crea otra operación.

## Implementación posterior

No hay todavía comandos de instalación, build ni ejecución del frontend aprobados. La próxima etapa creará el proyecto y fijará dependencias; después se implementarán acceso/catálogo, ficha/cotización, confirmación idempotente y consulta/cancelación, en ese orden.

Las pantallas de operador y personal, y las integraciones de gimnasio/hospital, pertenecen a etapas posteriores. La descripción de F3-01 no acredita el frontend completo exigido para la presentación.

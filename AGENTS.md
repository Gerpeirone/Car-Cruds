# Instrucciones del proyecto

## Repositorio y publicación

- Repositorio: https://github.com/Gerpeirone/Car-Cruds.
- Remoto local: `origin`. Rama principal: `main`.
- Existe autorización para registrar y subir los cambios del proyecto a GitHub al finalizar cada tarea. Esta preferencia se aplica a las tareas posteriores; no se requiere una nueva confirmación para cada publicación ordinaria.
- Revisar el estado local y el remoto, ejecutar las verificaciones pertinentes y crear commits con mensajes descriptivos antes de publicar la rama de trabajo.
- Conservar el historial y los cambios de otros integrantes. Si el remoto avanzó, integrar sus cambios antes de subir; no utilizar push forzado.
- Respetar las ramas y revisiones utilizadas por el grupo. La carga inicial de la entrega 1 se publica en `main`.

## Archivos versionados

- Versionar documentación, contrato, estructura de componentes, mock, pruebas y enunciado.
- Mantener fuera de Git los temporales de `tmp/`, las salidas de `output/`, los ejecutables, los logs y los archivos de entorno con secretos, según `.gitignore`.
- Mantener diferenciados el diseño previsto y los componentes efectivamente implementados. La documentación se presenta en español y con redacción formal.

## Material de cátedra

- Consultar `Enunciado TP Final.md` como referencia de los requisitos académicos del trabajo práctico.
- Las clases están en `clases.zip`: 17 archivos Markdown dentro de `teoria/` y `practica/`. Leer los temas pertinentes antes de definir o modificar arquitectura, tecnologías, contratos o implementación; constituyen el material teórico-práctico de referencia para la evaluación.
- Para patrones internos, consultar `teoria/08-estilos-arquitectura-backend.md` dentro del ZIP; para ejemplos de implementación, consultar las clases de `practica/` y su `README.md`.

## Diseño vigente de la entrega 2

- Consultar `SPEC.md`, `docs/BACKLOG.md`, `docs/REQUERIMIENTOS.md`, `docs/ARCHITECTURE.md` y el índice `docs/adr/README.md` antes de implementar; allí se identifica la propuesta vigente y los ADR históricos reemplazados.
- El diseño contempla Clientes, Flota, Alquileres y Empleados, además de gateway y Load Balancer. Las integraciones de gimnasio y clínica pertenecen a Empleados; los acuerdos externos pendientes se registran en `docs/INTEGRACIONES.md`.
- El único ejecutable actual es el mock histórico de reservas. El borrador de lista de empleados no es un contrato externo acordado ni un servicio operativo. Registrar evidencia real al implementar y no atribuir al diseño resultados de pruebas, tarifas o acuerdos inexistentes.

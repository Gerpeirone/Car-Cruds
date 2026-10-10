# ADR-017 — Capacidad compartida de Empleados

| Campo | Valor |
| --- | --- |
| Decisión del enunciado | D8: contrato propio |
| Versión documental | 0.2 |
| Fecha | 10 de octubre de 2026 |
| Estado | Propuesto; contrato no acordado ni implementado |
| Reemplaza | [ADR-008](ADR-008-contrato-propio.md) para la integración entre grupos |

## 1. Contexto

El grupo comunicó que el gimnasio consumirá una lista de empleados y devolverá uso/no uso de membresías para controlar gastos. La capacidad anterior de disponibilidad/reservas no satisface ese intercambio. Car Cruds también consumirá turnos de una clínica dentro del flujo de personal.

## 2. Decisión propuesta

Empleados publica la lista propia a través del gateway. Se prepara un [borrador OpenAPI](../contracts/employees-openapi-draft.json), versión 0.1.0, y [su documentación](../contracts/EMPLOYEES.md). Ruta, campos mínimos, población y paginación son propuestas internas para revisión; no se presentan como acuerdos alcanzados.

La identidad de aplicación del gimnasio tendrá permiso de lectura limitado, con autorización sobre los registros y datos compartidos. No se exponen candidatos, datos de clientes, credenciales o datos médicos. La población de empleados, las bajas y el identificador externo se acordarán expresamente.

La devolución del uso requiere un contrato independiente del listado: mecanismo, identidad, período, significado y reglas de repetición/corrección están pendientes. Empleados utiliza la evidencia para un informe administrativo y conserva incertidumbre cuando falta información. La lectura tiene una consecuencia de negocio en el control de beneficios, que deberá validarse y demostrarse con el grupo consumidor.

El contrato de reservas 1.0.0 y su mock se conservan como referencia histórica. La versión de la nueva API no renombra ni modifica ese contrato. La evolución compatible y las transiciones de versiones se acordarán antes de que exista un consumidor operativo; no se declara compatibilidad con un acuerdo todavía inexistente.

## 3. Alternativas

| Alternativa | Evaluación |
| --- | --- |
| Mantener reservas como capacidad para el gimnasio | No corresponde al intercambio comunicado |
| Acceso directo a la base de Empleados | Acopla esquemas y elude permisos y contrato |
| Publicar todos los datos del personal | Agrega datos innecesarios para el beneficio y dificulta controlar accesos |
| Acordar la lista y la devolución antes de formalizar el borrador | Reduce revisiones, pero retrasa la discusión; se elige un borrador claramente identificado para facilitarla |

## 4. Consecuencias y límites

El gimnasio puede integrar una capacidad administrativa sin acceder a bases internas. La empresa asume mantener el contrato y la disponibilidad publicados. Todavía no existen URL operativa, servidor de Empleados, credenciales o prueba de integración.

La lista por sí sola no determina cuánto debe pagar la empresa: las tarifas y reglas de imputación están pendientes. La separación de candidatos y empleados, el permiso de acceso y el significado del uso deben cerrarse antes de implementar.

## 5. Validación prevista

Revisar el borrador con el consumidor, validar formato y referencias, aportar ejemplos acordados, probar autenticación/población/paginación y compatibilidad y demostrar el informe con devolución real. No se ejecutaron estas verificaciones operativas.

Acuerdos pendientes y fuentes de referencia: [INTEGRACIONES.md](../INTEGRACIONES.md). Material de cátedra: teoría 02, 06 y 08; práctica 01 y 02 dentro de `clases.zip`.

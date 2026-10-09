# Documento de arquitectura — Car Cruds
| Campo | Valor |
| --- | --- |
| Versión | 0.1 |
| Fecha | 7 de octubre de 2026 |
| Estado | Diseño preliminar |
| Hito | Entrega 1 — 9 de octubre de 2026 |
| Registros relacionados | D1, D3, D5 y D8 en `docs/adr/` |

## 1. Propósito y alcance

Car Cruds es un sistema de alquiler de autos por período y con tarifa. Su arquitectura permite consultar vehículos, confirmar una reserva sin asignaciones superpuestas, conservar el precio acordado y controlar el retiro y la devolución.

Este documento define los límites de los servicios, la propiedad de los datos, las comunicaciones y la distribución prevista. La operación crítica es la confirmación de una reserva: debe resolver concurrencia y reintentos sin duplicar la asignación del auto.

### Estado de implementación de la versión 0.1

| Componente | Estado |
| --- | --- |
| Contrato y mock HTTP | Contrato versionado y simulación local en memoria de disponibilidad y reservas |
| Microservicios, gateway y frontend | Estructura inicial y responsabilidades documentadas |
| Persistencia, mensajería, búsqueda y caché | Diseño preliminar; implementación prevista en hitos posteriores |
| Despliegue e integración entre grupos | Entorno y participantes por definir |

## 2. Contexto del sistema

```mermaid
flowchart TB
    Client["Cliente<br/>[Persona]<br/>Busca vehículos y realiza reservas"]
    Operator["Operador<br/>[Persona]<br/>Administra flota, retiros y devoluciones"]

    System["Car Cruds<br/>[Sistema de software]<br/>Gestiona alquileres de autos"]

    Consumer["Grupo consumidor<br/>[Sistema externo]<br/>Consulta disponibilidad y gestiona reservas"]
    Provider["Grupo proveedor<br/>[Sistema externo]<br/>Provee una capacidad de negocio por acordar"]

    Client -->|"Busca y reserva mediante la interfaz web"| System
    Operator -->|"Administra flota y gestiona alquileres"| System
    Consumer -->|"Consulta disponibilidad y gestiona reservas [HTTPS/JSON]"| System
    System -->|"Consume capacidad externa [capacidad y protocolo por definir]"| Provider

    classDef person fill:#16324f,color:#ffffff,stroke:#16324f;
    classDef system fill:#2f80ed,color:#ffffff,stroke:#2f80ed;
    classDef external fill:#8a8a8a,color:#ffffff,stroke:#606060;

    class Client,Operator person;
    class System system;
    class Consumer,Provider external;
```

**Figura 1. Diagrama de contexto de Car Cruds — C4, nivel 1. El Cliente busca vehículos y realiza reservas. El Operador administra la flota y gestiona retiros y devoluciones. El grupo consumidor utiliza la capacidad publicada por Car Cruds y el grupo proveedor aporta una capacidad externa pendiente de acordar.
Leyenda: azul oscuro = persona; azul = sistema propio; gris = sistema externo. Las flechas indican dirección y propósito de la interacción. Los grupos externos serán asignados por la cátedra.

## 3. Contenedores y distribución de responsabilidades

```mermaid
flowchart TB
    Client["Cliente<br/>[Persona]"]
    Operator["Operador<br/>[Persona]"]
    Consumer["Grupo consumidor<br/>[Sistema externo]"]
    Provider["Grupo proveedor<br/>[Sistema externo]<br/>Capacidad por acordar"]

    subgraph System["Car Cruds — límite del sistema"]
        Web["Frontend web<br/>[Tecnología por definir]<br/>Interfaz de clientes y operadores"]

        GW["API Gateway<br/>[Tecnología por definir]<br/>Autentica y dirige solicitudes"]

        Customers["Servicio Clientes<br/>[API HTTP/JSON; tecnología por definir]<br/>Gestiona perfiles y habilitación del conductor"]

        Fleet["Servicio Flota<br/>[API HTTP/JSON; tecnología por definir]<br/>Gestiona vehículos, catálogo y tarifas"]

        Rentals["Servicio Alquileres<br/>[API HTTP/JSON; tecnología por definir]<br/>Gestiona disponibilidad, reservas, retiros y devoluciones"]

        CDB[("PostgreSQL Clientes<br/>[Base relacional propuesta]<br/>Perfiles, licencias e historial")]

        FDB[("MongoDB Flota<br/>[Base documental propuesta]<br/>Vehículos, características y tarifas")]

        RDB[("PostgreSQL Alquileres<br/>[Base relacional propuesta]<br/>Reservas, agenda, importes e idempotencia")]

        Broker["RabbitMQ<br/>[Broker propuesto]<br/>Distribuye eventos de dominio"]

        Indexer["Indexador de Flota<br/>[Worker propuesto; tecnología por definir]<br/>Actualiza el índice mediante eventos"]

        Search[("Solr<br/>[Motor de búsqueda propuesto]<br/>Índice de vehículos y tarifas")]

        Cache[("Redis<br/>[Caché propuesta]<br/>Fichas consultadas con frecuencia")]
    end

    Client -->|"Busca y reserva mediante la web [HTTPS]"| Web
    Operator -->|"Administra y opera mediante la web [HTTPS]"| Web

    Web -->|"Invoca funcionalidades [HTTPS/JSON]"| GW
    Consumer -->|"Consulta disponibilidad y gestiona reservas [HTTPS/JSON]"| GW

    GW -->|"Gestiona perfiles [HTTP/JSON]"| Customers
    GW -->|"Consulta y administra catálogo [HTTP/JSON]"| Fleet
    GW -->|"Gestiona disponibilidad y alquileres [HTTP/JSON]"| Rentals

    Customers -->|"Lee y escribe perfiles e historial [PostgreSQL/SQL]"| CDB
    Fleet -->|"Lee y escribe vehículos y tarifas [protocolo MongoDB]"| FDB
    Rentals -->|"Lee y escribe reservas y agenda [PostgreSQL/SQL]"| RDB

    Rentals -->|"Valida habilitación del conductor [HTTP/JSON]"| Customers
    Rentals -->|"Obtiene ficha y tarifa autoritativas [HTTP/JSON]"| Fleet
    Rentals -->|"Consume capacidad externa [protocolo por definir]"| Provider

    Rentals -.->|"Publica RentalCompleted.v1 [AMQP]"| Broker
    Fleet -.->|"Publica VehicleChanged.v1 [AMQP]"| Broker

    Broker -.->|"Entrega eventos para actualizar historial [AMQP]"| Customers
    Broker -.->|"Entrega cambios de vehículos [AMQP]"| Indexer

    Indexer -->|"Actualiza índice [HTTP/JSON]"| Search
    Fleet -->|"Busca con filtros y paginación [HTTP/JSON]"| Search
    Fleet -->|"Lee, escribe e invalida fichas [RESP]"| Cache

    classDef person fill:#16324f,color:#ffffff,stroke:#16324f;
    classDef external fill:#8a8a8a,color:#ffffff,stroke:#606060;
    classDef app fill:#2f80ed,color:#ffffff,stroke:#2f80ed;
    classDef data fill:#27ae60,color:#ffffff,stroke:#207c48;
    classDef support fill:#c98b2b,color:#ffffff,stroke:#91631e;

    class Client,Operator person;
    class Consumer,Provider external;
    class Web,GW,Customers,Fleet,Rentals,Indexer app;
    class CDB,FDB,RDB,Search,Cache data;
    class Broker support;
```
    
Figura 2. Diagrama de contenedores de Car Cruds — C4, nivel 2. Representa la arquitectura objetivo. Clientes gestiona perfiles y habilitación del conductor; Flota administra vehículos y tarifas; Alquileres controla la disponibilidad temporal, las reservas, los retiros y las devoluciones. Cada servicio es propietario de su almacenamiento.
Leyenda: azul oscuro = persona; gris = sistema externo; azul = aplicación, servicio o worker; verde = almacenamiento, índice o caché; ocre = mensajería. Flecha continua = interacción síncrona o acceso a datos; flecha discontinua = comunicación mediante eventos.
Estado del diseño: persistencia, RabbitMQ, Solr, Redis e indexador son propuestas para los próximos hitos. Las tecnologías pendientes se indican dentro de cada contenedor. El indexador pertenece a Flota y se representa como un worker desplegable propuesto. Los protocolos internos y la ubicación de la integración externa se confirmarán durante la implementación.



### Propiedad de los datos

| Servicio | Responsabilidad | Datos propios | Dependencias previstas |
| --- | --- | --- | --- |
| Clientes | Perfil y habilitación del conductor; historial proyectado | Cliente, licencia y estado, eventos procesados, historial de alquileres | PostgreSQL y mensajería |
| Flota | Características, catálogo y tarifa diaria vigente; búsqueda | Vehículo, tarifa, versión de ficha, eventos de cambio | MongoDB, motor de búsqueda, caché y mensajería |
| Alquileres | Precio acordado, reserva exclusiva, agenda, retiro y devolución | Reserva, alquiler, bloqueos de mantenimiento, importes congelados, idempotencia y outbox | PostgreSQL, Clientes, Flota, mensajería y proveedor externo |

**Alquileres es la única autoridad sobre la disponibilidad temporal.** Reservas y bloqueos de mantenimiento se validan dentro de su misma frontera transaccional. Flota conserva las características del auto y su habilitación administrativa para aparecer en el catálogo; la ocupación temporal corresponde a Alquileres.

Una modificación administrativa en Flota preserva las reservas existentes. Cualquier cambio que deba impedir una entrega requiere un comando explícito y validado en Alquileres. La coordinación de la baja de un auto con reservas futuras forma parte de los aspectos por resolver.

## 4. Arquitectura interna de los servicios

| Servicio | Patrón propuesto | Aplicación y justificación |
| --- | --- | --- |
| Alquileres | Arquitectura hexagonal | Aísla reglas de precio, solapamiento y estados de los adaptadores de persistencia, HTTP y mensajería. Los casos de uso utilizan puertos para Clientes, Flota, proveedor y eventos. |
| Clientes | Arquitectura en capas | Separa presentación HTTP, lógica de habilitación y acceso a datos para un modelo centrado en perfiles y validaciones. |
| Flota | Arquitectura en capas | Organiza gestión del catálogo y acceso a datos, con adaptadores de indexación y caché para las lecturas. |

En Alquileres, el dominio permanece independiente de HTTP y de los controladores de base de datos. En los servicios en capas, los controladores invocan la lógica de aplicación, que concentra las reglas y el acceso a persistencia. La búsqueda facilita la selección de un auto; la autorización de una reserva se realiza en Alquileres.

Los patrones se formalizarán en D2 durante la entrega 2 y se contrastarán con los trabajados en la materia. La estructura inicial reserva esas responsabilidades; la elección del lenguaje y del framework permanece abierta.

## 5. Confirmación de reserva y consistencia

1. El gateway autentica y dirige la solicitud a Alquileres, conservando la identidad de aplicación o usuario y el identificador de correlación.
2. Alquileres consulta la clave de idempotencia en el ámbito de la aplicación y la operación. Un reintento válido recupera la respuesta original antes de revalidar precio y agenda.
3. Obtiene la habilitación de Clientes y la ficha y tarifa de Flota con tiempos de espera limitados. La lectura utilizada para confirmar obtiene la tarifa de la fuente autoritativa y omite la caché, incluso con un TTL vigente.
4. Compara el importe esperado con el cálculo autorizado. Una diferencia genera un conflicto de precio.
5. En una transacción local, valida la exclusión por auto y período y registra la reserva, el precio acordado, la respuesta de idempotencia y el evento en outbox. Una restricción de exclusión de rangos o un mecanismo de bloqueo apropiado protege la operación concurrente; D4 determinará la solución concreta.
6. El publicador entrega el evento persistido a mensajería. Una indisponibilidad temporal del broker deja el evento pendiente y conserva la reserva confirmada.

La consistencia crítica se concentra en la transacción de Alquileres. Las validaciones de Clientes y Flota utilizan HTTP, sin una transacción distribuida entre los tres servicios. El precio congelado corresponde a la tarifa obtenida al confirmar; los cambios posteriores preservan ese acuerdo. La habilitación y las condiciones de entrega vuelven a verificarse en el retiro.

La exclusión de períodos protege la agenda planificada. Además, un alquiler activo impide otro retiro del mismo auto, aunque haya vencido su fecha de devolución. El transcurso del plazo previsto no finaliza automáticamente un alquiler.

El mock reemplaza las dependencias por datos de prueba y la transacción por un bloqueo de memoria. Permite verificar el comportamiento del contrato en un proceso. Las garantías entre múltiples instancias y la recuperación tras reinicio requieren la implementación y validación de la persistencia real.

## 6. Lecturas, proyecciones y caché

```mermaid
flowchart TB
    subgraph Services["Servicios de negocio"]
        Rentals["Alquileres"]
        Customers["Clientes"]
        Fleet["Flota"]
    end
    Broker["Mensajería<br/>RabbitMQ propuesto"]
    subgraph Reading["Estructuras de lectura de Flota"]
        Indexer["Indexador interno"]
        Search[("Índice<br/>Solr propuesto")]
        Cache[("Caché de fichas<br/>Redis propuesto")]
    end
    Rentals -->|HTTP: habilitación| Customers
    Rentals -->|HTTP: ficha y tarifa| Fleet
    Rentals -.->|RentalCompleted| Broker
    Fleet -.->|VehicleChanged| Broker
    Broker -.->|Historial| Customers
    Broker -.->|Cambio de ficha| Indexer
    Indexer -->|Actualiza| Search
    Fleet -->|Consulta| Search
    Fleet -->|Lectura de fichas| Cache
    classDef app fill:#eaf0f8,stroke:#526d91,color:#23364e
    classDef data fill:#f0f6f3,stroke:#52776a,color:#1f332b
    classDef support fill:#f7f3eb,stroke:#8c795b,color:#473a28
    class Rentals,Customers,Fleet,Indexer app
    class Search,Cache data
    class Broker support
```

**Figura 3. Comunicación y estructuras de lectura.** Las flechas continuas representan llamadas síncronas u operaciones sobre las estructuras de lectura; las discontinuas representan publicación y consumo de eventos. El indexador es un trabajador de Flota. La publicación utiliza outbox, según D3 y D5. La telemetría de gateway y servicios se describe en la sección 7.

### Búsqueda

Flota mantendrá un índice de características y tarifas. Las modificaciones publicarán `VehicleChanged.v1`; el indexador aplicará actualizaciones idempotentes por identificador y versión. Se establece como objetivo preliminar un retraso máximo de **5 segundos en operación normal**, medido desde el cambio en la fuente hasta su aparición en el índice. La reconstrucción utilizará los datos de Flota.

La búsqueda ofrecerá paginación, filtros y ordenamiento por tarifa. La disponibilidad por fechas se consultará a Alquileres. La actualización del índice se realizará a partir de cambios de datos, independientemente de las búsquedas de los usuarios.

### Caché

Se propone una caché de fichas consultadas con frecuencia, con TTL de 60 segundos e invalidación al modificar la ficha. Su impacto se verificará comparando latencia, accesos a almacenamiento y tasa de aciertos antes y después de incorporarla. La confirmación de una reserva utiliza la lectura autoritativa sin caché descrita en la sección 5.

Los objetivos de retraso y TTL se validarán mediante mediciones y se registrarán en D6 y D7. El índice y la caché son estructuras derivadas; las fuentes operativas conservan los datos de negocio.

## 7. Resiliencia, observabilidad y balanceo

### Comunicación y protección ante fallas

Se proponen tiempos de espera acotados, circuit breaker para dependencias HTTP y reintentos limitados ante errores transitorios de lectura. Una operación de resultado desconocido se recupera mediante la misma clave de idempotencia o mediante consulta de estado. Los mensajes fallidos tendrán reintentos limitados y una cola para mensajes no procesables. D5 especifica las políticas iniciales.

### Observabilidad

Los logs estructurados en JSON incluirán servicio, instancia, operación, duración, resultado y correlación. Las credenciales y los documentos completos del cliente se excluirán de los registros. Las trazas abarcarán el gateway y las comunicaciones entre servicios.

| Métrica prevista | Propósito |
| --- | --- |
| Latencia y errores por servicio | Detectar degradación de las operaciones |
| Conflictos de reserva | Observar el funcionamiento de la exclusión y la demanda |
| Tasa de aciertos y accesos a almacenamiento | Medir el efecto de la caché |
| Retraso del índice y eventos pendientes | Detectar acumulación en las proyecciones |
| Solicitudes y disponibilidad por instancia | Verificar balanceo y retirada de instancias caídas |

El tablero, el objetivo de servicio y las alertas se definirán en D11. La evidencia se obtendrá sobre componentes implementados mediante pruebas de carga y fallas controladas.

### Balanceo

Se propone desplegar dos instancias de Flota detrás de un mecanismo que compruebe su disponibilidad. El tablero deberá mostrar la distribución de solicitudes y la retirada de una instancia caída. D12 determinará el mecanismo. El eventual escalado de Alquileres conservará las garantías transaccionales en su base.

## 8. Puesta en marcha y publicación

La distribución prevista expone el frontend y el gateway como puntos de acceso y sitúa servicios y almacenes en una red interna. Cada servicio tendrá configuración por entorno y credenciales propias. Se propone Docker Compose para iniciar localmente los componentes y sus dependencias mediante un procedimiento único.

La capacidad publicada deberá estar accesible para el consumidor y permanecer operativa hasta finalizar la evaluación. La selección del entorno determinará la URL pública. Los secretos reales se suministrarán mediante configuración externa al repositorio.

## 9. Limitaciones y aspectos por resolver

| Aspecto | Resolución necesaria |
| --- | --- |
| Tecnologías e infraestructura | Seleccionar versiones, framework, herramientas y entorno; estimar capacidad y costos |
| Arquitectura interna | Formalizar D2 y verificar los patrones trabajados en la materia |
| Consistencia | Definir restricciones, bloqueos e idempotencia en D4; validar concurrencia y recuperación |
| Cambios administrativos | Coordinar bajas de autos y cambios de habilitación con reservas y entregas |
| Integración externa | Acordar consumidor, proveedor, vinculación de clientes, permisos y contrato operativo |
| Publicación | Definir credenciales, URL y puesta en marcha automatizada del sistema completo |
| Evidencia técnica | Implementar outbox y protección ante fallas; medir búsqueda, caché, balanceo, carga y observabilidad |

La versión 0.1 cuenta con un mock en memoria de un solo proceso. Los directorios de servicios, gateway y frontend constituyen una estructura inicial; la persistencia y el comportamiento distribuido se desarrollarán en los hitos posteriores. Las limitaciones conocidas y la deuda técnica aceptada se actualizarán junto con la implementación.

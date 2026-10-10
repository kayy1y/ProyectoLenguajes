# GAMESTATS — CONTEXTO MAESTRO Y SUPERPROMPT PARA CLAUDE CODE (PLAN MODE)

> **Uso:** copia este archivo al repositorio como `GAMESTATS_CONTEXT.md` y pásaselo a Claude Code. Abre Claude en **Plan Mode** y utiliza el prompt de la sección **0**.  
> **Objetivo de esta interacción:** producir un **plan exhaustivo y ejecutable**, no empezar a programar.  
> **Fuente de verdad:** el enunciado académico adjunto, *«GameStats — Plataforma multilenguaje para procesamiento y análisis de eventos de videojuegos»*, proyecto del curso de Lenguajes de Programación. Este documento distingue requisitos del profesor y propuestas técnicas.

---

## 0. SUPERPROMPT — CÓPIALO EN CLAUDE PLAN MODE

Actúa como **arquitecto de software senior, tech lead de Go y Scala, especialista en programación imperativa/funcional, integración de sistemas, testing y DevOps**, y como tutor de un proyecto universitario evaluado por rúbrica.

Tu misión es **DISEÑAR UN PLAN DE CONSTRUCCIÓN COMPLETO, REALISTA Y PASO A PASO para el proyecto GameStats**, usando este documento como contexto obligatorio.

### Reglas de trabajo

1. **PLANIFICACIÓN SOLAMENTE. NO implementes código todavía.** No crees servicios, archivos de código, Dockerfiles ni dependencias sin que apruebe el plan. Si la herramienta permite guardar el plan, hazlo en su mecanismo habitual; si no, entrégalo en Markdown.
2. **Inspecciona primero el repositorio si tienes acceso**: estructura, README, archivos, convenciones y progreso existente. No supongas que está vacío ni reemplaces trabajo previo. Si no existe repositorio o está vacío, planifica la estructura desde cero.
3. **Prioriza los requisitos expresos del enunciado** sobre preferencias tecnológicas. Señala con etiquetas claras: `[OBLIGATORIO]`, `[PROPUESTA]`, `[OPCIONAL]`, `[RIESGO]`, `[PENDIENTE DE CONFIRMAR]`.
4. **Elige un MVP alcanzable y demostrable**, con extensibilidad real pero sin sobreingeniería: no añadas Kafka, Kubernetes, bases de datos distribuidas, frontends completos, autenticación compleja ni servicios cloud costosos salvo justificación contundente.
5. Decide y justifica: juegos, catálogo de eventos, contratos JSON, métricas, patrones secuenciales, protocolo entre servicios, estado/concurrencia en Go, arquitectura funcional Scala, almacenamiento/consulta de resultados, testing y deployment.
6. **No permitas violar la separación académica:** Go ingiere, valida, coordina, agrupa y mantiene estado de flujo; **Scala calcula las estadísticas y detecta patrones**. No escondas análisis en Go ni hagas toda la ingestión en Scala.
7. Diseña **concurrencia real con goroutines y channels** para varias partidas simultáneas, especificando quién es dueño del estado, cómo evitas data races y cómo manejarás backpressure, shutdown y duplicados.
8. Usa técnicas funcionales idiomáticas en Scala (`map`, `filter`, `flatMap`, `groupBy`, `fold`, `reduce`, `collect`, composición de funciones), **según su pertinencia**, evitando mutabilidad innecesaria. El profesor no exige usar todas.
9. **Para cada métrica y patrón demuestra su derivación**: eventos requeridos → campos requeridos → fórmula/regla → tratamiento de casos límite → ejemplo de entrada/salida → prueba que la valida. No inventes valores imposibles de conocer.
10. Incluye **contratos versionables y comprensibles para productores externos**. Especifica también semántica temporal, orden de eventos, IDs, eventos globales sin `playerId`, versiones de juego, duplicados y errores.
11. Diseña **al menos dos partidas simultáneas**, múltiples jugadores, casos normales y especiales, y de ser viable **dos juegos de naturaleza diferente** para demostrar genericidad. Si incorporar un segundo juego complica demasiado el MVP, propón implementación incremental con una prueba de extensión convincente.
12. **No basta con que `docker compose up` arranque**: define una demo completa reproducible desde la simulación hasta el JSON de resultados, más pruebas automáticas y deployment funcional.
13. Incluye build reproducible, Dockerfile **por módulo**, Compose, Git con commits progresivos, GitHub Actions que **compile, pruebe y construya contenedores**, y deployment verificable. Diferencia CI real de CD real, sin prometer CD automatizado si no se configura.
14. Siempre ofrece un **camino principal recomendado** y una pequeña comparación de alternativas, con razones de complejidad, fiabilidad, facilidad de demo y valor académico.
15. Si falta información no bloqueante, **declara supuestos explícitos y continúa**. Formula como máximo unas pocas preguntas **solo si bloquean decisiones esenciales**.
16. Considera que el enunciado dice «domingo 18 de Setiembre de 2026»; el **18/09/2026 fue viernes y ya pasó**. Marca el plazo como **pendiente de confirmar con el docente**; no inventes una nueva fecha.

### Tu respuesta debe contener, en este orden

**A. Lectura del problema y matriz de cumplimiento.**
- Resumen de alcance / fuera de alcance.
- Tabla que vincule **cada requisito de las secciones 3, 5–12 del enunciado** con componente responsable, entregable verificable, caso de prueba y fase.
- Requisitos obligatorios vs decisiones del equipo; incertidumbres y conflictos.

**B. Propuesta de solución justificada.**
- Compara brevemente dos diseños: (i) un juego a fondo; (ii) núcleo genérico + dos juegos. Recomienda uno por puntuación académica, esfuerzo y riesgo.
- Propón juegos concretos y un catálogo **mínimo y suficiente** de eventos.
- Compara HTTP/REST con intercambio por archivos y mensajería; selecciona una opción base.
- Diagrama arquitectónico Mermaid con límites de procesos, flujo y contrato entre capas.
- Tabla de tecnologías y versiones a fijar (sin inventar versiones verificadas; indica que deben comprobarse en entorno real).

**C. Diseño de datos y protocolos.**
- JSON Schema o especificación equivalente para evento individual.
- Ejemplos completos de eventos válidos e inválidos; ejemplos para múltiples juegos.
- Contrato Go → Scala: cuerpo, cabeceras, endpoints si aplica, respuesta y errores.
- Contrato de resultados JSON: estadísticas por jugador/partida/juego, patrones, identificadores y estado del análisis.
- Semántica de IDs, tipos, fechas en UTC ISO 8601, orden por timestamp y criterio de desempate, duplicados/idempotencia, campos opcionales, validación por tipo de evento, eventos de partida sin `playerId`.
- Política explícita para partidas incompletas, reinicios, reenvíos, fallos del servicio analítico, timeouts y reintentos.
- Decide si se analiza al recibir `MATCH_FINISHED` y cómo evitar perder eventos tardíos; define el comportamiento más simple coherente.

**D. Diseño Go, con detalle suficiente para implementar.**
- Paquetes, responsabilidades, estructuras principales y dependencias.
- Pipeline de recepción → validación → clasificación → sesión/estado → coordinación → envío.
- Topología concreta de goroutines y channels; modelo de ownership para mapas y estado; workers por partida vs worker pool; cuándo se cierran canales.
- Estados posibles de la partida y transiciones.
- Estrategia para errores, logging estructurado, duplicados, cancelación, context, graceful shutdown, carga concurrente y pruebas con `go test -race`.
- Clarifica qué NO debe calcular Go.

**E. Diseño Scala funcional.**
- Modelos tipados, manejo de JSON, separación entre controller/entrada, lógica pura, reglas y serialización.
- Funciones puras con firmas/pseudocódigo y composiciones reutilizables.
- Métricas por jugador, partida y combinadas; al menos una de cada categoría.
- Al menos una regla de **secuencia real** (p. ej. racha de eliminaciones o remontada) que considere el orden de eventos.
- Ejemplos paso a paso de cálculo y casos límite (sin dividir entre cero, sin asumir información ausente, sin doble conteo).
- Extensión por juego sin editar el núcleo común; plantea interfaces/estrategias mínimas, no un framework genérico enorme.
- Qué técnicas funcionales demuestran el objetivo académico y cómo explicarlas en defensa.

**F. Organización del repositorio.**
- Árbol de carpetas propuesto (simulator/, ingestion-go/, analytics-scala/, contracts/, tests/, docs/, .github/workflows/, docker-compose.yml, etc.).
- Propósito de cada carpeta/archivo importante.
- README y documentación técnica recomendada.
- Variables de entorno, `.env.example`, gitignore y gestión de secretos.

**G. Plan de ejecución por fases y dependencias.**
Para **cada fase**, produce:
- Objetivo.
- Prerrequisitos.
- Tareas concretas en orden, numeradas y suficientemente pequeñas.
- Archivos o carpetas esperados.
- Diseño/código a implementar (sin implementarlo ahora).
- Comandos de verificación previstos.
- Tests.
- **Criterio de salida objetivo y observable**.
- Riesgos y contingencias.
- Un commit o varios commits con mensajes sugeridos.
- Dependencias y si se puede paralelizar entre dos integrantes.
- Duración relativa en esfuerzo (baja/media/alta; evita cronogramas precisos si no conoces disponibilidad).

Incluye, como mínimo, fases: requisitos/diseño, contratos, simulador y fixtures, Go, Scala, integración, robustez/tests, Docker, CI/CD, deployment, documentación y demo. Ordénalas para obtener una **vertical slice** funcional temprana antes de agregar características complejas.

**H. Estrategia de calidad y pruebas.**
- Matriz de casos: normal, inválido, duplicado, fuera de orden, múltiples partidas, múltiples juegos, partida sin cierre, error Scala, reinicio Go, carrera de datos, reintento, cero eventos, fórmula combinada, patrón secuencial positivo/negativo.
- Unit tests por módulo, contratos, integración HTTP, e2e con Compose y dataset determinista.
- Ejemplos de resultados esperados que sean calculables a mano.
- Criterios para pruebas automatizadas en CI y métricas simples de diagnóstico.

**I. DevOps y despliegue.**
- Diseño de Dockerfiles por servicio (multistage cuando convenga), puertos, networking de Compose, health checks, orden de disponibilidad, volúmenes si los necesitas.
- GitHub Actions: qué jobs ejecuta exactamente, condiciones de éxito, build, tests, construcción/verificación de imágenes; publicar imágenes y deployment solo si es factible.
- Elige un deployment **realista y barato** para dos servicios contenerizados, explicando restricciones/costos/credenciales y alternativa si la nube no está disponible.
- Guía de rollback/reinicio y cómo comprobar desde fuera que funciona.
- Evidencias (capturas, logs y comandos) para la entrega.

**J. Demo, defensa y entregables.**
- Un guion de demo reproducible de principio a fin con comandos propuestos.
- Muestra una secuencia que enseñe: evento válido, inválido, dos partidas simultáneas, estadística de jugador, de partida, combinada y patrón, y respuesta JSON.
- Cómo defender **imperativo vs funcional**, uso genuino de concurrencia, contratos, genérica/extensibilidad y DevOps.
- Checklist final de entregables y artefactos de evidencia.

**K. Decisiones finales y primer sprint.**
- Registro de decisiones arquitectónicas (ADRs breves): elección, motivos, alternativa descartada y consecuencia.
- Top 5 riesgos técnicos con mitigación.
- **MVP mínimo que ya obtiene todos los requisitos esenciales**, separado de mejoras opcionales.
- **Primeras 10–15 tareas concretas**, en orden, listas para convertirse en issues.
- Lista corta de aspectos que necesitan confirmación del docente (si los hay).

### Condiciones de calidad del plan

- Sé específico: evita indicaciones vagas como «hacer el backend» o «añadir pruebas».
- Mantén coherencia total entre eventos, métricas, reglas, fixtures, tests y JSON final.
- No confundas **eventos recibidos**, **datos agrupados por partida** y **resultados de análisis**.
- No conviertas Go en un sistema analítico ni Scala en coordinador de concurrencia e ingestión.
- Señala visiblemente cualquier decisión propuesta que **no esté impuesta** por el profesor.
- El resultado tiene que poder seguirse para construir, probar y defender el proyecto sin rehacer la arquitectura a mitad de camino.
- **Termina tu respuesta con un resumen de la arquitectura recomendada y solicita aprobación del plan antes de implementar.**

---

## 1. CONTEXTO Y ALCANCE ACADÉMICO

**Nombre:** GameStats — Plataforma multilenguaje para procesamiento y análisis de eventos de videojuegos.  
**Curso:** Lenguajes de Programación.  
**Propósito:** demostrar la integración de dos paradigmas: **imperativo en Go**, **funcional en Scala**, sobre una plataforma que recibe eventos de videojuegos simulados y produce análisis estructurados.

### Flujo de alto nivel

```text
┌───────────────────────────────┐
│ Simulador / productor externo │
│ juegos, partidas y jugadores  │
└──────────────┬────────────────┘
               │ eventos JSON
               ▼
┌───────────────────────────────┐
│ GO — ingestión imperativa     │
│ recibe, valida, enruta,       │
│ agrupa, coordina, concurre    │
└──────────────┬────────────────┘
               │ lotes / datos preparados
               ▼
┌───────────────────────────────┐
│ SCALA — análisis funcional    │
│ transforma, calcula métricas, │
│ detecta patrones secuenciales │
└──────────────┬────────────────┘
               │ resultados JSON
               ▼
┌───────────────────────────────┐
│ Consulta / exhibición JSON    │
└───────────────────────────────┘
```

**Fuera del alcance exigido:** desarrollar videojuegos reales, frontend elaborado, big data real, arquitectura de producción a escala masiva, ML, bases de datos obligatorias. Esos elementos solo serían extensiones optativas.

### Requisitos expresos que NO se pueden omitir

1. **Simulación externa** realista y variada con varias partidas, varios jugadores, diferencias de resultados, eventos en distintos instantes, juegos paralelos y situaciones especiales.
2. **Go**: recepción/carga de eventos; validación estructural; tratamiento de inválidos; administración del flujo; organización por juego/partida; estado temporal; concurrencia y envío a Scala.
3. **Scala**: recepción de datos preparados; estructuras propias; colecciones y operaciones funcionales justificadas; estadísticas definidas por el equipo; detección de patrones; JSON estructurado.
4. **Componentes independientes** y protocolo documentado.
5. **Estadística agregada por jugador**.
6. **Estadística agregada por partida**.
7. **Estadística que relacione distintos tipos de eventos**.
8. **Regla que dependa de secuencia o combinación de eventos** y no de un único evento.
9. **Consulta/presentación** de resultados.
10. Git con historial, builds reproducibles, Docker, ejecución integrada, pipeline CI/CD y deployment funcional.
11. Documentación técnica y demo.
12. Extensibilidad para nuevos conjuntos de eventos sin rehacer por completo el sistema.
13. Equipos de **máximo dos personas**. Copias prohibidas.

**Matiz importante:** el enunciado enumera `filter`, `map`, `flatMap`, `groupBy`, `fold`, `reduce`, `collect` y composición como *ejemplos pertinentes*, **no como obligación de usar todas**.

**Fecha administrativa:** el documento consigna «domingo 18 de Setiembre de 2026, antes de las 10:00 pm, por Tec Digital». Existe un error entre día de semana y fecha: el **18 de septiembre de 2026 fue viernes**, y es una fecha pasada respecto del contexto actual (octubre de 2026). No fijar un deadline nuevo; confirmarlo.

---

## 2. DECISIONES DE DISEÑO QUE CLAUDE DEBE ANALIZAR

> Lo siguiente **NO es un mandato del enunciado**. Son propuestas iniciales para reducir el riesgo; Claude debe evaluarlas y justificarlas.

### 2.1 Juegos candidatos

**Opción A — Combate por rondas (principal):** produce eventos de inicio, unión, daño, eliminación, objetivo y cierre. Permite:
- Jugador: eliminaciones, derrotas, daño total, objetivos cumplidos.
- Partida: duración, total de eliminaciones, número de participantes.
- Combinada: `eliminaciones / intentos_de_ataque`, **solo si hay eventos de intentos**; o `objetivos_completados / recursos_consumidos`, **solo si ambas cantidades están registradas**.
- Secuencia: racha de 3 eliminaciones **sin muerte intermedia** en orden temporal.

**Opción B — Carreras (extensión):** eventos de inicio, unión, vuelta completada, cambios de posición y cierre.
- Jugador: mejor vuelta y tiempo promedio de vuelta.
- Partida: duración y cantidad de finalistas.
- Combinada: porcentaje de vueltas válidas sobre vueltas intentadas, **si se registran tanto intentos como vueltas válidas**.
- Secuencia: remontada desde posición desfavorable hasta finalizar primero, si existe trayectoria de posiciones + clasificación final.

**Pregunta arquitectónica clave:** ¿conviene hacer un único juego completo con una extensión sencilla, o implementar ambos con un núcleo común desde el inicio? Recomendar atendiendo esfuerzo y evidencia de genericidad.

### 2.2 Formato común propuesto

```json
{
  "eventId": "evt-0001",
  "timestamp": "2026-10-01T15:30:00Z",
  "gameId": "arena-combat",
  "gameVersion": "1.0",
  "matchId": "match-001",
  "playerId": "player-01",
  "type": "PLAYER_ACTION",
  "action": "ATTACK",
  "data": {
    "targetPlayerId": "player-02",
    "damage": 25,
    "hit": true
  }
}
```

**Este ejemplo es una propuesta**, no un contrato cerrado. Ajustar nombres/tipos para facilitar validación y estadísticas. Justificar `playerId` opcional en eventos globales (`MATCH_STARTED`, `MATCH_FINISHED`) frente a la validación mínima sugerida por el profesor («cuando corresponda»). Evitar valores de `data` inconsistentes entre juegos. Definir claramente si `timestamp` es *event-time* y si también se registra tiempo de ingestión. Definir desempate de eventos simultáneos sin asumir causalidad inexistente.

### 2.3 Comunicación recomendada a estudiar

**HTTP/REST** como candidato principal:
- Simulador → `POST /events` en Go.
- Go → `POST /analyze` en Scala con un lote de una partida terminada y sus metadatos.
- Scala → JSON con análisis; Go podría exponer `GET /results/{gameId}/{matchId}` si conserva los resultados.
- Si las respuestas son síncronas, describir bloqueo, timeouts y reintentos; si asíncronas, describir estado y consulta.
- **Decidir expresamente dónde se guardan los resultados** y la semántica de error si falla Scala. No dar por supuesto que hay una base de datos.

Alternativas a evaluar: archivos JSON / JSONL y cola de mensajes. Evitar infraestructura extra si REST satisface la prueba académica.

### 2.4 Estado y simultaneidad: preguntas que debe resolver el plan

- ¿Un coordinador central despacha eventos a workers por partida?
- ¿Worker dedicado por partida o pool de workers con ownership estricto de estado?
- ¿Cómo se conserva el orden dentro de una partida cuando hay productores concurrentes?
- ¿Qué sucede si el evento final llega antes de acciones retrasadas?
- ¿Cómo se elimina estado de partidas cerradas?
- ¿Qué se conserva si falla Scala: lote, resultado o estado de error?
- ¿Qué ocurre si llega dos veces el mismo `eventId`?
- ¿Cómo demostrar que **varias partidas avanzan en paralelo**, no solo que existen goroutines decorativas?
- ¿Cómo observar data races con `go test -race` y evitar bloqueos de channels?

Estas preguntas deben resolverse con un diseño concreto y pruebas.

---

## 3. CRITERIOS DE ACEPTACIÓN NO NEGOCIABLES

| ID | Evidencia de aceptación |
|---|---|
| AC-01 | Un tercero puede construir un evento válido consultando solo la especificación. |
| AC-02 | El sistema recibe eventos de distintas partidas y juegos con identidad inequívoca. |
| AC-03 | Eventos mal formados se rechazan o registran con motivo claro; el proceso sigue estable. |
| AC-04 | Go coordina/agrupa y maneja partidas concurrentemente con goroutines y channels sin carreras detectadas. |
| AC-05 | Existe estado temporal por partida y criterio de envío a Scala. |
| AC-06 | Scala procesa datos preparados y usa transformaciones funcionales justificables. |
| AC-07 | Hay una métrica agregada por jugador y su test con cálculo esperado. |
| AC-08 | Hay una métrica agregada por partida y su test con cálculo esperado. |
| AC-09 | Hay una métrica que combina tipos de evento distintos y su test. |
| AC-10 | Hay al menos un patrón secuencial verdadero, con caso positivo y negativo. |
| AC-11 | El resultado JSON puede recuperarse o mostrarse mediante una ruta/comando documentado. |
| AC-12 | Simulador muestra diferentes jugadores, partidas simultáneas, varios resultados y caso especial. |
| AC-13 | La incorporación de un segundo juego o nuevo tipo no exige reescribir la lógica común. |
| AC-14 | Go y Scala funcionan como servicios/procesos separados mediante protocolo documentado. |
| AC-15 | `docker compose up --build` arranca un sistema utilizable y puede ejecutarse una demo e2e. |
| AC-16 | CI verifica compilación, tests y construcción/gestión de imágenes. |
| AC-17 | Existe despliegue funcional documentado y evidencias de ejecución. |
| AC-18 | Repositorio contiene commits incrementales y documentación/guion de defensa. |

---

## 4. PRUEBAS DE DISEÑO QUE DEBEN APARECER EN EL PLAN

Diseña fixtures **deterministas** (semillas fijas/tiempos conocidos) con entradas y resultados esperados:
- Partida normal con varios jugadores.
- Dos partidas del mismo juego intercaladas.
- Partidas de dos juegos diferentes intercaladas (si se escoge esa opción).
- Evento sin `eventId`, timestamp inválido, acción desconocida y campo `data` incorrecto.
- Dos eventos con el mismo `eventId`.
- Evento antiguo recibido después de uno nuevo.
- Evento global sin `playerId`.
- Partida sin `MATCH_FINISHED`.
- Falla momentánea de Scala y comportamiento observable de Go.
- Métrica combinada con denominador cero.
- Patrón secuencial positivo y negativo (una interrupción debe romper la racha).
- Eventos de eliminación contradictorios o jugador inexistente, según reglas semánticas definidas.
- Reinicio y documentación explícita de lo que se pierde si se usa almacenamiento en memoria.

Para cada fixture espera: IDs, secuencia ordenada, fórmula, JSON esperado, archivo asociado y test que lo ejecuta.

---

## 5. DEVOPS Y DOCUMENTACIÓN: DEFINICIÓN DE «TERMINADO»

**Repositorio:** README de inicio rápido; `docs/architecture.md`, `docs/event-spec.md`, `docs/statistics.md`, `docs/decisions.md`, `docs/deployment.md`, `docs/demo.md`; ejemplos JSON y `.env.example`.

**Docker:** Dockerfile para Go, Dockerfile para Scala, Compose con variables, red interna, health checks si son viables, puertos declarados, ejecución reproducible, imágenes correctamente construidas.

**GitHub Actions:** disparo push/PR; job Go (lint opcional, compilación, tests, race test cuando sea práctico), job Scala (compilar y tests), job Docker (build de ambas imágenes), job integración si está disponible. El nombre **CI/CD** no equivale a tener deploy automático: especificar si deployment es manual/documentado o automatizado.

**Deployment:** una versión funcional de la plataforma ejecutada fuera del entorno de desarrollo local, o en el entorno que el docente acepte como deployment. Elegir destino y demostrarlo. Si se propone una VM con Docker Compose, describir despliegue, acceso, reinicio, firewall/puertos y costos; si se propone PaaS, evaluar soporte real para ambos contenedores y gastos. Evitar asumir créditos cloud.

**Defensa académica:** preparar ejemplos explicables de goroutines/channels, estado imperativo, funciones puras y colecciones Scala, contratos HTTP, extensibilidad y secuencia de eventos. El plan debe incluir qué mostrar en pantalla y por qué satisface cada requisito.

---

## 6. LÍMITES, PRIORIDADES Y ANTIFALLOS

### Prioridad P0 — obligatorio para el MVP

Simulador; esquema documentado; eventos de varias partidas/jugadores; validación; concurrencia auténtica en Go; estado por partida; comunicación con Scala; cálculo funcional; los cuatro tipos de análisis solicitados; JSON consultable; Docker/Compose; CI; deployment; docs; demo.

### Prioridad P1 — robustez recomendable

Pruebas de contrato y e2e, tratamiento explícito de duplicados, reintentos acotados, logs, shut down ordenado, fixtures deterministas, extensibilidad demostrada con segundo juego.

### Prioridad P2 — mejoras solo si sobran recursos

Panel web, métricas de observabilidad avanzadas, base de datos persistente, cola de mensajes, autenticación multiusuario, dashboard de gráficos, despliegue automatizado completo.

### Antipatrones a impedir

- Calcular en Go los KPIs que pertenecen a Scala.
- Usar Scala de forma mayoritariamente imperativa sin operaciones funcionales reconocibles.
- Generar números «de resultado» ya calculados en los eventos, evitando el trabajo de Scala.
- Una única partida de ejemplo presentada como «concurrencia».
- Una regla secuencial implementada solo como conteo.
- Una métrica cuya información no está en el esquema.
- Un `docker-compose` que solo arranca dos procesos que no se hablan.
- Un workflow que solo imprime mensajes sin compilar o construir imágenes.
- Documentar endpoints o comandos distintos de los realmente implementados.
- Almacenar secretos en Git.
- Multiplicar módulos o infraestructura sin beneficio evaluable.
- Marcar una función o un despliegue como «terminado» sin prueba y evidencia.

---

## 7. INSTRUCCIÓN FINAL A CLAUDE

Con todo el contexto anterior, **redacta ahora el plan completo, estructurado según A–K**. Trátalo como el documento que guiará toda la construcción del proyecto. Haz elecciones justificadas, mantén el alcance viable para uno o dos estudiantes, define contratos y casos de aceptación verificables, y describe los pasos exactos para producir una primera versión end-to-end lo antes posible.

**En esta ejecución solo planifica. No programes. No cambies el repositorio.** Presenta el plan para mi revisión y aprobación antes de empezar la fase 1.

---

## 8. TRAZABILIDAD DEL ENUNCIADO ORIGINAL

Referencia de secciones para cotejar el plan:
- **§1–2:** problema y propósito de GameStats.
- **§3:** estructura común del evento y campos ilustrativos.
- **§4:** juegos/eventos ilustrativos, no obligatorios.
- **§5:** diseño libre, estadísticas derivables y regla sobre secuencia.
- **§6.1:** responsabilidades imperativas de Go, estado, concurrencia.
- **§6.2:** responsabilidades funcionales de Scala, patrones, salida estructurada.
- **§7:** comunicación entre procesos y contrato externo.
- **§8:** simulación de escenarios variados/paralelos.
- **§9:** diez funcionalidades mínimas.
- **§10:** Git, reproducibilidad, Docker, integración, CI/CD y deployment.
- **§11:** entregables académicos.
- **§12:** restricciones de lenguaje y extensibilidad.
- **§13:** justificación de paradigmas.
- **§14:** equipo máximo de dos integrantes, originalidad y dato de entrega por confirmar.

**Importante:** cuando un requisito del enunciado sea abierto (p. ej. cuál juego, qué estadísticas, cómo comunicar módulos), presentarlo como una **decisión de arquitectura**, no como requisito textual del docente.

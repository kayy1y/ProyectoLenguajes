# Registro de decisiones arquitectónicas (ADR)

Todas son decisiones **del equipo** (no impuestas por el enunciado). Cada una indica la alternativa descartada y su consecuencia.

## ADR-01 — REST síncrono entre Go y Scala
- **Decisión:** Go envía el lote completo de una partida cerrada a `POST /v1/analyze` y espera la respuesta.
- **Motivo:** contrato explícito, fácil de probar con `curl`, sin infraestructura extra.
- **Descartado:** cola de mensajes (servicio adicional sin beneficio evaluable); archivos compartidos (polling, mal feedback de error).
- **Consecuencia:** Go implementa timeouts, reintentos con backoff y estado `FAILED` con re-análisis manual.

## ADR-02 — Núcleo genérico + Arena completo + Racing incremental
- **Decisión:** `arena-combat` completo en el MVP; `street-racing` mínimo después, como prueba de extensibilidad.
- **Motivo:** equilibrio entre esfuerzo y evidencia de AC-13.
- **Descartado:** un único juego (extensibilidad solo prometida); dos juegos desde el inicio (abstracción prematura).
- **Consecuencia:** las abstracciones (`GameAnalyzer`, esquema por juego) nacen del primer juego y se validan con el segundo. Contingencia: agregar `ITEM_USED` a Arena.

## ADR-03 — Dispatcher único + worker por partida + pool de análisis
- **Decisión:** una goroutine dispatcher es la única dueña del mapa de partidas; cada partida tiene su goroutine; las llamadas a Scala las hace un pool aparte.
- **Motivo:** orden garantizado dentro de cada partida sin locks, paralelismo entre partidas, una partida lenta no bloquea a otras.
- **Descartado:** mapa con mutex global; pool con asignación por hash.
- **Consecuencia:** más goroutines (baratas) y un diseño explicable en la defensa.

## ADR-04 — JSON Schema como contrato único
- **Decisión:** `contracts/schemas/*.json` es la fuente de verdad, usada por Go para validar y por las pruebas y la documentación.
- **Motivo:** un productor externo puede construir eventos solo con la especificación (AC-01); agregar un juego = agregar un esquema (AC-13).
- **Descartado:** validación codificada a mano en Go.
- **Consecuencia:** dependencia de una librería de JSON Schema en Go.

## ADR-05 — Scala sin estado; Go guarda los resultados
- **Decisión:** Scala solo calcula; Go conserva estado y resultados en memoria, con snapshot JSON opcional (`RESULTS_DIR`).
- **Motivo:** separación de responsabilidades; sin base de datos.
- **Descartado:** base de datos persistente.
- **Consecuencia:** lo que se pierde al reiniciar queda documentado en `docs/architecture.md`.

## ADR-06 — Análisis al cierre con período de gracia
- **Decisión:** tras `MATCH_FINISHED` se esperan `MATCH_GRACE_PERIOD` (2 s) antes de enviar; los eventos posteriores reciben `409 LATE_EVENT`.
- **Motivo:** el comportamiento más simple que sigue siendo coherente.
- **Descartado:** re-análisis incremental.
- **Consecuencia:** los eventos muy tardíos se pierden, pero quedan visibles en `/v1/rejections`.

## ADR-07 — Orden total `(timestamp, seq, ingest.seq)`
- **Decisión:** dentro de una partida se ordena por timestamp, luego por `seq` del productor (ausente = +∞), luego por orden de llegada.
- **Motivo:** orden determinista sin inventar causalidad.
- **Descartado:** ordenar solo por timestamp (no determinista ante empates).
- **Consecuencia:** se recomienda a los productores enviar `seq`.

## ADR-08 — Scala 3 + Cask + upickle + munit
- **Decisión:** capa HTTP delgada con Cask; núcleo puro.
- **Motivo:** curva de aprendizaje baja; el foco académico queda en colecciones y funciones puras.
- **Descartado:** http4s + cats-effect + circe.
- **Consecuencia:** menos programación funcional de efectos y más foco en transformaciones de colecciones.

## ADR-09 — Simulador en Go, módulo separado
- **Decisión:** `simulator/` con su propio `go.mod`.
- **Motivo:** un toolchain menos; goroutines para enviar partidas en paralelo.
- **Descartado:** Python.
- **Consecuencia:** debe mantenerse fuera de `ingestion-go`.

## ADR-10 — Deployment en VM + Compose + GHCR, deploy manual documentado
- **Decisión:** las imágenes se publican en GHCR desde CI; la VM las descarga con Compose.
- **Motivo:** barato e idéntico al entorno local.
- **Descartado:** PaaS (la red privada entre dos servicios suele ser de pago y hay cold starts).
- **Consecuencia:** el CD automatizado queda como opcional; el deploy es manual y documentado.

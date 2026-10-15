# GameStats

![CI](../../actions/workflows/ci.yml/badge.svg)

**GameStats recibe los eventos que produce un videojuego durante una partida y los convierte en estadísticas.**
Por ejemplo: quién eliminó a quién, la precisión de cada jugador, la mejor vuelta de una carrera, o si alguien encadenó una racha o hizo una remontada.

Es el proyecto del curso **Lenguajes de Programación**. Cada parte está hecha con el paradigma que mejor le encaja:

- **Go (imperativo y concurrente)** recibe los eventos, los valida, los agrupa por partida y decide cuándo una partida está lista para analizarse.
- **Scala (funcional)** recibe la partida completa y calcula las estadísticas y los patrones con funciones puras.

No hace falta un videojuego real: un **simulador** genera partidas de dos juegos distintos, incluidos casos raros a propósito (eventos inválidos, duplicados, eventos fuera de orden o partidas que nunca terminan).

```
 simulador ──eventos──▶ ingestion-go ──partida completa──▶ analytics-scala
                         (valida, agrupa,                  (estadísticas y
                          1 goroutine por partida)          patrones)
                              │
                              └──▶ resultados consultables por HTTP (:8080)
```

---

## Contenido

1. [Requisitos](#1-requisitos)
2. [Instalar y arrancar en tu computadora](#2-instalar-y-arrancar-en-tu-computadora)
3. [Versión desplegada en la nube](#3-versión-desplegada-en-la-nube)
4. [Forma 1: probarlo con la interfaz gráfica](#4-forma-1-probarlo-con-la-interfaz-gráfica)
5. [Forma 2: probarlo desde la terminal](#5-forma-2-probarlo-desde-la-terminal)
6. [Qué calcula](#6-qué-calcula)
7. [API](#7-api)
8. [Configuración](#8-configuración)
9. [Problemas frecuentes](#9-problemas-frecuentes)
10. [Estructura del repositorio](#10-estructura-del-repositorio)
11. [Desarrollo, pruebas automáticas y CI/CD](#11-desarrollo-pruebas-automáticas-y-cicd)
12. [Documentación técnica](#12-documentación-técnica)

---

## 1. Requisitos

Depende de lo que quieras hacer:

| Para… | Necesitas |
|---|---|
| **Levantar el sistema** (lo único obligatorio) | [Docker Desktop](https://www.docker.com/products/docker-desktop/) o Docker Engine con Compose v2. No hace falta instalar Go, Java ni Scala: todo se compila dentro de los contenedores |
| **Probarlo con la interfaz gráfica** | Go 1.26 y un compilador de C (ver [4.1](#41-compilar-y-abrir-la-aplicación)). También puedes bajar el ejecutable ya compilado desde GitHub Actions |
| **Probarlo desde la terminal** | `curl` y `jq` (en macOS: `brew install jq`; en Ubuntu: `sudo apt install jq curl`) |
| **Usar los atajos `make …`** | `make` (viene en macOS y Linux). En Windows puedes usar directamente los comandos `docker compose …` que aparecen junto a cada atajo |
| **Modificar el código y correr las pruebas unitarias** | Go 1.26, JDK 21 y sbt 1.x |

Puertos que usa: **8080** (servicio Go, el único expuesto). El servicio Scala (8081) solo es accesible dentro de la red de Docker.

---

## 2. Instalar y arrancar en tu computadora

```bash
git clone https://github.com/kayy1y/ProyectoLenguajes.git
cd ProyectoLenguajes
docker compose up -d --build --wait      # o: make up
```

La primera vez tarda unos minutos, porque descarga las imágenes base y compila Go y Scala. Las siguientes son casi inmediatas.

**Cómo saber que funciona:**

```bash
docker compose ps
curl -s localhost:8080/healthz
```

Debes ver los servicios `analytics` e `ingestion` en estado `healthy`, y la respuesta `{"status":"ok"}`.

> El sistema no tiene una pantalla propia: es un servicio que espera eventos. Para verlo funcionar tienes dos caminos: la **interfaz gráfica** (sección 4) o la **terminal** (sección 5). Los dos usan el mismo sistema, así que puedes combinarlos.

Para apagarlo:

```bash
docker compose down        # apaga y conserva los resultados (o: make down)
docker compose down -v     # apaga y borra todo, para empezar de cero
```

---

## 3. Versión desplegada en la nube

El proyecto también está publicado en internet, para que puedas probarlo sin instalar nada:

| | |
|---|---|
| **Dirección** | https://gamestats-ingestion.onrender.com |
| **Comprobar que está vivo** | https://gamestats-ingestion.onrender.com/healthz (debe responder `{"status":"ok"}`) |
| **Servicio de análisis (Scala)** | https://gamestats-analytics.onrender.com/health (debe responder `{"status":"ok"}`). Lo usa la ingestión; no hace falta llamarlo directamente |
| **Plataforma** | [Render](https://render.com), plan gratuito |

**Antes de usarla, ten en cuenta:**
- **Se duerme** si nadie la usa durante 15 minutos. La primera petición tarda alrededor de un minuto en despertarla. Abre el `/healthz` un minuto antes de la demo.
- **Los resultados no duran:** están en memoria, así que cuando el servicio se duerme se borran. Si quieres estadísticas, genera datos de nuevo (sección 5.3 o la pestaña *Generar datos* de la aplicación).
- **Escribir requiere clave:** los eventos nuevos llevan la clave de API (`X-API-Key`). Pídela al equipo; las lecturas (`GET`) son públicas.

### Cómo probarla

Con la aplicación de escritorio, apuntando a la nube:
```bash
./bin/gamestats-gui --target https://gamestats-ingestion.onrender.com --api-key TU_CLAVE
```

Con la terminal, la verificación completa:
```bash
docker compose --profile demo run --rm --no-deps simulator --target https://gamestats-ingestion.onrender.com --api-key TU_CLAVE --fixture all --expect --wait 240s
```
Debe terminar con 8 líneas `ok`.

Para consultar un resultado (no necesita clave):
```bash
curl -s https://gamestats-ingestion.onrender.com/v1/games/arena-combat/matches/match-a1/results | jq '.result.playerStats'
```

### Otras formas de desplegarlo

| Opción | Costo | Cuándo usarla |
|---|---|---|
| **Render con el Blueprint** (la que usamos) | Gratis, sin tarjeta | Es la forma más simple. El archivo [`render.yaml`](render.yaml) describe los dos servicios; en Render: *New → Blueprint* → este repositorio → *Apply* |
| **Redespliegue automático** | Gratis | Cada push a `main` vuelve a desplegar. Requiere configurar los secretos `RENDER_DEPLOY_HOOK_ANALYTICS` y `RENDER_DEPLOY_HOOK_INGESTION` en GitHub |
| **VM con Docker Compose** | Según el proveedor | Servidor propio, siempre encendido. Usa [`deploy/docker-compose.deploy.yml`](deploy/docker-compose.deploy.yml) |
| **Túnel desde tu computadora** (`cloudflared tunnel --url http://localhost:8080`) | Gratis | Solo mientras tu computadora esté encendida. Úsala como respaldo |

Los pasos completos de cada opción están en [docs/deployment.md](docs/deployment.md).

---

## 4. Forma 1: probarlo con la interfaz gráfica

Es una **aplicación de escritorio** (una ventana normal, no una página web) para recorrer todo el proyecto con clics. Se conecta al sistema que levantaste en el paso anterior.

### 4.1 Compilar y abrir la aplicación

Instala una sola vez lo que pide tu sistema operativo:

| Sistema | Qué instalar |
|---|---|
| macOS | [Go 1.26](https://go.dev/dl/) y `xcode-select --install` |
| Windows | [Go 1.26](https://go.dev/dl/) y un gcc ([MSYS2](https://www.msys2.org/) o TDM-GCC) |
| Linux (Debian/Ubuntu) | [Go 1.26](https://go.dev/dl/) y `sudo apt install gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev wayland-protocols` |

Después, desde la carpeta del proyecto:

```bash
make gui-run
```

Sin `make` (por ejemplo, en Windows):

```bash
cd simulator
go build -tags gui -o ../bin/gamestats-gui ./cmd/gui
cd ..
./bin/gamestats-gui            # en Windows: bin\gamestats-gui.exe
```

La primera compilación tarda alrededor de un minuto.

**Sin compilar nada:** cada ejecución de CI deja el ejecutable para Linux, macOS y Windows. En GitHub entra a **Actions → CI**, abre la ejecución más reciente en verde y descarga el artefacto `gamestats-gui-<tu sistema>`. Hay que ejecutarlo dentro de la carpeta del repositorio, porque lee los casos de prueba de `contracts/`.

### 4.2 Recorrido sugerido (5 minutos)

La pestaña **Guía** de la aplicación trae este mismo recorrido.

1. **Generar datos**: elige *Escenario demo* y pulsa **Ejecutar**. Se envían cinco partidas de dos juegos a la vez, con eventos inválidos, un duplicado, un evento tardío y una partida sin final. Verás el progreso y una tabla que explica cada respuesta del sistema.
2. **Partidas**: cada partida pasa por `COLLECTING` → `ANALYZING` → `ANALYZED`. La que nunca recibió `MATCH_FINISHED` se cierra sola por inactividad y queda como `INCOMPLETE`.
3. **Selecciona una partida**: a la derecha aparecen sus estadísticas por partida y por jugador, los patrones detectados (racha o remontada) y los avisos sobre datos incoherentes. Lo que no se puede calcular aparece como «—», nunca como un 0 inventado.
4. **Verificación**: pulsa **Ejecutar suite completa**. Se corren 37 comprobaciones y deben pasar todas:
   - 8 partidas de prueba cuyo resultado debe ser idéntico al calculado a mano;
   - 17 eventos inválidos que deben rechazarse con el error exacto;
   - 12 comprobaciones del ciclo de vida de una partida.

   Con **Guardar informe…** obtienes un `.md` que sirve como evidencia.
5. **Enviar evento**: elige una plantilla inválida (por ejemplo `bad-timestamp.json`), pulsa **Enviar** y mira cómo el sistema indica el campo y el motivo del error.

### 4.3 Qué hay en cada pestaña

| Pestaña | Qué puedes hacer |
|---|---|
| **Partidas** | Ver todas las partidas en vivo, filtrarlas y abrir el detalle de cualquiera |
| **Generar datos** | Enviar casos de prueba concretos, el escenario demo o una carga de muchas partidas en paralelo, con semilla y velocidad |
| **Verificación** | Ejecutar la batería completa de pruebas y guardar el informe |
| **Enviar evento** | Escribir un evento a mano o partir de un ejemplo del contrato |
| **Rechazos** | Ver los últimos eventos rechazados y por qué |
| **Guía** | Seguir el recorrido sugerido y ver los juegos soportados |

Puedes repetir cualquier ejecución sin reiniciar nada: la aplicación usa identificadores nuevos cada vez.

Para conectarla a un servidor remoto: `./bin/gamestats-gui --target http://<ip>:8080 --api-key <clave>`. Más detalles en [docs/gui.md](docs/gui.md).

---

## 5. Forma 2: probarlo desde la terminal

Todo lo que hace la interfaz también se puede hacer desde la terminal. Los comandos se ejecutan desde la carpeta del proyecto, con el sistema levantado.

> **Usuarios de macOS (zsh):** no pegues las líneas que empiezan con `#`, porque zsh no las trata como comentarios. Si quieres pegarlas, ejecuta antes `setopt interactivecomments`.

### 5.1 Verificación automática (el camino más rápido)

Envía las 8 partidas de prueba y compara cada resultado con el calculado a mano:

```bash
docker compose --profile demo run --rm simulator --fixture all --expect
```

Al final debe aparecer:

```
--- comparación con contracts/expected
ok   F01-arena-normal.result.json
ok   F06-arena-out-of-order.result.json
...
ok   F18-racing-no-comeback.result.json
```

Si lo vuelves a correr sin borrar los datos, agrega `--suffix -2` (o `-3`, …): las partidas ya cerradas no se reabren y el sistema rechazaría sus eventos.

### 5.2 Ver un resultado

```bash
curl -s localhost:8080/v1/games/arena-combat/matches/match-a1/results | jq '.result'      # o: make results
```

Para `match-a1` (la partida F01) debes ver, entre otras cosas:

| Jugador | Ataques | Aciertos | Precisión | Daño | Eliminaciones | Muertes | Eficiencia |
|---|---|---|---|---|---|---|---|
| p1 | 5 | 4 | 0.8 | 300 | 3 | 1 | 0.6 |
| p2 | 2 | 1 | 0.5 | 100 | 1 | 2 | 0.5 |
| p3 | 1 | 1 | 1 | 30 | 0 | 1 | 0 |

- **Partida:** 120 s, 3 participantes, 18 eventos, 4 eliminaciones, 430 de daño, 2 eliminaciones por minuto. El mejor jugador es `p1`.
- **Patrón:** `KILL_STREAK` de `p1` con longitud 3, del evento `a1-e07` al `a1-e13`.

Atajos útiles para mirar partes concretas:

```bash
R=localhost:8080/v1/games/arena-combat/matches/match-a1/results
curl -s $R | jq '.result.playerStats'      # por jugador
curl -s $R | jq '.result.matchStats'       # por partida
curl -s $R | jq '.result.patterns'         # patrones
curl -s localhost:8080/v1/games/street-racing/matches/race-r1/results | jq '.result.patterns'   # remontada (COMEBACK) de d3
curl -s localhost:8080/v1/matches | jq -c '.[] | {matchId, status, completeness}'               # todas las partidas
```

### 5.3 Escenario demo

```bash
make demo
# sin make:
docker compose --profile demo run --rm simulator --scenario demo --seed 42
```

El simulador envía cinco partidas de dos juegos en paralelo, más los casos especiales. Al terminar muestra una tabla de respuestas como esta:

```
--- respuestas de ingestión
  171  202 ACCEPTED
    1  200 DUPLICATE
    1  400 REJECTED INVALID_FORMAT /timestamp
    1  400 REJECTED REQUIRED_FIELD /playerId
    1  400 REJECTED UNKNOWN_EVENT_TYPE /type
    1  409 REJECTED DUPLICATE_ID_CONFLICT
    1  409 REJECTED LATE_EVENT
```

Cada línea demuestra algo:
- **Inválidos:** los eventos con errores se rechazan diciendo el campo y el motivo.
- **Duplicado idéntico:** se acepta sin contarse dos veces.
- **Mismo id con otro contenido:** es un conflicto y se rechaza.
- **Evento tardío:** el que llega después de cerrar la partida se rechaza.

La partida `demo-open` nunca termina y se analiza sola como `INCOMPLETE`. Si ya corriste los fixtures, agrega `--suffix -2`.

### 5.4 Demo guiada paso a paso

```bash
./scripts/demo.sh
```

Recorre las mismas pruebas de la demostración con pausas: pulsa ENTER para avanzar. El guion completo está en [docs/demo.md](docs/demo.md).

### 5.5 Pruebas a mano, evento por evento

Para armar tu propia partida y calcular el resultado con calculadora, carga primero estas funciones auxiliares:

```bash
S=-1
source scripts/pruebas.sh
```

Debe responder `Listo: ev, mk y res cargadas (S=-1, G=arena-combat)`. Las funciones viven solo en esa terminal: si abres otra, repite el `source`.

Ejemplo: una partida corta entre `ana` y `beto`.

```bash
M=mi-partida
ev "$(mk e1 15:00:00 MATCH_STARTED '' '{"map":"dust"}')"
ev "$(mk e2 15:00:01 PLAYER_JOINED ana '{}')"
ev "$(mk e3 15:00:01 PLAYER_JOINED beto '{}')"
ev "$(mk e4 15:00:10 ATTACK ana '{"targetPlayerId":"beto","weapon":"rifle","hit":true,"damage":50}')"
ev "$(mk e5 15:00:12 ATTACK ana '{"targetPlayerId":"beto","weapon":"rifle","hit":false,"damage":0}')"
ev "$(mk e6 15:00:15 ATTACK ana '{"targetPlayerId":"beto","weapon":"rifle","hit":true,"damage":60}')"
ev "$(mk e7 15:00:15 ELIMINATION ana '{"victimPlayerId":"beto","weapon":"rifle"}')"
ev "$(mk e8 15:00:20 ATTACK beto '{"targetPlayerId":"ana","weapon":"pistol","hit":true,"damage":20}')"
ev "$(mk e9 15:01:00 MATCH_FINISHED '' '{"reason":"COMPLETED"}')"
sleep 3
res | jq -c '.result.playerStats[]'
```

Cada envío responde `{"status":"ACCEPTED",...} [202]`. Después de 3 segundos, el resultado debe ser:

- **ana:** 3 ataques, 2 aciertos, precisión `0.6667` (2/3), 110 de daño (50+60), 1 eliminación, eficiencia `0.3333` (1 eliminación / 3 ataques).
- **beto:** 1 ataque, 1 acierto, precisión `1`, 20 de daño, 1 muerte, eficiencia `0`.

Para repetir la prueba, cambia el sufijo (`S=-2`) o borra todo con `docker compose down -v`.

### 5.6 Prueba de punta a punta

```bash
make e2e      # o: ./tests/e2e/e2e.sh
```

Levanta el sistema desde cero y ejecuta 23 comprobaciones:
- todas las partidas de prueba;
- inválidos, duplicados y eventos tardíos;
- caída y recuperación de Scala;
- reinicio de Go sin perder resultados;
- una carga de 30 partidas en paralelo.

Debe terminar con `E2E OK`. Al final **apaga el sistema y borra sus datos**: vuelve a levantarlo con `make up` si quieres seguir probando.

---

## 6. Qué calcula

Se implementaron dos juegos muy distintos para demostrar que el núcleo no depende de uno solo:

| | `arena-combat` (combate) | `street-racing` (carreras) |
|---|---|---|
| **Por jugador** | ataques, aciertos, precisión, daño, eliminaciones, muertes, objetivos | vueltas válidas e inválidas, mejor vuelta, promedio, posición final |
| **Por partida** | duración, participantes, eliminaciones y daño totales, mejor jugador | finalistas, abandonos, vuelta más rápida, ganador |
| **Combinada** (relaciona tipos de evento distintos) | `killEfficiency` = eliminaciones ÷ ataques; eliminaciones por minuto | `positionsGained` = posición de salida − posición de llegada |
| **Patrón secuencial** | `KILL_STREAK`: 3 o más eliminaciones seguidas sin morir | `COMEBACK`: estar 3º o peor y terminar ganando |

Los patrones dependen del orden de los eventos, no solo de contarlos. Dos ejemplos:

- **Racha:** un jugador con 3 eliminaciones no tiene racha si murió entre la segunda y la tercera (fixture F12).
- **Remontada:** ganar la carrera no basta si nunca se estuvo atrás (fixture F18).

Cuando una métrica no se puede calcular (por ejemplo, la precisión de alguien que nunca atacó), vale `null`. Nunca se inventa un 0. La derivación de cada métrica está en [docs/statistics.md](docs/statistics.md).

---

## 7. API

Todo se consulta en el servicio Go, en el puerto 8080:

| Método y ruta | Para qué |
|---|---|
| `POST /v1/events` | Enviar un evento (formato en [docs/event-spec.md](docs/event-spec.md)) |
| `GET /v1/matches` | Listar las partidas y su estado (admite `?status=ANALYZED`) |
| `GET /v1/games/{gameId}/matches/{matchId}` | Estado de una partida |
| `GET /v1/games/{gameId}/matches/{matchId}/results` | Resultado del análisis |
| `POST /v1/games/{gameId}/matches/{matchId}/reanalyze` | Reintentar una partida cuyo análisis falló |
| `GET /v1/rejections` | Últimos eventos rechazados y el motivo |
| `GET /v1/stats` | Contadores internos |
| `GET /healthz` | Comprobar que el servicio está vivo |

Ejemplo de evento:

```json
{
  "eventId": "a1-e05",
  "timestamp": "2026-10-01T15:00:10Z",
  "gameId": "arena-combat",
  "gameVersion": "1.0",
  "matchId": "match-a1",
  "playerId": "p1",
  "type": "ATTACK",
  "data": { "targetPlayerId": "p2", "weapon": "rifle", "hit": true, "damage": 40 }
}
```

Respuestas:

| Código | Significado |
|---|---|
| `202` | Evento aceptado |
| `200` | Duplicado idéntico; no se cuenta dos veces |
| `400` | Evento inválido; indica el campo y el motivo |
| `409` | Conflicto: id repetido con otro contenido, partida ya cerrada o evento tardío |
| `413` | Evento o partida demasiado grandes |
| `503` | Sistema saturado; reintentar más tarde |

El protocolo completo, incluida la comunicación entre Go y Scala, está en [docs/protocol.md](docs/protocol.md).

---

## 8. Configuración

Los valores por defecto sirven para probar en local. Para cambiarlos, copia `.env.example` a `.env` y edítalo. Docker Compose lo lee solo. Las variables más útiles son:

| Variable | Por defecto | Qué controla |
|---|---|---|
| `INGEST_PORT` | `8080` | Puerto publicado del servicio Go |
| `INGEST_API_KEY` | vacío | Si tiene valor, los `POST` exigen la cabecera `X-API-Key`. Obligatoria en un servidor público |
| `MATCH_GRACE_PERIOD` | `2s` | Cuánto se esperan eventos rezagados después de `MATCH_FINISHED` |
| `MATCH_IDLE_TIMEOUT` | `15s` | Tras cuánto tiempo sin eventos una partida se cierra y se analiza como `INCOMPLETE` |
| `ANALYZE_MAX_RETRIES` | `3` | Reintentos si Scala no responde |
| `RESULTS_DIR` | `/data` | Dónde se guardan los resultados; sobreviven a un reinicio (volumen `gamestats-data`) |

---

## 9. Problemas frecuentes

| Síntoma | Causa y solución |
|---|---|
| `Cannot connect to the Docker daemon` | Docker no está abierto. Inicia Docker Desktop y espera a que diga *running* |
| `port is already allocated` (8080) | Otro programa usa el puerto. Ciérralo, o levanta el sistema con `INGEST_PORT=9090 docker compose up -d --wait` |
| `curl: (7) Failed to connect` o la app dice «ingestión no responde» | El sistema no está levantado o sigue arrancando. Corre `docker compose up -d --wait` |
| `409 MATCH_CLOSED` o `409 LATE_EVENT` al repetir una prueba | La partida ya se cerró y no se reabre (es intencional). Usa otro sufijo (`--suffix -2`, `S=-2`) o `docker compose down -v` |
| `command not found: mk` (o `ev`, `res`) | Esa terminal no tiene las funciones auxiliares. Ejecuta `source scripts/pruebas.sh` |
| zsh muestra errores con líneas que empiezan con `#` | No pegues los comentarios, o ejecuta `setopt interactivecomments` |
| `jq: command not found` | Instala jq (`brew install jq` o `sudo apt install jq`) |
| La app no compila: falta `cc`, `gcc` o `GL/gl.h` | Falta el compilador de C o las librerías gráficas. Revisa la tabla de [4.1](#41-compilar-y-abrir-la-aplicación) |
| La app dice «no se encontró la carpeta contracts/» | Ejecútala dentro de la carpeta del repositorio, o pásale `--contracts <ruta>/contracts` |
| Un resultado sigue en `COLLECTING` | Falta el `MATCH_FINISHED`, o no han pasado los 2 s de gracia. Espera unos segundos; sin final, la partida se cierra sola a los 15 s |

---

## 10. Estructura del repositorio

| Carpeta | Qué contiene |
|---|---|
| [`ingestion-go/`](ingestion-go/) | Servicio Go: validación con JSON Schema, una goroutine por partida, reintentos hacia Scala, almacenamiento de resultados y API |
| [`analytics-scala/`](analytics-scala/) | Servicio Scala 3: núcleo funcional común y un analizador por juego |
| [`simulator/`](simulator/) | Simulador de partidas (CLI) y aplicación de escritorio (`cmd/gui`) |
| [`contracts/`](contracts/) | La fuente de verdad compartida: esquemas JSON, ejemplos válidos e inválidos, partidas de prueba y sus resultados calculados a mano |
| [`tests/e2e/`](tests/e2e/) | Prueba de punta a punta sobre Docker Compose |
| [`scripts/`](scripts/) | Demo guiada (`demo.sh`) y funciones para pruebas manuales (`pruebas.sh`) |
| [`deploy/`](deploy/) | Compose y `.env` de ejemplo para un servidor |
| [`docs/`](docs/) | Documentación técnica |

---

## 11. Desarrollo, pruebas automáticas y CI/CD

Con Go 1.26, JDK 21 y sbt instalados:

```bash
make test      # pruebas de Go (con detector de carreras) y de Scala
make gui       # solo compila la aplicación de escritorio en bin/
make fmt       # formatea el código Go
```

| Suite | Cantidad | Comando |
|---|---|---|
| Go `ingestion-go` (unitarias y de sistema, con `-race`) | 19 | `cd ingestion-go && go test -race ./...` |
| Go `simulator` (simulador y lógica de la app) | 14 | `cd simulator && go test -race ./...` |
| Aplicación de escritorio | 2 | `cd simulator && go test -tags gui ./cmd/gui` |
| Scala (resultados esperados, propiedades, HTTP) | 42 | `cd analytics-scala && sbt test` |
| De punta a punta | 23 comprobaciones | `make e2e` |

La matriz completa de casos está en [docs/testing.md](docs/testing.md).

**CI/CD con GitHub Actions:**

| Workflow | Cuándo corre | Qué hace |
|---|---|---|
| `ci.yml` | En cada push y pull request | Formato, `vet`, compilación, pruebas de Go y Scala, construcción de las imágenes Docker, prueba de punta a punta y ejecutable de escritorio para Linux, macOS y Windows |
| `release.yml` | Push a `main` y tags `v*` | Publica las imágenes (amd64 y arm64) en GitHub Container Registry y, si están configurados los deploy hooks, redespliega en Render |
| `deploy.yml` | Manual | Despliega una versión en el servidor por SSH y comprueba que responda |

**Despliegue:** la forma recomendada es **Render**, gratis y sin tarjeta. El archivo [`render.yaml`](render.yaml) define los dos servicios y se despliega en *New → Blueprint*. También se puede usar una VM con Docker Compose. Los pasos de ambas opciones están en [docs/deployment.md](docs/deployment.md).

---

## 12. Documentación técnica

| Documento | Contenido |
|---|---|
| [Arquitectura y concurrencia](docs/architecture.md) | Componentes, goroutines y canales, estados de una partida, qué se pierde al reiniciar |
| [Especificación de eventos](docs/event-spec.md) | Formato de los eventos, para quien quiera producirlos |
| [Protocolo](docs/protocol.md) | API de Go y contrato entre Go y Scala |
| [Estadísticas y patrones](docs/statistics.md) | Cómo se deriva cada métrica, con ejemplos |
| [Decisiones de arquitectura](docs/decisions.md) | Por qué se eligió cada cosa y qué se descartó |
| [Aplicación de escritorio](docs/gui.md) | Detalles de la interfaz gráfica |
| [Pruebas](docs/testing.md) | Matriz de casos de prueba |
| [Deployment](docs/deployment.md) | Despliegue en un servidor |
| [Guion de demo](docs/demo.md) | Demostración paso a paso |
| [Defensa](docs/defense.md) | Imperativo frente a funcional y dónde verlo en el código |

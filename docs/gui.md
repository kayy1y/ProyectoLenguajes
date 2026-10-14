# Aplicación de escritorio

`gamestats-gui` es una ventana nativa (no una página web) para evaluar el sistema sin terminal: ver partidas y estadísticas, generar datos con el simulador, ejecutar la verificación y enviar eventos a mano.

Es una **herramienta de apoyo**: corre en la máquina del evaluador y solo usa la API pública de la ingestión (`:8080`). No participa en el análisis: Go sigue ingiriendo y Scala sigue calculando, cada uno en su contenedor.

## Uso

```bash
docker compose up -d --build --wait     # el sistema, como siempre
make gui-run                            # compila bin/gamestats-gui y abre la ventana
```

Sin `make`:

```bash
cd simulator
go build -tags gui -o ../bin/gamestats-gui ./cmd/gui
../bin/gamestats-gui
```

Contra un servidor remoto: `bin/gamestats-gui --target http://<ip>:8080 --api-key <clave>`.

La app busca la carpeta `contracts/` subiendo desde el directorio actual y desde la carpeta del ejecutable. Si se copia el binario a otro lugar, hay que pasarle `--contracts <ruta>`.

### Requisitos para compilarla

Go 1.26 y un compilador de C, porque la ventana usa [Fyne](https://fyne.io), que dibuja con OpenGL:

| Sistema | Qué instalar |
|---|---|
| macOS | `xcode-select --install` |
| Windows | gcc de MSYS2 o TDM-GCC |
| Linux (Debian/Ubuntu) | `sudo apt install gcc libgl1-mesa-dev xorg-dev` |

Si no se quiere compilar, CI publica el ejecutable de los tres sistemas como artefacto (`gamestats-gui-Linux`, `-macOS` y `-Windows`) en cada ejecución de *Actions → CI*.

## Pestañas

| Pestaña | Para qué sirve |
|---|---|
| **Partidas** | Lista en vivo de las partidas, con filtro. Al seleccionar una se ven sus estadísticas por partida y por jugador, sus patrones y los avisos de calidad de datos. Un valor no calculable aparece como «—». Las columnas salen de las claves de `metrics`, así que la misma vista sirve para cualquier juego |
| **Generar datos** | Envía fixtures, el escenario `demo` o carga (`stress`) con semilla y velocidad. Muestra el progreso, las respuestas HTTP explicadas y si cada fixture coincide con el resultado calculado a mano |
| **Verificación** | Ejecuta la suite de 37 comprobaciones y guarda un informe Markdown para `docs/evidence/` |
| **Enviar evento** | Editor JSON con las plantillas válidas e inválidas del contrato; muestra el código HTTP y la respuesta |
| **Rechazos** | Últimos eventos rechazados con su código y campo |
| **Guía** | Recorrido de 5 pasos para el evaluador y catálogo de juegos |

## Suite de verificación (37 comprobaciones)

| Grupo | Cantidad | Comprobación |
|---|---|---|
| Casos de prueba | 8 | Cada fixture pasa por Go y Scala y el resultado es idéntico al de `contracts/expected` |
| Eventos inválidos | 17 | Cada ejemplo de `contracts/examples/invalid` da 400 y el **primer** error tiene el código y el campo de `expected.json` |
| Comportamiento | 12 | Evento válido, duplicado idéntico, conflicto de `eventId`, segundo `MATCH_STARTED` y `MATCH_FINISHED`, resultado de una partida abierta, análisis tras el período de gracia, evento tardío, partida inexistente y salud |

Cada ejecución usa un sufijo propio en `matchId` y `eventId`, porque las partidas cerradas no se reabren. Antes de comparar, el sufijo se quita del resultado; así la suite se repite sin reiniciar nada.

## Código

| Ruta | Contenido |
|---|---|
| `simulator/cmd/gui` | La ventana (Fyne). Se compila solo con `-tags gui`, para que el simulador, las imágenes Docker y el resto de CI no dependan de librerías gráficas |
| `simulator/internal/lab` | Lógica sin interfaz: catálogo leído de `contracts/`, generación de datos y suite de verificación (reusa `scenario` y `sender`), cliente de la API y formato de métricas |
| `simulator/internal/verify` | Comparación con `contracts/expected` y con los códigos de los inválidos; también la usa el CLI (`--expect`) |

Agregar un juego (esquema, analizador y fixture) no requiere cambiar la app: los juegos se leen de `contracts/schemas/games` y las columnas salen de los datos.

## Pruebas

- `cd simulator && go test -race ./...`: `lab` y `verify` (catálogo, comparación con sufijo, trabajos, solicitudes inválidas, formato).
- `go test -tags gui -run TestWindowBuildsWithoutIngestion ./cmd/gui`: la ventana se arma con sus 6 pestañas aunque la ingestión no responda. Lo corren `make test` y CI en Linux, macOS y Windows.
- `SNAPSHOT_DIR=<carpeta> go test -tags gui -run TestSnapshots ./cmd/gui`: con el sistema levantado, guarda una captura de cada pestaña, del detalle de una partida de cada juego y de la suite terminada. Falla si alguna comprobación de la suite falla.

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
| Windows | un gcc de **64 bits**: `winget install -e --id BrechtSanders.WinLibs.POSIX.UCRT` (o MSYS2 / TDM-GCC). El MinGW clásico de 32 bits no sirve |
| Linux (Debian/Ubuntu) | `sudo apt install gcc libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev wayland-protocols` |

Si no se quiere compilar, CI publica el ejecutable de los tres sistemas como artefacto (`gamestats-gui-Linux`, `-macOS` y `-Windows`) en cada ejecución de *Actions → CI*.

Para comprobar solo que la interfaz compila, sin compilador de C: `make gui-check` (la compila para WebAssembly y corre `go vet`).

## Pestañas

La cabecera muestra el servidor y un indicador de la ingestión (verde con el tiempo de respuesta, rojo si no responde).

| Pestaña | Para qué sirve |
|---|---|
| **Partidas** | Tarjetas con los totales (partidas, aceptados, rechazados, duplicados, tardíos) y lista en vivo de las partidas, con búsqueda, filtros por juego y estado y orden por cualquier columna (clic en el encabezado). Al seleccionar una se ven sus estadísticas por partida (en tarjetas) y por jugador, sus patrones y los avisos de calidad de datos. Un valor no calculable aparece como «—». Las columnas salen de las claves de `metrics`, así que la misma vista sirve para cualquier juego |
| **Generar datos** | Envía fixtures, el escenario `demo` o carga (`stress`) con semilla y velocidad. Valida la semilla y el número de partidas antes de enviar. Muestra el progreso, las respuestas HTTP con colores y explicadas, y si cada fixture coincide con el resultado calculado a mano |
| **Verificación** | Ejecuta la suite de 37 comprobaciones, con un resumen por grupo, y guarda un informe Markdown para `docs/evidence/` |
| **Enviar evento** | Editor JSON con las plantillas válidas e inválidas del contrato, aviso en vivo si el JSON no es válido y botón para formatearlo; muestra el código HTTP y la respuesta |
| **Rechazos** | Últimos eventos rechazados con su código (en color) y campo, con filtro por código |
| **Guía** | Recorrido de 5 pasos para el evaluador y catálogo de juegos |

Al terminar *Generar datos* o la *Verificación* la app envía una notificación del sistema.

## Menú y atajos

| Atajo | Acción |
|---|---|
| `Ctrl+1` … `Ctrl+6` (`Cmd` en macOS) | Ir a cada pestaña, en el orden de arriba (también en el menú **Ir a**) |
| **Ayuda → Acerca de** | Qué es GameStats y cómo se reparte el trabajo entre Go y Scala |

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
| `simulator/internal/lab` | Lógica sin interfaz: catálogo leído de `contracts/`, generación de datos y suite de verificación (reusa `scenario` y `sender`), cliente de la API, formato de métricas y orden de partidas |
| `simulator/internal/verify` | Comparación con `contracts/expected` y con los códigos de los inválidos; también la usa el CLI (`--expect`) |

Agregar un juego (esquema, analizador y fixture) no requiere cambiar la app: los juegos se leen de `contracts/schemas/games` y las columnas salen de los datos.

## Pruebas

- `cd simulator && go test -race ./...`: `lab` y `verify` (catálogo, comparación con sufijo, trabajos, solicitudes inválidas, formato).
- `go test -tags gui -run TestWindowBuildsWithoutIngestion ./cmd/gui`: la ventana se arma con sus 6 pestañas aunque la ingestión no responda. Lo corren `make test` y CI en Linux, macOS y Windows.
- `SNAPSHOT_DIR=<carpeta> go test -tags gui -run TestSnapshots ./cmd/gui`: con el sistema levantado, guarda una captura de cada pestaña, del detalle de una partida de cada juego y de la suite terminada. Falla si alguna comprobación de la suite falla.

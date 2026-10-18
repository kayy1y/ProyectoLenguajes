# Consola web

La consola web es una página para el navegador con las mismas seis pestañas que la [aplicación de escritorio](gui.md): **Partidas, Generar datos, Verificación, Enviar evento, Rechazos y Guía**. Existe para que se pueda probar el sistema desplegado sin instalar nada.

| Dónde | Dirección |
|---|---|
| Render | https://gamestats-ingestion.onrender.com |
| Tu computadora (`docker compose up -d --build --wait`) | http://localhost:8080 |

## Cómo encaja en la arquitectura

- La sirve el **mismo servicio Go** de ingestión, en `/` y `/ui/`, por lo que queda en la misma URL que la API y no necesita un servicio extra en Render ni configuración de CORS.
- Es una **herramienta de apoyo**, igual que la aplicación de escritorio: el JavaScript corre en el navegador y llama solo a la API pública (`/v1/events`, `/v1/matches`, `/v1/stats`, `/v1/rejections`, `/healthz`). Go sigue ingiriendo y Scala sigue calculando; la consola no calcula métricas.
- La API no cambió. Todas las rutas que no son `/` ni `/ui/...` llegan a la API exactamente como antes (lo comprueba `TestMountServesConsoleAndKeepsAPI`).
- Los casos de prueba salen de `contracts/` (la misma fuente de verdad que usan los tests y la aplicación de escritorio). Go los entrega en `GET /ui/contracts.json`. Si la carpeta falta, la página carga igual y solo se desactivan *Generar datos* y *Verificación*.

## Clave de API

En Render los envíos (`POST`) exigen la cabecera `X-API-Key`. La consola tiene un campo **Clave de API** arriba a la derecha; la clave se guarda solo en el navegador de quien la escribe (`localStorage`) y se envía únicamente al mismo servidor. Sin clave, todas las consultas funcionan; los envíos responden `401` y la consola explica por qué.

## Qué hace cada pestaña

| Pestaña | Qué hace |
|---|---|
| **Partidas** | Tarjetas con los totales de `/v1/stats`, tabla de partidas con búsqueda, filtros y orden por columna. Al seleccionar una se ven sus estadísticas, patrones y avisos de calidad. Un valor no calculable se muestra como «—» |
| **Generar datos** | Envía los 8 casos de prueba, uno solo, o el **escenario demo**: F01 con 3 eventos inválidos, un duplicado, un conflicto de `eventId` y un evento tardío intercalados; F17 (remontada); y una partida sin `MATCH_FINISHED` que se cierra sola por inactividad. Muestra cada respuesta explicada y si el resultado coincide con el calculado a mano |
| **Verificación** | La misma suite de 37 comprobaciones que la aplicación de escritorio (8 casos de prueba, 17 inválidos, 12 de comportamiento). Permite descargar el informe en Markdown |
| **Enviar evento** | Editor con las plantillas válidas e inválidas del contrato, botón para formatear, botón para generar IDs nuevos y la respuesta del sistema |
| **Rechazos** | Últimos rechazos con su código y campo, con filtro por código |
| **Guía** | Recorrido sugerido y juegos soportados |

Cada ejecución usa un sufijo propio en `matchId` y `eventId` (por ejemplo `match-a1-wmve888n4`), porque las partidas cerradas no se reabren. Antes de comparar con `contracts/expected`, el sufijo se quita del resultado.

## Código

| Ruta | Contenido |
|---|---|
| `ingestion-go/internal/webui/webui.go` | Rutas `/` y `/ui/`, y el paquete de contratos (`/ui/contracts.json`) |
| `ingestion-go/internal/webui/static/` | `index.html`, `app.js` y `style.css`, incrustados en el binario con `go:embed`. Sin dependencias externas ni paso de compilación |
| `ingestion-go/Dockerfile` | Copia `contracts/fixtures`, `contracts/expected` y `contracts/examples` a la imagen, junto a los esquemas |

## Pruebas

- `cd ingestion-go && go test ./internal/webui`: la página y sus archivos se sirven, la API sigue recibiendo todo lo demás, el paquete de contratos trae 8 fixtures y 17 inválidos, y la página carga aunque falten los contratos.
- Probada en un navegador sin interfaz contra Docker Compose con `INGEST_API_KEY` configurada (como en Render): el escenario demo da F01 y F17 idénticos a lo calculado a mano y la partida abierta termina `INCOMPLETE`; la verificación da 37 de 37.

## Actualizar la versión de Render

La consola va dentro de la imagen `gamestats-ingestion`. Tras un push a `main`, `release.yml` publica la imagen nueva; Render la usa en el siguiente despliegue (automático si están configurados los deploy hooks, o desde *Dashboard → gamestats-ingestion → Manual Deploy → Deploy latest reference*). Ver [deployment.md](deployment.md).

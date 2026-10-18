# Guía de la entrega

Esta página responde una pregunta: **¿dónde está cada cosa que pide el enunciado?** Cada entregable apunta al archivo donde se encuentra, y al final hay un recorrido corto para probar el proyecto.

## Enlaces rápidos

| | |
|---|---|
| **Repositorio** | https://github.com/kayy1y/ProyectoLenguajes |
| **Versión desplegada** | https://gamestats-ingestion.onrender.com: en el navegador abre la consola web (comprobar con `/healthz`) |
| **Servicio de análisis** | https://gamestats-analytics.onrender.com/health |
| **Pruebas automáticas (CI)** | https://github.com/kayy1y/ProyectoLenguajes/actions |
| **Imágenes Docker publicadas** | https://github.com/kayy1y?tab=packages |

> La versión desplegada usa el plan gratuito de Render: si nadie la usa durante 15 minutos se duerme, y la primera petición tarda alrededor de un minuto en despertarla. Conviene abrir `/healthz` antes de probarla.
> Para enviar eventos nuevos al servicio desplegado hace falta una clave de API. Está en el **documento adjunto a la entrega** («Links e información importante»); en los comandos aparece como `<clave>` y hay que reemplazarla completa, incluidos los signos `<` y `>`. Las consultas (`GET`) no la necesitan.

## Los entregables del enunciado y dónde están

| Entregable que pide el enunciado | Dónde está |
|---|---|
| Código fuente de Go | [`ingestion-go/`](../ingestion-go/) |
| Código fuente de Scala | [`analytics-scala/`](../analytics-scala/) |
| Simulador de la fuente de eventos | [`simulator/`](../simulator/) |
| Especificación del formato de eventos | [`event-spec.md`](event-spec.md) y los esquemas en [`contracts/schemas/`](../contracts/schemas/) |
| Tipos de juegos considerados y justificación de los eventos | [`games.md`](games.md) |
| Estadísticas y reglas implementadas | [`statistics.md`](statistics.md) |
| Protocolo entre Go y Scala | [`protocol.md`](protocol.md) |
| Dockerfiles y configuración para ejecutar | [`ingestion-go/Dockerfile`](../ingestion-go/Dockerfile), [`analytics-scala/Dockerfile`](../analytics-scala/Dockerfile), [`simulator/Dockerfile`](../simulator/Dockerfile) y [`docker-compose.yml`](../docker-compose.yml) |
| Pipeline CI/CD | [`.github/workflows/`](../.github/workflows/): `ci.yml`, `release.yml` y `deploy.yml` |
| Documentación de instalación y ejecución | [`README.md`](../README.md) |
| Documentación de despliegue | [`deployment.md`](deployment.md) y [`render.yaml`](../render.yaml) |
| Demostración funcional | [`demo.md`](demo.md), [`scripts/demo.sh`](../scripts/demo.sh), la aplicación de escritorio, la [consola web](web.md) y la versión desplegada |

## Otros documentos que ayudan a entender el diseño

| Documento | Para qué sirve |
|---|---|
| [`GameStats-Arquitectura-y-Decisiones.pdf`](GameStats-Arquitectura-y-Decisiones.pdf) | Explicación completa de la arquitectura, las decisiones y la solución, lista para descargar |
| [`architecture.md`](architecture.md) | Componentes, concurrencia en Go, ciclo de vida de una partida y qué se pierde al reiniciar |
| [`decisions.md`](decisions.md) | Cada decisión de diseño con su motivo y las alternativas descartadas |
| [`defense.md`](defense.md) | Imperativo frente a funcional, y dónde verlo en el código |
| [`testing.md`](testing.md) | Los casos de prueba y cómo correrlos |
| [`gui.md`](gui.md) | La aplicación de escritorio para probar el sistema |
| [`web.md`](web.md) | La consola web, la misma herramienta en el navegador (local o en Render) |

## Probarlo en cinco minutos

**Sin instalar nada**, con la versión en la nube: abre https://gamestats-ingestion.onrender.com en el navegador. Es la [consola web](web.md): pega la clave de API arriba a la derecha, ve a **Generar datos → Escenario demo → Ejecutar**, mira las partidas en **Partidas** y ejecuta la **Verificación** (37 comprobaciones).

Para comprobar solo que está vivo:

```bash
curl -s https://gamestats-ingestion.onrender.com/healthz
```

Debe responder `{"status":"ok"}`. Todas las formas de probarlo están en el README, sección [Formas de probar el sistema](../README.md#formas-de-probar-el-sistema).

**En tu computadora**, solo con Docker:

```bash
git clone https://github.com/kayy1y/ProyectoLenguajes.git
cd ProyectoLenguajes
docker compose up -d --build --wait
docker compose --profile demo run --rm simulator --fixture all --expect
```

Al final deben salir ocho líneas que empiezan con `ok`: el sistema produjo exactamente los resultados que calculamos a mano para cada partida de prueba.

Después, la forma más cómoda de explorar es la aplicación de escritorio (`make gui-run`), con su pestaña **Guía** y la **Verificación**, que ejecuta 37 comprobaciones. Todas las opciones, con el detalle de qué debe salir en cada una, están en el [README](../README.md).

## Qué demuestra cada parte del proyecto

| Requisito del enunciado | Cómo se ve | Dónde mirarlo |
|---|---|---|
| Go concentra la ingestión y la concurrencia | Una goroutine por partida, un despachador y un pool de análisis | [`architecture.md`](architecture.md) |
| Scala concentra el análisis funcional | Funciones puras, `foldLeft`, `collect`, `groupMapReduce` | [`defense.md`](defense.md) |
| Estadística por jugador y por partida | Precisión, daño, mejor vuelta, duración, ganador | [`statistics.md`](statistics.md) |
| Estadística que combina tipos de evento | Eficiencia de eliminación, eliminaciones por minuto, posiciones ganadas | [`statistics.md`](statistics.md) |
| Regla basada en una secuencia | `KILL_STREAK` y `COMEBACK` | [`games.md`](games.md) |
| Sistema genérico para varios juegos | Dos juegos distintos sobre el mismo núcleo | [`architecture.md`](architecture.md), sección de extensibilidad |
| Docker, CI/CD y despliegue | Imágenes publicadas y servicio en Render | [`deployment.md`](deployment.md) |

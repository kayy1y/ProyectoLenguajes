package gamestats.domain

import java.time.Instant

/**
 * Tipos de ciclo de vida comunes a todos los juegos; el núcleo los entiende sin conocer el juego.
 * MatchStarted y PlayerJoined conservan su `data` para los atributos propios de cada juego
 * (p. ej. el mapa o la posición de salida), que solo el analizador del juego interpreta.
 */
enum Lifecycle:
  case MatchStarted(data: ujson.Value)
  case PlayerJoined(data: ujson.Value)
  case PlayerLeft(reason: String)
  case MatchFinished(reason: String)

object Lifecycle:
  val types: Set[String] = Set("MATCH_STARTED", "PLAYER_JOINED", "PLAYER_LEFT", "MATCH_FINISHED")

/** Evento tipado. `body` es Left para el ciclo de vida común y Right para la carga propia del juego `P`. */
final case class Event[+P](
    id: String,
    ts: Instant,
    rawTimestamp: String,
    player: Option[String],
    body: Either[Lifecycle, P]
)

/** Patrón secuencial detectado. `extra` lleva los campos propios del patrón (p. ej. length). */
final case class PatternHit(
    kind: String,
    playerId: String,
    fromEventId: String,
    toEventId: String,
    from: String,
    to: String,
    extra: List[(String, ujson.Value)]
):
  def toJson: ujson.Value =
    ujson.Obj.from(
      List[(String, ujson.Value)](
        "type" -> kind,
        "playerId" -> playerId,
        "fromEventId" -> fromEventId,
        "toEventId" -> toEventId,
        "from" -> from,
        "to" -> to
      ) ++ extra
    )

package gamestats.core

import java.time.Instant

import scala.collection.immutable.ListMap

import gamestats.domain.*

/** Métricas con nombre, en orden estable de presentación. `None` se serializa como null. */
type Metrics = ListMap[String, ujson.Value]

/** Datos de una partida ya ordenados y depurados, listos para calcular. */
final case class MatchContext[P](
    events: List[Event[P]],
    players: List[String],
    completeness: String,
    start: Option[Instant],
    finish: Option[Instant]
):
  /** Duración en segundos; None si falta el inicio o el fin (no se asume información ausente). */
  val durationSeconds: Option[Double] =
    for s <- start; f <- finish yield (f.toEpochMilli - s.toEpochMilli) / 1000.0

  /** Cargas propias del juego junto al jugador que las origina. */
  def payloads: List[(Option[String], P)] = events.collect { case Event(_, _, _, p, Right(payload)) => (p, payload) }

/**
 * Lo único que un juego nuevo implementa. El núcleo (orden, semántica común,
 * métricas comunes, armado del resultado) no se modifica para agregar juegos.
 */
trait GameAnalyzer[P]:
  def gameId: String
  def versions: Set[String]

  /** Decodifica un tipo propio del juego. Left con un aviso si el tipo no existe o la carga no encaja. */
  def decode(eventType: String, data: ujson.Value): Either[(String, String), P]

  /** Regla semántica propia del juego. `joined` son los jugadores unidos hasta ese momento. */
  def check(event: Event[P], joined: Set[String]): Option[(String, String)]

  def playerMetrics(ctx: MatchContext[P]): Map[String, Metrics]
  def matchMetrics(ctx: MatchContext[P], players: Map[String, Metrics]): Metrics
  def patterns(ctx: MatchContext[P]): List[PatternHit]

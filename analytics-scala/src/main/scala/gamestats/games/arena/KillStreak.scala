package gamestats.games.arena

import gamestats.domain.*

/**
 * Patrón secuencial KILL_STREAK: un jugador elimina a `minLength` o más rivales
 * seguidos sin ser eliminado ni salir de la partida entre medio.
 *
 * Depende del orden de los eventos, no de un conteo: la misma cantidad de
 * eliminaciones puede formar racha o no según dónde caiga la muerte.
 */
object KillStreak:
  val Kind = "KILL_STREAK"
  val DefaultMinLength = 3

  private type Run = Vector[Event[ArenaPayload]]

  /** Estado del fold: rachas abiertas por jugador y rachas ya cerradas. */
  private final case class State(open: Map[String, Run], closed: Vector[(String, Run)]):
    def close(player: String): State =
      open.get(player).fold(this)(run => State(open - player, closed :+ (player -> run)))
    def extend(player: String, e: Event[ArenaPayload]): State =
      copy(open = open.updated(player, open.getOrElse(player, Vector.empty) :+ e))

  def detect(events: List[Event[ArenaPayload]], minLength: Int = DefaultMinLength): List[PatternHit] =
    val end = events.foldLeft(State(Map.empty, Vector.empty)) { (state, e) =>
      (e.player, e.body) match
        case (Some(killer), Right(ArenaPayload.Elimination(victim, _))) => state.extend(killer, e).close(victim)
        case (Some(p), Left(Lifecycle.PlayerLeft(_)))                    => state.close(p)
        case _                                                           => state
    }
    (end.closed ++ end.open.toVector)
      .filter((_, run) => run.size >= minLength)
      .sortBy((player, run) => (run.head.ts, player))
      .map(toHit)
      .toList

  private def toHit(player: String, run: Run): PatternHit =
    PatternHit(Kind, player, run.head.id, run.last.id, run.head.rawTimestamp, run.last.rawTimestamp, List("length" -> run.size))

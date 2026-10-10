package gamestats.games.arena

import gamestats.domain.*

/** Métricas de arena-combat. Vertical slice: solo eliminaciones por jugador. */
object ArenaAnalyzer:
  val gameId = "arena-combat"
  val versions = Set("1.0")

  /** Conteo de ELIMINATION por eliminador: collect + groupMapReduce, sin estado mutable. */
  def eliminations(events: List[RawEvent]): Map[String, Int] =
    events
      .collect { case e if e.`type` == "ELIMINATION" => e.playerId }
      .flatten
      .groupMapReduce(identity)(_ => 1)(_ + _)

  def playerStats(players: List[String], events: List[RawEvent]): List[PlayerStats] =
    val elims = eliminations(events)
    players.map(p => PlayerStats(p, Map("eliminations" -> ujson.Num(elims.getOrElse(p, 0)))))

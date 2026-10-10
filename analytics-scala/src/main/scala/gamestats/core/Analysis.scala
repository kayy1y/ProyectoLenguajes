package gamestats.core

import java.time.Instant

import gamestats.domain.*
import gamestats.games.arena.ArenaAnalyzer

/** Núcleo puro: valida el lote, lo ordena y delega las métricas en el analizador del juego. */
object Analysis:

  /** Orden total del contrato: (timestamp, seq ausente = +inf, orden de llegada). */
  def ordered(events: List[RawEvent]): List[RawEvent] =
    events.sortBy(e => (Instant.parse(e.timestamp), e.seq.getOrElse(Long.MaxValue), e.ingest.seq))

  /** Jugadores que hicieron PLAYER_JOINED, sin repetir y ordenados. */
  def participants(events: List[RawEvent]): List[String] =
    events.collect { case e if e.`type` == "PLAYER_JOINED" => e.playerId }.flatten.distinct.sorted

  def analyze(req: AnalyzeRequest): Either[AnalysisError, AnalysisResult] =
    for
      _ <- Either.cond(req.events.nonEmpty, (), AnalysisError.EmptyBatch)
      _ <- Either.cond(req.gameId == ArenaAnalyzer.gameId, (), AnalysisError.UnsupportedGame(req.gameId))
      _ <- Either.cond(
        ArenaAnalyzer.versions.contains(req.gameVersion),
        (),
        AnalysisError.UnsupportedVersion(req.gameId, req.gameVersion)
      )
    yield
      val events = ordered(req.events)
      val players = participants(events)
      AnalysisResult(
        resultVersion = "1",
        gameId = req.gameId,
        gameVersion = req.gameVersion,
        matchId = req.matchId,
        completeness = req.completeness,
        input = InputSummary(events.size, events.size, 0),
        matchStats = MatchStats(ujson.Null, players.size, events.size, Map.empty),
        playerStats = ArenaAnalyzer.playerStats(players, events),
        patterns = Nil,
        warnings = Nil
      )

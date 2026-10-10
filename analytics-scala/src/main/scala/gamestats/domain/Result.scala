package gamestats.domain

import upickle.default.*

/** Resultado del análisis (contracts/schemas/analysis-result.v1.schema.json). */
final case class InputSummary(eventsReceived: Int, eventsUsed: Int, eventsIgnored: Int) derives ReadWriter

final case class MatchStats(
    durationSeconds: ujson.Value,
    participants: Int,
    totalEvents: Int,
    metrics: Map[String, ujson.Value]
) derives ReadWriter

final case class PlayerStats(playerId: String, metrics: Map[String, ujson.Value]) derives ReadWriter

final case class Warning(eventId: String, code: String, message: String) derives ReadWriter

final case class AnalysisResult(
    resultVersion: String,
    gameId: String,
    gameVersion: String,
    matchId: String,
    completeness: String,
    input: InputSummary,
    matchStats: MatchStats,
    playerStats: List[PlayerStats],
    patterns: List[ujson.Value],
    warnings: List[Warning]
) derives ReadWriter

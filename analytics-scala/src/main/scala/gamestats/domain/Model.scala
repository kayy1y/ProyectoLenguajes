package gamestats.domain

import upickle.default.*

/** Campos que Go agrega a cada evento al aceptarlo. */
final case class Ingest(receivedAt: String, seq: Long) derives ReadWriter

/** Evento tal como llega en el lote (contracts/schemas/analyze-request.v1.schema.json). */
final case class RawEvent(
    eventId: String,
    timestamp: String,
    gameId: String,
    gameVersion: String,
    matchId: String,
    playerId: Option[String] = None,
    `type`: String,
    seq: Option[Long] = None,
    data: ujson.Value,
    ingest: Ingest
) derives ReadWriter

final case class AnalyzeRequest(
    contractVersion: String,
    batchId: String,
    gameId: String,
    gameVersion: String,
    matchId: String,
    completeness: String,
    closedReason: String,
    events: List[RawEvent]
) derives ReadWriter

/** Errores del análisis; cada uno sabe su código de contrato y su estado HTTP. */
enum AnalysisError(val code: String, val status: Int, val message: String):
  case InvalidBatch(detail: String) extends AnalysisError("INVALID_BATCH", 400, detail)
  case EmptyBatch extends AnalysisError("EMPTY_BATCH", 400, "events must not be empty")
  case UnsupportedGame(gameId: String) extends AnalysisError("UNSUPPORTED_GAME", 422, s"game '$gameId' is not supported")
  case UnsupportedVersion(gameId: String, version: String)
      extends AnalysisError("UNSUPPORTED_VERSION", 422, s"version '$version' of '$gameId' is not supported")

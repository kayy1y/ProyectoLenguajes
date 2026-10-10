package gamestats.api

import scala.util.Try

import gamestats.core.Analysis
import gamestats.domain.*

/** Traduce entre HTTP y el núcleo. No contiene lógica de análisis. */
object Http:
  final case class Response(status: Int, body: String)

  def analyze(body: String): Response =
    val result = for
      req <- Try(upickle.default.read[AnalyzeRequest](body)).toEither.left
        .map(e => AnalysisError.InvalidBatch(Option(e.getMessage).getOrElse(e.toString)))
      res <- Analysis.analyze(req)
    yield res
    result match
      case Right(res) => Response(200, upickle.default.write(res))
      case Left(err)  => Response(err.status, ujson.write(ujson.Obj("code" -> err.code, "message" -> err.message)))

package gamestats

import java.nio.file.{Files, Path}

import gamestats.api.Http
import gamestats.core.Analysis
import gamestats.domain.*

class VerticalSliceSpec extends munit.FunSuite:
  private val contracts = Path.of(sys.props.getOrElse("contracts.dir", "../contracts"))
  private def read(rel: String) = Files.readString(contracts.resolve(rel))
  private val f01 = upickle.default.read[AnalyzeRequest](read("examples/protocol/analyze-request.F01.json"))

  test("F01: eliminaciones por jugador calculadas a mano (p1=3, p2=1, p3=0)") {
    val res = Analysis.analyze(f01).toOption.get
    val elims = res.playerStats.map(p => p.playerId -> p.metrics("eliminations").num.toInt)
    assertEquals(elims, List("p1" -> 3, "p2" -> 1, "p3" -> 0))
  }

  test("el resultado no depende del orden de llegada") {
    val shuffled = f01.copy(events = f01.events.reverse)
    assertEquals(Analysis.analyze(shuffled), Analysis.analyze(f01))
  }

  test("playerId ausente se decodifica como None y seq es opcional") {
    val start = f01.events.head
    assertEquals(start.`type`, "MATCH_STARTED")
    assertEquals(start.playerId, None)
    assertEquals(f01.events(1).playerId, Some("p1"))
  }

  test("HTTP: lote vacío -> 400 EMPTY_BATCH; juego desconocido -> 422; JSON roto -> 400") {
    val empty = Http.analyze(upickle.default.write(f01.copy(events = Nil)))
    assertEquals(empty.status, 400)
    assertEquals(ujson.read(empty.body)("code").str, "EMPTY_BATCH")
    val chess = Http.analyze(upickle.default.write(f01.copy(gameId = "chess")))
    assertEquals(chess.status, 422)
    assertEquals(Http.analyze("{not json").status, 400)
  }

  test("HTTP: la respuesta serializa métricas por jugador como objeto JSON") {
    val res = Http.analyze(read("examples/protocol/analyze-request.F01.json"))
    assertEquals(res.status, 200)
    val json = ujson.read(res.body)
    assertEquals(json("playerStats")(0)("metrics")("eliminations").num, 3.0)
    assertEquals(json("matchStats")("durationSeconds"), ujson.Null)
  }

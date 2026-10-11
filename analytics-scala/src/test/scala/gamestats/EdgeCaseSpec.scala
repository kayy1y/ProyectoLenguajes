package gamestats

import gamestats.api.Http
import gamestats.core.{Analysis, Fn}

class EdgeCaseSpec extends munit.FunSuite:
  private val f01 = Fixtures.events("F01-arena-normal")

  test("partida sin MATCH_FINISHED: duración y eliminaciones por minuto en null, sin asumir el fin") {
    val r = Analysis.analyze(Fixtures.request(f01.filterNot(_.`type` == "MATCH_FINISHED"))).toOption.get
    assertEquals(r.completeness, "INCOMPLETE")
    assertEquals(r.matchStats.durationSeconds, ujson.Null)
    assertEquals(r.matchStats.metrics("eliminationsPerMinute"), ujson.Null)
    assertEquals(r.matchStats.metrics("totalEliminations").num, 4.0)
  }

  test("partida de duración cero: eliminaciones por minuto en null (sin dividir entre cero)") {
    val start = f01.head
    val finish = f01.last.copy(timestamp = start.timestamp)
    val r = Analysis.analyze(Fixtures.request(List(start, finish))).toOption.get
    assertEquals(r.matchStats.durationSeconds.num, 0.0)
    assertEquals(r.matchStats.metrics("eliminationsPerMinute"), ujson.Null)
    assertEquals(r.matchStats.metrics("topPlayerId"), ujson.Null)
  }

  test("tipo desconocido: aviso UNKNOWN_EVENT_TYPE y evento excluido") {
    val weird = f01(5).copy(eventId = "zz", `type` = "DANCE")
    val r = Analysis.analyze(Fixtures.request(f01 :+ weird)).toOption.get
    assertEquals(r.warnings.map(w => (w.eventId, w.code)), List("zz" -> "UNKNOWN_EVENT_TYPE"))
    assertEquals(r.input.eventsIgnored, 1)
  }

  test("safeRatio nunca produce NaN ni infinito") {
    assertEquals(Fn.safeRatio(3, 0), None)
    assertEquals(Fn.safeRatio(0, 0), None)
    assertEquals(Fn.safeRatio(3, 5), Some(0.6))
  }

  test("redondeo a 4 decimales y enteros sin decimales") {
    assertEquals(Fn.num(2.0 / 3), ujson.Num(0.6667))
    assertEquals(upickle.default.write(Fn.num(120.0)), "120")
  }

  test("playerId ausente se decodifica como None") {
    assertEquals(f01.head.playerId, None)
    assertEquals(f01(1).playerId, Some("p1"))
  }

  test("HTTP: lote vacío 400, juego o versión desconocidos 422, JSON roto 400, otra partida 400") {
    val req = Fixtures.request("F01-arena-normal")
    def call(r: gamestats.domain.AnalyzeRequest) = Http.analyze(upickle.default.write(r))
    def code(res: Http.Response) = ujson.read(res.body)("code").str
    assertEquals((call(req.copy(events = Nil)).status, code(call(req.copy(events = Nil)))), (400, "EMPTY_BATCH"))
    val chess = req.copy(gameId = "chess", events = req.events.map(_.copy(gameId = "chess")))
    assertEquals((call(chess).status, code(call(chess))), (422, "UNSUPPORTED_GAME"))
    assertEquals(code(call(req.copy(gameVersion = "9.0"))), "UNSUPPORTED_VERSION")
    assertEquals(code(call(req.copy(gameId = "chess"))), "INVALID_BATCH")
    assertEquals(Http.analyze("{not json").status, 400)
    assertEquals(code(call(req.copy(matchId = "otra"))), "INVALID_BATCH")
  }

  test("HTTP: la petición de ejemplo del contrato produce el resultado esperado de F01") {
    val res = Http.analyze(Fixtures.read("examples/protocol/analyze-request.F01.json"))
    assertEquals(res.status, 200)
    assertEquals(ujson.read(res.body), Fixtures.expected("F01-arena-normal"))
  }

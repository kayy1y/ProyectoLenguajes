package gamestats

import java.nio.file.{Files, Path}

import scala.jdk.CollectionConverters.*

import gamestats.domain.*

/** Carga los fixtures de contracts/ y los convierte en la petición que Go enviaría. */
object Fixtures:
  val contracts: Path = Path.of(sys.props.getOrElse("contracts.dir", "../contracts"))

  def read(rel: String): String = Files.readString(contracts.resolve(rel))

  lazy val names: List[String] = ujson.read(read("fixtures/index.json")).obj.keys.toList.sorted

  /** Eventos en orden de envío; ingest.seq = posición de llegada. */
  def events(name: String): List[RawEvent] =
    Files.readAllLines(contracts.resolve(s"fixtures/$name.jsonl")).asScala.toList
      .filter(_.trim.nonEmpty)
      .zipWithIndex
      .map { (line, i) =>
        val obj = ujson.read(line).obj
        obj("ingest") = ujson.Obj("receivedAt" -> "2026-10-09T18:00:00.000Z", "seq" -> i)
        upickle.default.read[RawEvent](obj)
      }

  def request(events: List[RawEvent]): AnalyzeRequest =
    val first = events.head
    val complete = events.exists(_.`type` == "MATCH_FINISHED")
    AnalyzeRequest("1", s"${first.gameId}:${first.matchId}:1", first.gameId, first.gameVersion, first.matchId,
      if complete then "COMPLETE" else "INCOMPLETE", if complete then "MATCH_FINISHED" else "IDLE_TIMEOUT", events)

  def request(name: String): AnalyzeRequest = request(events(name))

  def expected(name: String): ujson.Value = ujson.read(read(s"expected/$name.result.json"))

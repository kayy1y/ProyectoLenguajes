package gamestats.games.racing

import scala.collection.immutable.ListMap
import scala.util.Try

import gamestats.core.*
import gamestats.core.Fn.*
import gamestats.domain.*

/** Carga propia de street-racing (contracts/schemas/games/street-racing.v1.schema.json). */
enum RacingPayload:
  case Lap(lapNumber: Int, lapTimeMs: Long, valid: Boolean)
  case Position(position: Int, lapNumber: Int)
  case Finished(finalPosition: Int, totalTimeMs: Long)

import RacingPayload.*

object RacingAnalyzer extends GameAnalyzer[RacingPayload]:
  val gameId = "street-racing"
  val versions = Set("1.0")

  /** Umbral de COMEBACK: haber estado en esta posición o peor y terminar primero. */
  val ComebackFromPosition = 3

  def decode(eventType: String, data: ujson.Value): Either[(String, String), RacingPayload] =
    def parse(f: => RacingPayload) =
      Try(f).toEither.left.map(e => ("INVALID_PAYLOAD", s"data de $eventType no encaja: ${e.getMessage}"))
    eventType match
      case "LAP_COMPLETED"    => parse(Lap(data("lapNumber").num.toInt, data("lapTimeMs").num.toLong, data("valid").bool))
      case "POSITION_CHANGED" => parse(Position(data("position").num.toInt, data("lapNumber").num.toInt))
      case "PLAYER_FINISHED"  => parse(Finished(data("finalPosition").num.toInt, data("totalTimeMs").num.toLong))
      case other              => Left(("UNKNOWN_EVENT_TYPE", s"tipo '$other' no pertenece a $gameId"))

  def check(event: Event[RacingPayload], joined: Set[String]): Option[(String, String)] = None

  /** Posición de salida, leída del PLAYER_JOINED (atributo propio de este juego). */
  def grid(ctx: MatchContext[RacingPayload]): Map[String, Int] =
    ctx.events.collect {
      case Event(_, _, _, Some(p), Left(Lifecycle.PlayerJoined(data))) if data.objOpt.exists(_.contains("gridPosition")) =>
        p -> data("gridPosition").num.toInt
    }.toMap

  def laps(ctx: MatchContext[RacingPayload]): List[(String, Lap)] =
    ctx.payloads.collect { case (Some(p), lap: Lap) => p -> lap }

  def finals(ctx: MatchContext[RacingPayload]): Map[String, Int] =
    ctx.payloads.collect { case (Some(p), Finished(pos, _)) => p -> pos }.toMap

  def playerMetrics(ctx: MatchContext[RacingPayload]): Map[String, Metrics] =
    val byPlayer = laps(ctx).groupMap(_._1)(_._2)
    val (grids, finished) = (grid(ctx), finals(ctx))
    ctx.players.map { p =>
      val (valid, invalid) = byPlayer.getOrElse(p, Nil).partition(_.valid)
      val validTimes = valid.map(_.lapTimeMs.toDouble)
      // Métrica combinada: PLAYER_JOINED.gridPosition - PLAYER_FINISHED.finalPosition.
      val gained = for g <- grids.get(p); f <- finished.get(p) yield (g - f).toDouble
      p -> ListMap[String, ujson.Value](
        "validLaps" -> valid.size,
        "invalidLaps" -> invalid.size,
        "bestValidLapMs" -> opt(validTimes.minOption),
        "avgValidLapMs" -> opt(safeRatio(validTimes.sum, validTimes.size)),
        "gridPosition" -> opt(grids.get(p).map(_.toDouble)),
        "finalPosition" -> opt(finished.get(p).map(_.toDouble)),
        "positionsGained" -> opt(gained)
      )
    }.toMap

  def matchMetrics(ctx: MatchContext[RacingPayload], players: Map[String, Metrics]): Metrics =
    val finished = finals(ctx)
    val fastest = laps(ctx).filter(_._2.valid).sortBy((p, lap) => (lap.lapTimeMs, p)).headOption
    ListMap[String, ujson.Value](
      "finishers" -> finished.size,
      "dnf" -> (ctx.players.size - finished.size),
      "fastestLapMs" -> opt(fastest.map(_._2.lapTimeMs.toDouble)),
      "fastestLapPlayerId" -> fastest.fold(ujson.Null)(f => ujson.Str(f._1)),
      "winnerPlayerId" -> finished.collectFirst { case (p, 1) => ujson.Str(p) }.getOrElse(ujson.Null)
    )

  /**
   * COMEBACK: la trayectoria de posiciones de un jugador (salida y luego cada
   * POSITION_CHANGED, en orden) alcanza ComebackFromPosition o peor y después
   * termina primero. Depende del orden: terminar primero sin haber estado atrás no cuenta.
   */
  def patterns(ctx: MatchContext[RacingPayload]): List[PatternHit] =
    val indexed = ctx.events.zipWithIndex
    // (orden, evento, posición) de cada jugador: salida y luego cada cambio de posición.
    val trajectories: Map[String, List[(Int, Event[RacingPayload], Int)]] =
      indexed
        .collect {
          case (e @ Event(_, _, _, Some(p), Left(Lifecycle.PlayerJoined(d))), i) if d.objOpt.exists(_.contains("gridPosition")) =>
            p -> (i, e, d("gridPosition").num.toInt)
          case (e @ Event(_, _, _, Some(p), Right(Position(pos, _))), i) => p -> (i, e, pos)
        }
        .groupMap(_._1)(_._2)
    val wins = indexed.collect { case (e @ Event(_, _, _, Some(p), Right(Finished(1, _))), i) => (p, e, i) }
    wins
      .flatMap { (player, finish, finishIdx) =>
        val before = trajectories.getOrElse(player, Nil).filter(_._1 < finishIdx)
        before.map(_._3).maxOption.filter(_ >= ComebackFromPosition).flatMap { worst =>
          before.find(_._3 == worst).map { (_, from, _) =>
            (from.ts, PatternHit("COMEBACK", player, from.id, finish.id, from.rawTimestamp, finish.rawTimestamp,
              List("worstPosition" -> ujson.Num(worst))))
          }
        }
      }
      .sortBy((ts, hit) => (ts, hit.playerId))
      .map(_._2)

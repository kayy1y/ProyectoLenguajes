package gamestats.games.arena

import scala.collection.immutable.ListMap
import scala.util.Try

import gamestats.core.*
import gamestats.core.Fn.*
import gamestats.domain.*

/** Carga propia de arena-combat (contracts/schemas/games/arena-combat.v1.schema.json). */
enum ArenaPayload:
  case Attack(target: String, weapon: String, hit: Boolean, damage: Int)
  case Elimination(victim: String, weapon: String)
  case ObjectiveCaptured(objectiveId: String)

import ArenaPayload.*

/** Acumulador inmutable por jugador. */
final case class Tally(attacks: Int, hits: Int, damage: Int, eliminations: Int, deaths: Int, objectives: Int)

object Tally:
  val zero: Tally = Tally(0, 0, 0, 0, 0, 0)

object ArenaAnalyzer extends GameAnalyzer[ArenaPayload]:
  val gameId = "arena-combat"
  val versions = Set("1.0")

  def decode(eventType: String, data: ujson.Value): Either[(String, String), ArenaPayload] =
    def parse(f: => ArenaPayload) =
      Try(f).toEither.left.map(e => ("INVALID_PAYLOAD", s"data de $eventType no encaja: ${e.getMessage}"))
    eventType match
      case "ATTACK" =>
        parse(Attack(data("targetPlayerId").str, data("weapon").str, data("hit").bool, data("damage").num.toInt))
      case "ELIMINATION"        => parse(Elimination(data("victimPlayerId").str, data("weapon").str))
      case "OBJECTIVE_CAPTURED" => parse(ObjectiveCaptured(data("objectiveId").str))
      case other                => Left(("UNKNOWN_EVENT_TYPE", s"tipo '$other' no pertenece a $gameId"))

  def check(event: Event[ArenaPayload], joined: Set[String]): Option[(String, String)] =
    (event.player, event.body) match
      case (Some(p), Right(Elimination(v, _))) if v == p =>
        Some(("SELF_ELIMINATION", s"el jugador '$p' no puede eliminarse a sí mismo"))
      case (_, Right(Elimination(v, _))) if !joined(v) =>
        Some(("UNKNOWN_VICTIM", s"victimPlayerId '$v' no se unió a la partida"))
      case (Some(p), Right(Attack(t, _, _, _))) if t == p =>
        Some(("SELF_TARGET", s"el jugador '$p' no puede atacarse a sí mismo"))
      case (_, Right(Attack(t, _, _, _))) if !joined(t) =>
        Some(("UNKNOWN_TARGET", s"targetPlayerId '$t' no se unió a la partida"))
      case _ => None

  /** Un solo recorrido con foldLeft: cada evento actualiza el acumulador de quien corresponde. */
  def tally(ctx: MatchContext[ArenaPayload]): Map[String, Tally] =
    val start = ctx.players.map(_ -> Tally.zero).toMap
    ctx.payloads.foldLeft(start) {
      case (acc, (Some(p), Attack(_, _, hit, damage))) =>
        acc.updatedWith(p)(_.map(t =>
          t.copy(attacks = t.attacks + 1, hits = t.hits + (if hit then 1 else 0), damage = t.damage + (if hit then damage else 0))
        ))
      case (acc, (Some(k), Elimination(v, _))) =>
        acc
          .updatedWith(k)(_.map(t => t.copy(eliminations = t.eliminations + 1)))
          .updatedWith(v)(_.map(t => t.copy(deaths = t.deaths + 1)))
      case (acc, (Some(p), ObjectiveCaptured(_))) =>
        acc.updatedWith(p)(_.map(t => t.copy(objectives = t.objectives + 1)))
      case (acc, _) => acc
    }

  def playerMetrics(ctx: MatchContext[ArenaPayload]): Map[String, Metrics] =
    tally(ctx).view.mapValues { t =>
      ListMap[String, ujson.Value](
        "attacks" -> t.attacks,
        "hits" -> t.hits,
        "accuracy" -> opt(safeRatio(t.hits, t.attacks)),
        "damageDealt" -> t.damage,
        "eliminations" -> t.eliminations,
        "deaths" -> t.deaths,
        "objectivesCaptured" -> t.objectives,
        "killEfficiency" -> opt(safeRatio(t.eliminations, t.attacks))
      )
    }.toMap

  def matchMetrics(ctx: MatchContext[ArenaPayload], players: Map[String, Metrics]): Metrics =
    def total(name: String) = players.values.map(_(name).num).sum
    val eliminations = total("eliminations")
    val perMinute = ctx.durationSeconds.flatMap(d => safeRatio(eliminations, d / 60))
    val top = ctx.players
      .sortBy(p => (-players(p)("eliminations").num, -players(p)("damageDealt").num, p))
      .headOption
    ListMap[String, ujson.Value](
      "totalEliminations" -> num(eliminations),
      "totalDamage" -> num(total("damageDealt")),
      "eliminationsPerMinute" -> opt(perMinute),
      "topPlayerId" -> top.fold(ujson.Null)(ujson.Str(_))
    )

  def patterns(ctx: MatchContext[ArenaPayload]): List[PatternHit] = KillStreak.detect(ctx.events)

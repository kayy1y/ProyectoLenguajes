package gamestats.core

import java.time.Instant

import scala.util.Try

import gamestats.domain.*

/** Núcleo puro: ninguna función de este objeto hace I/O ni muta estado. */
object Analysis:

  /** Orden total del contrato: (timestamp, seq ausente = +inf, orden de llegada). */
  def ordered(events: List[RawEvent]): List[RawEvent] =
    events.sortBy(e => (Instant.parse(e.timestamp), e.seq.getOrElse(Long.MaxValue), e.ingest.seq))

  /** Lifecycle común o carga del juego; Left con un aviso si el evento no se puede tipar. */
  def decode[P](game: GameAnalyzer[P])(raw: RawEvent): Either[Warning, Event[P]] =
    def str(field: String) = raw.data.objOpt.flatMap(_.get(field)).flatMap(_.strOpt).getOrElse("")
    val body: Either[(String, String), Either[Lifecycle, P]] = raw.`type` match
      case "MATCH_STARTED"  => Right(Left(Lifecycle.MatchStarted))
      case "PLAYER_JOINED"  => Right(Left(Lifecycle.PlayerJoined))
      case "PLAYER_LEFT"    => Right(Left(Lifecycle.PlayerLeft(str("reason"))))
      case "MATCH_FINISHED" => Right(Left(Lifecycle.MatchFinished(str("reason"))))
      case other            => game.decode(other, raw.data).map(Right(_))
    body
      .map(b => Event(raw.eventId, Instant.parse(raw.timestamp), raw.timestamp, raw.playerId, b))
      .left
      .map((code, msg) => Warning(raw.eventId, code, msg))

  /**
   * Reglas semánticas en orden de eventos. Un evento que rompe una regla genera un
   * aviso y se excluye. La primera regla es común; el resto la decide el juego.
   */
  def semantics[P](game: GameAnalyzer[P])(decoded: List[Either[Warning, Event[P]]]): (List[Event[P]], List[Warning]) =
    final case class Acc(joined: Set[String], kept: List[Event[P]], warnings: List[Warning])
    val end = decoded.foldLeft(Acc(Set.empty, Nil, Nil)) {
      case (acc, Left(w)) => acc.copy(warnings = w :: acc.warnings)
      case (acc, Right(e)) =>
        val rule = e.body match
          case Left(Lifecycle.PlayerJoined) => None
          case _ =>
            e.player
              .filterNot(acc.joined)
              .map(p => ("UNKNOWN_PLAYER", s"playerId '$p' no se unió a la partida"))
              .orElse(game.check(e, acc.joined))
        rule match
          case Some((code, msg)) => acc.copy(warnings = Warning(e.id, code, msg) :: acc.warnings)
          case None =>
            val joined = if e.body == Left(Lifecycle.PlayerJoined) then acc.joined ++ e.player else acc.joined
            acc.copy(joined = joined, kept = e :: acc.kept)
    }
    (end.kept.reverse, end.warnings.reverse)

  def context[P](completeness: String, events: List[Event[P]]): MatchContext[P] =
    def first(l: Lifecycle => Boolean) = events.collectFirst { case Event(_, ts, _, _, Left(x)) if l(x) => ts }
    MatchContext(
      events = events,
      players = events.collect { case Event(_, _, _, Some(p), Left(Lifecycle.PlayerJoined)) => p }.distinct.sorted,
      completeness = completeness,
      start = first(_ == Lifecycle.MatchStarted),
      finish = first { case Lifecycle.MatchFinished(_) => true; case _ => false }
    )

  /** Pipeline de un juego concreto: decode -> semántica -> contexto -> métricas -> resultado. */
  def run[P](game: GameAnalyzer[P], req: AnalyzeRequest): AnalysisResult =
    val (kept, warnings) = (ordered andThen (_.map(decode(game))) andThen semantics(game))(req.events)
    val ctx = context(req.completeness, kept)
    val players = game.playerMetrics(ctx)
    AnalysisResult(
      resultVersion = "1",
      gameId = req.gameId,
      gameVersion = req.gameVersion,
      matchId = req.matchId,
      completeness = req.completeness,
      input = InputSummary(req.events.size, kept.size, req.events.size - kept.size),
      matchStats = MatchStats(
        durationSeconds = Fn.opt(ctx.durationSeconds),
        participants = ctx.players.size,
        totalEvents = kept.size,
        metrics = game.matchMetrics(ctx, players)
      ),
      playerStats = ctx.players.map(p => PlayerStats(p, players(p))),
      patterns = game.patterns(ctx).map(_.toJson),
      warnings = warnings
    )

  def analyze(req: AnalyzeRequest): Either[AnalysisError, AnalysisResult] =
    for
      _ <- Either.cond(req.events.nonEmpty, (), AnalysisError.EmptyBatch)
      _ <- req.events
        .find(e => e.gameId != req.gameId || e.matchId != req.matchId)
        .toLeft(())
        .left
        .map(e => AnalysisError.InvalidBatch(s"event ${e.eventId} belongs to another match"))
      _ <- Try(req.events.foreach(e => Instant.parse(e.timestamp))).toEither.left
        .map(e => AnalysisError.InvalidBatch(s"invalid timestamp: ${e.getMessage}"))
      game <- Registry.lookup(req.gameId, req.gameVersion)
    yield run(game, req)

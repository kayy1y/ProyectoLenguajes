package gamestats

import java.time.Instant

import gamestats.domain.*
import gamestats.games.arena.{ArenaPayload, KillStreak}

class KillStreakSpec extends munit.FunSuite:
  private val t0 = Instant.parse("2026-10-01T15:00:00Z")
  private def at(i: Int, player: String, body: Either[Lifecycle, ArenaPayload]) =
    val ts = t0.plusSeconds(i.toLong)
    Event(s"e$i", ts, ts.toString, Some(player), body)
  private def kill(i: Int, k: String, v: String) = at(i, k, Right(ArenaPayload.Elimination(v, "rifle")))
  private def left(i: Int, p: String) = at(i, p, Left(Lifecycle.PlayerLeft("QUIT")))

  test("tres eliminaciones seguidas forman una racha") {
    val hits = KillStreak.detect(List(kill(1, "a", "b"), kill(2, "a", "c"), kill(3, "a", "d")))
    assertEquals(hits.map(h => (h.playerId, h.fromEventId, h.toEventId)), List(("a", "e1", "e3")))
  }

  test("una racha de 4 es un único patrón maximal") {
    val hits = KillStreak.detect((1 to 4).toList.map(i => kill(i, "a", s"v$i")))
    assertEquals(hits.map(_.extra), List(List("length" -> ujson.Num(4))))
  }

  test("dos eliminaciones no son racha") {
    assertEquals(KillStreak.detect(List(kill(1, "a", "b"), kill(2, "a", "c"))), Nil)
  }

  test("morir entre medio rompe la racha aunque el total sea 3") {
    assertEquals(KillStreak.detect(List(kill(1, "a", "b"), kill(2, "a", "c"), kill(3, "x", "a"), kill(4, "a", "d"))), Nil)
  }

  test("salir de la partida rompe la racha") {
    assertEquals(KillStreak.detect(List(kill(1, "a", "b"), kill(2, "a", "c"), left(3, "a"), kill(4, "a", "d"))), Nil)
  }

  test("rachas de jugadores distintos intercaladas se detectan por separado y en orden") {
    val events = List(kill(1, "a", "x"), kill(2, "b", "y"), kill(3, "a", "x"), kill(4, "b", "y"), kill(5, "a", "x"), kill(6, "b", "y"))
    assertEquals(KillStreak.detect(events).map(_.playerId), List("a", "b"))
  }

  test("el umbral es configurable") {
    assertEquals(KillStreak.detect(List(kill(1, "a", "b"), kill(2, "a", "c")), minLength = 2).size, 1)
  }

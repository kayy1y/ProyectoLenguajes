package gamestats

import scala.util.Random

import gamestats.core.Analysis

/** Propiedades que deben cumplirse para cualquier orden de llegada (transparencia referencial). */
class PropertySpec extends munit.FunSuite:
  private val seeds = 1 to 50

  Fixtures.names.foreach { name =>
    test(s"$name: cualquier permutación de llegada produce el mismo resultado") {
      val base = Fixtures.events(name)
      val expected = Analysis.analyze(Fixtures.request(base))
      seeds.foreach { seed =>
        val shuffled = new Random(seed).shuffle(base).zipWithIndex.map((e, i) => e.copy(ingest = e.ingest.copy(seq = i)))
        assertEquals(Analysis.analyze(Fixtures.request(shuffled)), expected, s"seed $seed")
      }
    }

    test(s"$name: invariantes de sumas") {
      val r = Analysis.analyze(Fixtures.request(name)).toOption.get
      def sum(metric: String) = r.playerStats.map(_.metrics(metric).num).sum
      def matchNum(metric: String) = r.matchStats.metrics(metric).num
      assertEquals(r.input.eventsUsed + r.input.eventsIgnored, r.input.eventsReceived)
      assertEquals(r.input.eventsIgnored, r.warnings.size)
      r.gameId match
        case "arena-combat" =>
          assertEquals(sum("eliminations"), matchNum("totalEliminations"))
          assertEquals(sum("deaths"), matchNum("totalEliminations"))
          assertEquals(sum("damageDealt"), matchNum("totalDamage"))
        case "street-racing" =>
          assertEquals(matchNum("finishers") + matchNum("dnf"), r.matchStats.participants.toDouble)
          assertEquals(r.playerStats.count(!_.metrics("finalPosition").isNull).toDouble, matchNum("finishers"))
        case other => fail(s"sin invariantes para $other")
    }
  }

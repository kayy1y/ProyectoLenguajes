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
      val total = r.matchStats.metrics("totalEliminations").num
      assertEquals(sum("eliminations"), total)
      assertEquals(sum("deaths"), total)
      assertEquals(sum("damageDealt"), r.matchStats.metrics("totalDamage").num)
      assertEquals(r.input.eventsUsed + r.input.eventsIgnored, r.input.eventsReceived)
      assertEquals(r.input.eventsIgnored, r.warnings.size)
    }
  }

package gamestats

import gamestats.core.Analysis

/** Compara el análisis de cada fixture con el resultado calculado a mano en contracts/expected. */
class GoldenSpec extends munit.FunSuite:
  test("hay fixtures registrados") {
    assert(Fixtures.names.size >= 6, Fixtures.names)
  }

  Fixtures.names.foreach { name =>
    test(s"$name coincide con contracts/expected") {
      val result = Analysis.analyze(Fixtures.request(name)).fold(e => fail(e.message), identity)
      val actual = ujson.read(upickle.default.write(result))
      assertEquals(actual, Fixtures.expected(name))
    }
  }

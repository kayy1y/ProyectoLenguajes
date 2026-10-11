package gamestats.core

import scala.math.BigDecimal.RoundingMode

/** Utilidades puras reutilizadas por todos los juegos. */
object Fn:

  /** División segura: None cuando el denominador es 0 (nunca NaN ni infinito). */
  def safeRatio(n: Double, d: Double): Option[Double] = Option.when(d != 0)(n / d)

  /** Cuenta cuántas veces aparece cada clave que `key` logra extraer. */
  def countBy[A, K](xs: List[A])(key: A => Option[K]): Map[K, Int] =
    xs.flatMap(key).groupMapReduce(identity)(_ => 1)(_ + _)

  def round4(x: Double): Double = BigDecimal(x).setScale(4, RoundingMode.HALF_UP).toDouble

  /** Número JSON: enteros sin decimales, el resto redondeado a 4 cifras. */
  def num(x: Double): ujson.Value = if x.isWhole then ujson.Num(x) else ujson.Num(round4(x))

  def opt(x: Option[Double]): ujson.Value = x.fold(ujson.Null)(num)

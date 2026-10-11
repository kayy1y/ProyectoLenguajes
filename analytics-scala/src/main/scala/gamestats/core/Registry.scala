package gamestats.core

import gamestats.domain.AnalysisError
import gamestats.games.arena.ArenaAnalyzer

/** Juegos soportados. Agregar un juego = agregar su analizador a esta lista. */
object Registry:
  val games: Map[String, GameAnalyzer[?]] =
    List[GameAnalyzer[?]](ArenaAnalyzer).map(g => g.gameId -> g).toMap

  def lookup(gameId: String, version: String): Either[AnalysisError, GameAnalyzer[?]] =
    games.get(gameId) match
      case None                                     => Left(AnalysisError.UnsupportedGame(gameId))
      case Some(g) if !g.versions.contains(version) => Left(AnalysisError.UnsupportedVersion(gameId, version))
      case Some(g)                                  => Right(g)

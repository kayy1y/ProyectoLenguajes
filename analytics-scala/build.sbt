ThisBuild / scalaVersion := "3.9.0"
ThisBuild / organization := "gamestats"

lazy val root = (project in file("."))
  .settings(
    name := "analytics-scala",
    scalacOptions ++= Seq("-deprecation", "-feature", "-Wunused:all"),
    libraryDependencies ++= Seq(
      "com.lihaoyi" %% "cask" % "0.11.3",
      "com.lihaoyi" %% "upickle" % "4.4.3",
      "org.scalameta" %% "munit" % "1.3.6" % Test
    ),
    // Los tests leen contracts/ desde la raíz del repositorio.
    Test / fork := true,
    Test / javaOptions += s"-Dcontracts.dir=${(ThisBuild / baseDirectory).value.getParentFile / "contracts"}",
    assembly / test := {},
    assembly / mainClass := Some("gamestats.Main"),
    assembly / assemblyJarName := "analytics.jar",
    assembly / assemblyMergeStrategy := {
      case PathList("META-INF", "versions", _, "module-info.class") => MergeStrategy.discard
      case "module-info.class"                                      => MergeStrategy.discard
      case PathList("META-INF", "services", _*)                     => MergeStrategy.concat
      case PathList("META-INF", _*)                                 => MergeStrategy.discard
      case x =>
        val old = (assembly / assemblyMergeStrategy).value
        old(x)
    }
  )

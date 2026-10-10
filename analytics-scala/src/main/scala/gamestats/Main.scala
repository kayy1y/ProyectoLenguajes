package gamestats

import gamestats.api.Http

/** Servidor HTTP (borde impuro). Las rutas solo delegan en api.Http. */
object Main extends cask.MainRoutes:
  override def host: String = "0.0.0.0"
  override def port: Int = sys.env.get("HTTP_PORT").flatMap(_.toIntOption).getOrElse(8081)

  private val json = Seq("Content-Type" -> "application/json")

  @cask.get("/health")
  def health() = cask.Response("""{"status":"ok"}""", 200, json)

  @cask.post("/v1/analyze")
  def analyze(request: cask.Request) =
    val res = Http.analyze(request.text())
    val requestId = request.headers.get("x-request-id").flatMap(_.headOption).getOrElse("")
    println(s"""{"component":"analytics","path":"/v1/analyze","status":${res.status},"requestId":"$requestId"}""")
    cask.Response(res.body, res.status, json)

  initialize()

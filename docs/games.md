# Los juegos que elegimos y por qué

Este documento cuenta qué juegos escogimos para GameStats, qué hechos registra cada uno y qué se puede aprender de ellos. Si quieres ver el formato exacto de cada evento, está en [event-spec.md](event-spec.md); y si te interesa cómo se calcula cada número, en [statistics.md](statistics.md).

## Por qué dos juegos tan distintos

El enunciado pide algo importante: que el sistema no dependa de un único juego. Podríamos haber elegido dos juegos parecidos, pero eso no habría demostrado nada. Si el mismo sistema funciona con dos juegos que casi no se parecen, entonces sí es genérico de verdad.

Por eso escogimos un juego de **combate** y uno de **carreras**. En el combate, los jugadores se afectan directamente: uno ataca, otro cae. En las carreras, cada piloto corre contra el reloj y contra los demás, pero nadie le hace nada a nadie. En uno importan las acciones y en el otro los tiempos y las posiciones. Y cada uno tiene su propio tipo de situación interesante: en el combate, una racha de eliminaciones; en las carreras, una remontada.

## Primer juego: `arena-combat` (combate)

Imagina una partida de todos contra todos. Cada jugador dispara a los demás, a veces acierta y a veces falla, elimina a algún rival, reaparece, y de vez en cuando captura un objetivo. No hay equipos: cada uno juega por su cuenta.

### Qué hechos registramos y por qué

- **Inicio y final de la partida** (`MATCH_STARTED`, `MATCH_FINISHED`). Sin ellos no sabríamos cuánto duró la partida, y sin la duración no podemos calcular cosas como eliminaciones por minuto.
- **Entrada de un jugador** (`PLAYER_JOINED`). Nos dice quiénes están jugando. Si de pronto aparece un evento de alguien que nunca entró, sabemos que algo no cuadra.
- **Ataque** (`ATTACK`). Guarda a quién se atacó, con qué arma, si acertó y cuánto daño hizo. Con esto sale la precisión y el daño total. Si el ataque falla, el daño tiene que ser cero: un fallo no puede hacer daño.
- **Eliminación** (`ELIMINATION`). Guarda quién eliminó a quién. De aquí salen las eliminaciones y las muertes de cada jugador, y también las rachas.
- **Captura de un objetivo** (`OBJECTIVE_CAPTURED`). Sirve para ver quién participa en los objetivos y no solo en el combate.
- **Salida de un jugador** (`PLAYER_LEFT`). Parece menor, pero importa: si alguien se desconecta en medio de una racha, la racha se acaba.

### Qué se puede saber de una partida

De cada **jugador** calculamos cuántos ataques hizo, cuántos acertó, qué porcentaje fue, cuánto daño causó, cuántas eliminaciones logró, cuántas veces cayó y cuántos objetivos capturó.

De la **partida** completa calculamos cuánto duró, cuántos jugadores participaron, cuántas eliminaciones y cuánto daño hubo en total, y quién fue el mejor jugador.

También calculamos dos números que **combinan tipos de evento distintos**, que es justo lo que pide el enunciado:
- La **eficiencia de eliminación**: cuántas eliminaciones logró un jugador por cada ataque. Mezcla los eventos de ataque con los de eliminación.
- Las **eliminaciones por minuto**: mezcla las eliminaciones con la duración de la partida.

Y detectamos un **patrón secuencial**: la **racha de eliminaciones**, que ocurre cuando un jugador elimina a tres o más rivales seguidos sin que lo eliminen y sin que se desconecte.

### Por qué una racha no se puede resolver contando

Dos jugadores pueden tener tres eliminaciones cada uno, y que solo uno tenga racha. ¿Cómo? Porque el otro murió entre la segunda y la tercera. Contar no distingue ese caso; hay que mirar el **orden** en que pasaron las cosas. Lo probamos con dos partidas de prueba: en una el jugador logra tres eliminaciones seguidas y se detecta la racha; en la otra, las mismas tres eliminaciones están separadas por una muerte, y no hay racha.

## Segundo juego: `street-racing` (carreras)

Aquí hay una pista y unos pilotos que salen en cierto orden. Cada uno da vueltas, cambia de posición durante la carrera y al final cruza la meta, o abandona.

### Qué hechos registramos y por qué

- **Inicio y final de la carrera** (`MATCH_STARTED`, `MATCH_FINISHED`). Igual que en el combate, nos dan la duración.
- **Entrada de un piloto** (`PLAYER_JOINED`). Aquí guarda algo muy importante: la **posición de salida**. Sin saber de dónde partió cada piloto, no podríamos saber cuánto ganó o perdió.
- **Vuelta completada** (`LAP_COMPLETED`). Guarda el número de vuelta, el tiempo y si la vuelta fue válida. Con esto sale la mejor vuelta y el promedio.
- **Cambio de posición** (`POSITION_CHANGED`). Registra cada vez que un piloto sube o baja de puesto. Es lo que nos permite reconstruir cómo evolucionó la carrera, y sin esto sería imposible detectar una remontada.
- **Llegada** (`PLAYER_FINISHED`). Guarda en qué posición terminó y su tiempo total.
- **Abandono** (`PLAYER_LEFT`). Un piloto que se retira no termina la carrera, y eso también es información.

### Qué se puede saber de una carrera

De cada **piloto** calculamos cuántas vueltas válidas e inválidas hizo, su mejor vuelta y el promedio de sus vueltas válidas, y en qué posición salió y llegó.

De la **carrera** completa calculamos cuántos terminaron, cuántos abandonaron, cuál fue la vuelta más rápida y de quién, y quién ganó.

Hay un número que **combina eventos distintos**: las **posiciones ganadas**, que es la posición de salida menos la de llegada. Un piloto que sale tercero y termina primero ganó dos posiciones. Mezcla la entrada del piloto con su llegada.

Y detectamos otro **patrón secuencial**: la **remontada**, cuando un piloto estuvo en tercer lugar o peor y después ganó la carrera.

### Por qué una remontada tampoco se resuelve contando

Ganar no basta para que haya remontada. Si un piloto sale primero y gana, no remontó nada, aunque se haya movido un poco en el camino. Para saber si hubo remontada hay que revisar la trayectoria de ese piloto: tiene que haber estado en tercer lugar o peor **antes** de llegar primero. Lo comprobamos con dos carreras de prueba: en una el ganador viene desde atrás y se detecta la remontada; en la otra, el ganador nunca bajó del primer puesto, y no hay remontada.

### Por qué las vueltas inválidas no cuentan

Cada vuelta dice si fue válida o no, por ejemplo, si el piloto se salió de la pista. Las vueltas inválidas las contamos aparte y no entran en la mejor vuelta ni en el promedio. Así, "la vuelta más rápida" es siempre un tiempo real y no uno que en la carrera no contaba.

## Lo que comparten los dos juegos

Aunque son muy distintos, los dos juegos usan los mismos cuatro hechos básicos de una partida: que empezó, que alguien entró, que alguien salió y que terminó. Esos cuatro los entiende el sistema sin saber de qué juego vienen. Todo lo demás es propio de cada juego, y por eso agregar uno nuevo no obliga a tocar lo que ya funciona.

## Cómo pensamos los eventos

Cuatro ideas guiaron el diseño:

1. **Los eventos cuentan lo que pasó, no lo que significa.** Ningún evento trae "eliminaciones" ni "precisión" ya calculadas. Esos números se calculan después, a partir de los hechos. Si alguien intenta enviar un evento con un campo así, el sistema lo rechaza.
2. **Cada evento lleva lo justo para poder relacionarse con otros.** Un ataque y una eliminación dicen quién es el afectado. Gracias a eso se pueden calcular la eficiencia y las rachas.
3. **Solo los eventos de jugador llevan jugador.** Que una partida empiece o termine no le pertenece a nadie; si alguien envía un jugador en esos eventos, se rechaza.
4. **Los datos tienen tipo y límites.** El daño no puede ser negativo y las posiciones tienen un rango. Un evento mal formado se detiene a la entrada, antes de llegar al análisis.

## Si quisiéramos agregar un tercer juego

Haría falta describir sus eventos en un archivo de esquema, escribir en Scala cómo se calculan sus estadísticas, registrarlo en una línea y preparar una partida de prueba con su resultado calculado a mano. Ya lo hicimos con el juego de carreras: ese cambio no tocó nada de la ingestión en Go, ni el núcleo de Scala, salvo una línea del registro. Esa es la mejor prueba de que el sistema es genérico.

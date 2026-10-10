# Contexto de build: raíz del repositorio.
FROM eclipse-temurin:21-jdk AS build
ARG SBT_VERSION=1.13.0
ADD https://github.com/sbt/sbt/releases/download/v${SBT_VERSION}/sbt-${SBT_VERSION}.tgz /tmp/sbt.tgz
RUN tar -xzf /tmp/sbt.tgz -C /opt && rm /tmp/sbt.tgz
ENV PATH=/opt/sbt/bin:$PATH
WORKDIR /build/analytics-scala
# Capa de dependencias: solo se rehace si cambia la definición del build.
COPY analytics-scala/project ./project
COPY analytics-scala/build.sbt ./
RUN sbt -batch update
COPY analytics-scala/src ./src
RUN sbt -batch assembly && cp target/scala-*/analytics.jar /build/analytics.jar

FROM eclipse-temurin:21-jre-alpine
RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /build/analytics.jar /app/analytics.jar
USER app
ENV HTTP_PORT=8081 \
    JAVA_OPTS="-XX:MaxRAMPercentage=75"
EXPOSE 8081
HEALTHCHECK --interval=5s --timeout=3s --start-period=30s --retries=12 CMD wget -qO- http://127.0.0.1:8081/health || exit 1
ENTRYPOINT ["sh", "-c", "exec java $JAVA_OPTS -jar /app/analytics.jar"]

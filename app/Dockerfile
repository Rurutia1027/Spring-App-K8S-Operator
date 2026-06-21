# Runtime image only — build the JAR locally first:
#   mvn -f app/pom.xml package -DskipTests
# Or: cd operator && make docker-build-app  (runs mvn then docker build)
FROM eclipse-temurin:21-jre
WORKDIR /app

COPY target/notes-service-0.1.0.jar app.jar

EXPOSE 8080

ENV SPRING_PROFILES_ACTIVE=prod
ENV SERVER_PORT=8080

ENTRYPOINT ["java", "-jar", "/app/app.jar"]

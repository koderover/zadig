FROM maven:3.5-jdk-8-alpine AS build

WORKDIR /code

RUN mkdir -p /root/.m2
COPY settings.xml /root/.m2/settings.xml
COPY pom.xml /code/pom.xml
RUN mvn --batch-mode dependency:resolve
RUN mvn --batch-mode verify

# Adding source, compile and package into a fat jar
COPY ["src/main", "/code/src/main"]
RUN mvn --batch-mode package

FROM openjdk:8-jre-alpine

COPY --from=build /code/target/worker-1.0-SNAPSHOT.jar /worker-jar-with-dependencies.jar

CMD ["java", "-XX:+UnlockExperimentalVMOptions", "-XX:+UseCGroupMemoryLimitForHeap", "-jar", "/worker-jar-with-dependencies.jar"]

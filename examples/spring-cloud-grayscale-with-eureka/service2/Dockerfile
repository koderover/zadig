# Start with a base image containing Java runtime
FROM openjdk:22-jdk-slim

# Add a volume pointing to /tmp
VOLUME /tmp

EXPOSE 8000
ARG JAR_FILE=target/*.jar
ADD ${JAR_FILE} app.jar

# Run the jar file
ENTRYPOINT ["java", "-jar", "/app.jar"]

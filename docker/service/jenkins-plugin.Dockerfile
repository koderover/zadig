#alpine.Dockerfile

WORKDIR /app

COPY docker/dist/jenkins-plugin /app/jenkins-plugin

ENTRYPOINT ["/app/jenkins-plugin"]

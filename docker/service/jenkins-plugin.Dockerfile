#alpine.Dockerfile

WORKDIR /app

ADD docker/dist/jenkins-plugin .

ENTRYPOINT ["/app/jenkins-plugin"]

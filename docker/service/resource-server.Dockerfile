#golang.Dockerfile

RUN go build -o /reaper ./cmd/reaper/main.go

#nginx.Dockerfile

WORKDIR /app

ADD resource-server-nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /reaper .

ADD http://resource.koderover.com/k8s-json-schema.tar.gz .
RUN tar -zxvf k8s-json-schema.tar.gz
RUN rm k8s-json-schema.tar.gz

EXPOSE 80

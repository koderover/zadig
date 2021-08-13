#golang.Dockerfile

RUN go build -o /reaper ./cmd/reaper/main.go

#nginx.Dockerfile

WORKDIR /app

ADD resource-server-nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /reaper .

EXPOSE 80

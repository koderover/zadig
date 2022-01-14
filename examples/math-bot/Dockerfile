From golang:1.17

WORKDIR /app

COPY . /app/

RUN GOOS=linux GOARCH=amd64 go build -o math-bot src/main.go

RUN chmod +x /app/math-bot

CMD ["/app/math-bot"]

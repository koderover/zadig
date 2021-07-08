
build-backend:
	go clean && GOOS=linux GOARCH=amd64 go build -o backend  -ldflags="-X main.BuildStamp=`date +%Y-%m-%d.%H:%M:%S`" backend.go

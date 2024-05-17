FROM golang:1.21

WORKDIR /notification-service

COPY go.mod .
COPY cmd/main.go .

RUN go build -o b .
ENTRYPOINT ["notification-service/bun"]
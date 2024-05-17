FROM golang:1.20

WORKDIR /notification-service
COPY ./ ./
RUN go mod download
RUN go build -o notification-service ./cmd/main.go

ENTRYPOINT ["/notification-service/bin"]
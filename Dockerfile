FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /migrator ./cmd/migrator/ && \
    go build -o /server ./cmd/server/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /migrator /app/migrator
COPY --from=builder /server /app/server

EXPOSE 8080

CMD ["/app/server"]

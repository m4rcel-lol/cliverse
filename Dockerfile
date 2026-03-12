FROM golang:1.22-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /cliverse ./cmd/cliverse

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 1000 cliverse
WORKDIR /app
COPY --from=builder /cliverse /app/cliverse
COPY migrations/ /app/migrations/
RUN mkdir -p /app/data && chown -R cliverse:cliverse /app
USER cliverse
EXPOSE 6969 8080
ENTRYPOINT ["/app/cliverse"]

FROM golang:1.25-trixie AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o server ./cmd/api/

# Final stage
FROM debian:trixie-slim

RUN useradd -m appuser

COPY --from=builder /app/server /opt/bin/server

RUN chown appuser:appuser /opt/bin/server
USER appuser

EXPOSE 8080
ENTRYPOINT ["/opt/bin/server"]

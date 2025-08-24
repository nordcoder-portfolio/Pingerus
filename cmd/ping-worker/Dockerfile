FROM golang:1.24 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /bin/ping-worker ./cmd/ping-worker

FROM gcr.io/distroless/base-debian12 AS final

COPY --from=builder /bin/ping-worker /bin/ping-worker
COPY --from=builder /app/internal/config /config

EXPOSE 8083

ENTRYPOINT ["/bin/ping-worker"]
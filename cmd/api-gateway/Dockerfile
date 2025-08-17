FROM golang:1.24 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /bin/api-gateway ./cmd/api-gateway

FROM gcr.io/distroless/base-debian12 AS final

COPY --from=builder /bin/api-gateway /bin/api-gateway
COPY --from=builder /app/internal/config /config

EXPOSE 8080 9090

ENTRYPOINT ["/bin/api-gateway"]

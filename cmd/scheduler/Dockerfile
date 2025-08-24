FROM golang:1.24 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o /bin/scheduler ./cmd/scheduler

FROM gcr.io/distroless/base-debian12 AS final

COPY --from=builder /bin/scheduler /bin/scheduler
COPY --from=builder /app/internal/config /config

EXPOSE 8082

ENTRYPOINT ["/bin/scheduler"]

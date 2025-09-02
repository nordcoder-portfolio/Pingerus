FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /kafka-init ./cmd/kafka-init

FROM gcr.io/distroless/static-debian12
COPY --from=build /kafka-init /kafka-init
ENTRYPOINT ["/kafka-init"]
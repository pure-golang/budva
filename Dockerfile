FROM dockerhub.timeweb.cloud/library/golang:1.25.9-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/facade ./cmd/facade
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/engine ./cmd/engine

FROM dockerhub.timeweb.cloud/library/debian:bookworm-slim

RUN adduser --disabled-password --gecos '' appuser
USER appuser

COPY --from=builder /bin/facade /facade
COPY --from=builder /bin/engine /engine
COPY --from=builder /app/ruleset.yml /ruleset.yml
COPY --from=builder /app/.env.example /.env

EXPOSE 7070
ENTRYPOINT ["/facade"]

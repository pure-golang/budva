# Stage 0: Сборка TDLib C++
FROM dockerhub.timeweb.cloud/library/debian:bookworm AS tdlib-builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake g++ gperf libssl-dev zlib1g-dev php-cli git make ca-certificates

RUN git clone https://github.com/tdlib/td.git /td && \
    cd /td && git checkout 22d49d5

RUN cd /td && mkdir build && cd build && \
    cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr/local .. && \
    cmake --build . --target prepare_cross_compiling && \
    cd .. && php SplitSource.php && cd build && \
    cmake --build . --target install

# Stage 1: Go builder
FROM dockerhub.timeweb.cloud/library/golang:1.25.9-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    libssl-dev zlib1g-dev && \
    rm -rf /var/lib/apt/lists/*

COPY --from=tdlib-builder /usr/local/include/td /usr/local/include/td/
COPY --from=tdlib-builder /usr/local/lib/libtd* /usr/local/lib/
RUN ldconfig

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=1 go build -a -trimpath -ldflags "-s -w" -o /bin/facade ./cmd/facade
RUN CGO_ENABLED=1 go build -a -trimpath -ldflags "-s -w" -o /bin/engine ./cmd/engine
RUN CGO_ENABLED=1 go build -a -trimpath -ldflags "-s -w" -o /bin/stand ./cmd/stand

# Stage 2: Runtime
FROM dockerhub.timeweb.cloud/library/debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libstdc++6 libssl3 zlib1g && \
    rm -rf /var/lib/apt/lists/*

COPY --from=tdlib-builder /usr/local/lib/libtd* /usr/local/lib/
RUN ldconfig

RUN adduser --disabled-password --gecos '' appuser && \
    mkdir -p /app && chown appuser:appuser /app
USER appuser
WORKDIR /app

COPY --from=builder --chown=appuser:appuser /bin/facade /app/facade
COPY --from=builder --chown=appuser:appuser /bin/engine /app/engine
COPY --from=builder --chown=appuser:appuser /bin/stand /app/stand
COPY --from=builder --chown=appuser:appuser /app/ruleset.yml /app/ruleset.yml
COPY --from=builder --chown=appuser:appuser /app/.env.example /app/.env

EXPOSE 7070

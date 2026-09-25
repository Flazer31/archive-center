# syntax=docker/dockerfile:1

# Build stage: produces the backend and the schema migration tool as static
# binaries so they run identically on amd64 and arm64 base images.
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go-service/go.mod go-service/go.sum ./go-service/
RUN cd go-service && go mod download
COPY go-service ./go-service
RUN cd go-service && \
    CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-s -w" \
      -o /out/archive-center-go ./cmd/archive-center-go && \
    CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-s -w" \
      -o /out/mariadb-schema ./cmd/mariadb-schema

FROM debian:bookworm-slim
# ca-certificates: outbound HTTPS to LLM providers and the update checker.
# curl: container healthcheck against /health.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 10001 archive-center \
    && useradd --system --uid 10001 --gid archive-center --create-home archive-center
COPY --from=build /out/archive-center-go /usr/local/bin/archive-center-go
COPY --from=build /out/mariadb-schema /usr/local/bin/mariadb-schema
COPY migrations /app/migrations
COPY prompts /app/prompts
RUN mkdir -p /data && chown -R archive-center:archive-center /data
USER archive-center
WORKDIR /app

ARG AC_BUILD_VERSION=""
ENV AC_BUILD_VERSION=${AC_BUILD_VERSION} \
    AC_BIND_ADDR=0.0.0.0:28080 \
    AC_MODE=live \
    AC_STORE_MODE=mariadb_authority \
    AC_RUNTIME_PROFILE=vector_external \
    AC_VECTOR_MODE=external \
    AC_CHROMA_API_PATH=/api/v2 \
    AC_PROMPT_DIR=/app/prompts \
    ARCHIVE_CENTER_DATA_DIR=/data \
    AC_UPDATE_ENABLED=false

EXPOSE 28080
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD ["curl", "--fail", "--silent", "http://127.0.0.1:28080/health"]

ENTRYPOINT ["/usr/local/bin/archive-center-go"]

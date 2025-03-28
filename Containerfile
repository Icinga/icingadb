FROM golang:1 AS base

# Cache dependencies:
# The go mod download command uses a cache mount,
# ensuring Go modules are cached separately from the build context and not included in image layers.
# This cache is used in the build stage and reused across builds, unless go.mod or go.sum changes.
WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

FROM base AS build

# Mount source code and build:
# The --mount=target=. option mounts the source code without adding an extra image layer, unlike `COPY . .`.
# The go build command uses the dependency cache and a dedicated mount to cache build artifacts for future builds.
RUN --mount=target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o /icingadb ./cmd/icingadb/main.go

FROM scratch

COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=build /icingadb /icingadb

COPY ./schema/mysql/schema.sql /schema/mysql/schema.sql
COPY ./schema/pgsql/schema.sql /schema/pgsql/schema.sql

# addgroup -g 1001 icinga
COPY <<EOF /etc/group
icinga:x:1001:
EOF

# adduser -u 1001 --no-create-home -h /var/empty -s /sbin/nologin --disabled-password -G icinga icinga
COPY <<EOF /etc/passwd
icinga:x:1001:1001::/var/empty:/sbin/nologin
EOF

USER icinga

CMD ["/icingadb", "--database-auto-import"]

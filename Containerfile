# TODO(el): Introduce GO_VERSION build argument.
FROM golang AS base

# Cache dependencies:
# The go mod download command uses a cache mount,
# ensuring Go modules are cached separately from the build context and not included in image layers.
# This cache is used in the build stage and reused across builds, unless go.mod or go.sum changes.
WORKDIR /build
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Mount source code and build:
# The --mount=target=. option mounts the source code into the build stage without creating an image layer.
# The go build command uses the dependency cache and a dedicated mount to cache build artifacts,
# speeding up future builds.
FROM base AS build
RUN --mount=target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags '-s -w' -o /icingadb cmd/icingadb/main.go

FROM scratch
COPY --from=build /icingadb /icingadb
CMD ["/icingadb"]

# =============================================================================
# Dockerfile — Quark Platform build verification
# =============================================================================
# Builds the entire project (Java + Go) in a clean container with no host
# dependencies. Used by `make docker-verify` to confirm the build doesn't
# depend on any local tool installations or paths.
#
# This is a BUILD verification image, not a runtime image. The runtime image
# would be smaller (only JRE + the quarkus-app) — see docker/Dockerfile.runtime
# (TODO: not yet implemented).
# =============================================================================

# ----- Stage 1: Build Java modules -----
FROM maven:3.9-eclipse-temurin-21 AS java-builder

WORKDIR /build
COPY pom.xml .
COPY mvnw mvnw.cmd .mvn ./
COPY quark-core-domain/pom.xml quark-core-domain/
COPY quark-core-parser/pom.xml quark-core-parser/
COPY quark-core-registry/pom.xml quark-core-registry/
COPY quark-core-event/pom.xml quark-core-event/
COPY quark-core-engine/pom.xml quark-core-engine/
COPY quark-adapter-store-duckdb/pom.xml quark-adapter-store-duckdb/
COPY quark-observability/pom.xml quark-observability/
COPY quark-core-script/pom.xml quark-core-script/
COPY quark-app/pom.xml quark-app/
COPY quark-api/pom.xml quark-api/
COPY quark-server/pom.xml quark-server/
COPY providers/pom.xml providers/
COPY providers/provider-stubs/pom.xml providers/provider-stubs/
COPY providers/provider-timer/pom.xml providers/provider-timer/
COPY providers/provider-cpu-profiler/pom.xml providers/provider-cpu-profiler/
COPY providers/provider-memory-profiler/pom.xml providers/provider-memory-profiler/
COPY providers/provider-list/pom.xml providers/provider-list/
COPY providers/provider-json-writer/pom.xml providers/provider-json-writer/
COPY providers/provider-streaming-endpoint/pom.xml providers/provider-streaming-endpoint/
COPY example/simple-streaming/runner/pom.xml example/simple-streaming/runner/

# Pre-fetch dependencies (cached layer)
RUN mvn -B dependency:go-offline -DskipTests || true

# Copy sources and build
COPY quark-core-domain/src quark-core-domain/src
COPY quark-core-parser/src quark-core-parser/src
COPY quark-core-registry/src quark-core-registry/src
COPY quark-core-event/src quark-core-event/src
COPY quark-core-engine/src quark-core-engine/src
COPY quark-adapter-store-duckdb/src quark-adapter-store-duckdb/src
COPY quark-observability/src quark-observability/src
COPY quark-core-script/src quark-core-script/src
COPY quark-app/src quark-app/src
COPY quark-api/src quark-api/src
COPY quark-server/src quark-server/src
COPY providers/provider-stubs/src providers/provider-stubs/src
COPY providers/provider-timer/src providers/provider-timer/src
COPY providers/provider-cpu-profiler/src providers/provider-cpu-profiler/src
COPY providers/provider-memory-profiler/src providers/provider-memory-profiler/src
COPY providers/provider-list/src providers/provider-list/src
COPY providers/provider-json-writer/src providers/provider-json-writer/src
COPY providers/provider-streaming-endpoint/src providers/provider-streaming-endpoint/src
COPY example/simple-streaming/runner/src example/simple-streaming/runner/src

RUN mvn -B clean install -DskipTests

# ----- Stage 2: Build Go CLI -----
FROM golang:1.24 AS go-builder

WORKDIR /build
COPY cli/go.mod cli/go.sum ./
RUN go mod download

COPY cli/ .
RUN go vet ./... && go test ./... && go build -trimpath -buildvcs=false -o /quarkctl .

# ----- Stage 3: Runtime image (JVM mode) -----
FROM eclipse-temurin:21-jre AS runtime-jvm

COPY --from=java-builder /build/quark-server/target/quark-server-0.1.0-SNAPSHOT-runner.jar /app/quark-server.jar
COPY --from=java-builder /build/quark-server/target/lib /app/lib
COPY --from=go-builder /quarkctl /app/quarkctl

# Smoke test
RUN /app/quarkctl --help | head -1
RUN java -version 2>&1 | head -1

WORKDIR /app
EXPOSE 8080 8081

# Default: run the JVM server
CMD ["java", "-jar", "quark-server.jar"]

# ----- Stage 4: Runtime image (native mode) -----
# To build: docker build --target runtime-native -t quark-native .
FROM debian:bookworm-slim AS runtime-native

# Install required shared libraries for native image
RUN apt-get update && apt-get install -y --no-install-recommends \
    libstdc++6 \
    zlib1g \
    && rm -rf /var/lib/apt/lists/*

COPY --from=java-builder /build/quark-server/target/quark-server-0.1.0-SNAPSHOT-runner /app/quark-server
COPY --from=go-builder /quarkctl /app/quarkctl

# Smoke test
RUN /app/quarkctl --help | head -1
RUN /app/quark-server --version 2>&1 | head -1 || true

WORKDIR /app
EXPOSE 8080 8081

# Default: run the native server
CMD ["./quark-server"]

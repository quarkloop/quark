# =============================================================================
# Dockerfile — Quark Platform build verification
# =============================================================================
# Builds the entire project (Java + Go) in a clean container with no host
# dependencies. Used by `make docker-verify` to confirm the build doesn't
# depend on any local tool installations or paths.
#
# This is a BUILD verification image, not a runtime image. The runtime image
# would be smaller (only JRE + the quarkus-app).
# =============================================================================

# ----- Stage 1: Build Java modules -----
FROM maven:3.9-eclipse-temurin-21 AS java-builder

WORKDIR /build

# Copy parent POM and Maven wrapper first (cached layer for deps)
COPY pom.xml .
COPY mvnw mvnw.cmd .mvn ./

# Copy all module POMs (the reactor layout is: core/, server/, runtime/)
COPY core/quark-domain/pom.xml core/quark-domain/
COPY core/quark-event/pom.xml core/quark-event/
COPY core/quark-registry/pom.xml core/quark-registry/
COPY core/quark-script/pom.xml core/quark-script/
COPY core/quark-engine/pom.xml core/quark-engine/
COPY server/quark-app/pom.xml server/quark-app/
COPY server/quark-api/pom.xml server/quark-api/
COPY server/quark-observability/pom.xml server/quark-observability/
COPY server/quark-server/pom.xml server/quark-server/
COPY runtime/quark-script/pom.xml runtime/quark-script/
COPY runtime/quark-polyglot/pom.xml runtime/quark-polyglot/
COPY runtime/quark-app/pom.xml runtime/quark-app/
COPY runtime/quark-runtime/pom.xml runtime/quark-runtime/
COPY runtime/providers/pom.xml runtime/providers/
COPY runtime/providers/provider-timer/pom.xml runtime/providers/provider-timer/
COPY runtime/providers/provider-cpu-profiler/pom.xml runtime/providers/provider-cpu-profiler/
COPY runtime/providers/provider-memory-profiler/pom.xml runtime/providers/provider-memory-profiler/
COPY runtime/providers/provider-json-writer/pom.xml runtime/providers/provider-json-writer/
COPY runtime/providers/provider-streaming-endpoint/pom.xml runtime/providers/provider-streaming-endpoint/

# Pre-fetch dependencies (cached layer). Tolerate failures here because
# the parent POM references some artifacts that aren't needed in this build.
RUN mvn -B dependency:go-offline -DskipTests || true

# Copy sources and build
COPY core/quark-domain/src core/quark-domain/src
COPY core/quark-event/src core/quark-event/src
COPY core/quark-registry/src core/quark-registry/src
COPY core/quark-script/src core/quark-script/src
COPY core/quark-engine/src core/quark-engine/src
COPY server/quark-app/src server/quark-app/src
COPY server/quark-api/src server/quark-api/src
COPY server/quark-observability/src server/quark-observability/src
COPY server/quark-server/src server/quark-server/src
COPY runtime/quark-script/src runtime/quark-script/src
COPY runtime/quark-polyglot/src runtime/quark-polyglot/src
COPY runtime/quark-app/src runtime/quark-app/src
COPY runtime/quark-runtime/src runtime/quark-runtime/src
COPY runtime/providers/provider-timer/src runtime/providers/provider-timer/src
COPY runtime/providers/provider-cpu-profiler/src runtime/providers/provider-cpu-profiler/src
COPY runtime/providers/provider-memory-profiler/src runtime/providers/provider-memory-profiler/src
COPY runtime/providers/provider-json-writer/src runtime/providers/provider-json-writer/src
COPY runtime/providers/provider-streaming-endpoint/src runtime/providers/provider-streaming-endpoint/src

RUN mvn -B clean install -DskipTests

# ----- Stage 2: Build Go CLI + Catalog -----
FROM golang:1.24 AS go-builder

# Build CLI
WORKDIR /build/cli
COPY cli/go.mod cli/go.sum ./
RUN go mod download
COPY cli/ .
RUN go vet ./... && go test ./... && go build -trimpath -buildvcs=false -o /quarkctl .

# Build Catalog
WORKDIR /build/catalog
COPY quark-catalog/go.mod quark-catalog/go.sum ./
RUN go mod download
COPY quark-catalog/ .
RUN go vet ./... && go test ./... && go build -trimpath -buildvcs=false -o /quark-catalog ./cmd/quark-catalog

# ----- Stage 3: Runtime image (JVM mode) -----
FROM eclipse-temurin:21-jre AS runtime-jvm

# Copy control plane (server) jar + lib dir
COPY --from=java-builder /build/server/quark-server/target/quark-server-0.1.0-SNAPSHOT-runner.jar /app/quark-server.jar
COPY --from=java-builder /build/server/quark-server/target/lib /app/lib/server
# Copy data plane (runtime) jar + lib dir
COPY --from=java-builder /build/runtime/quark-runtime/target/quark-runtime-runner-runner.jar /app/quark-runtime.jar
COPY --from=java-builder /build/runtime/quark-runtime/target/lib /app/lib/runtime
# Copy Go binaries
COPY --from=go-builder /quarkctl /app/quarkctl
COPY --from=go-builder /quark-catalog /app/quark-catalog

# Smoke tests
RUN /app/quarkctl --help | head -1
RUN /app/quark-catalog -h 2>&1 | head -1 || true
RUN java -version 2>&1 | head -1

WORKDIR /app
EXPOSE 8080 8081 4222

# Default: run the JVM server. For full platform bring-up, also start
# nats-server, the Catalog, and (if needed) the data-plane jar.
CMD ["java", "-jar", "quark-server.jar"]

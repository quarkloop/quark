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
COPY quark-adapter-state/pom.xml quark-adapter-state/
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
COPY quark-adapter-state/src quark-adapter-state/src
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
FROM golang:1.26 AS go-builder

WORKDIR /build
COPY cli/go.mod cli/go.sum ./
RUN go mod download

COPY cli/ .
RUN go vet ./... && go test ./... && go build -trimpath -o /quarkctl .

# ----- Stage 3: Verify image (small, just the artifacts) -----
FROM eclipse-temurin:21-jre AS verify

COPY --from=java-builder /build/quark-server/target/quarkus-app /app/quarkus-app
COPY --from=go-builder /quarkctl /app/quarkctl
COPY example/simple-streaming/system.quark.yaml /app/example/system.quark.yaml

# Smoke test: both artifacts exist and are runnable
RUN /app/quarkctl --help | head -1
RUN java -version 2>&1 | head -1

WORKDIR /app
EXPOSE 8080 8081

# Default: run the Quarkus server
CMD ["java", "-jar", "quarkus-app/quarkus-run.jar"]

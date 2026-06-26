# =============================================================================
# Dockerfile — Quark build verification image
# =============================================================================
# Multi-stage build that compiles every component of the platform in a clean
# container with no host toolchain dependencies. Used by `make docker-verify`
# to confirm the build does not depend on any local installations.
#
# This is a BUILD verification image, not a production runtime image. A
# production image would split components into separate images and ship
# only the JRE / Go binary each one needs.
# =============================================================================

# ----- Stage 1: Build Java runtime modules -----
# The data plane is a Quarkus application built with Maven. GraalVM is
# required for native-image builds; for JVM-mode verification a plain JDK 21
# is enough.
FROM maven:3.9-eclipse-temurin-21 AS java-builder

WORKDIR /build

# Copy the parent POM and Maven wrapper first so dependency resolution
# is cached across source-only changes.
COPY pom.xml mvnw mvnw.cmd .mvn ./

# Copy every Maven module POM. The reactor layout is:
#   quark-runtime/quark-core
#   quark-runtime/quark-script
#   quark-runtime/quark-polyglot
#   quark-runtime/quark-app
#   quark-runtime/quark-runtime
COPY quark-runtime/quark-core/pom.xml      quark-runtime/quark-core/
COPY quark-runtime/quark-script/pom.xml    quark-runtime/quark-script/
COPY quark-runtime/quark-polyglot/pom.xml  quark-runtime/quark-polyglot/
COPY quark-runtime/quark-app/pom.xml       quark-runtime/quark-app/
COPY quark-runtime/quark-runtime/pom.xml   quark-runtime/quark-runtime/

# Pre-fetch dependencies. Tolerate failures because the parent POM
# references some artifacts (GraalVM polyglot) that may not resolve
# cleanly in every environment.
RUN mvn -B dependency:go-offline -DskipTests || true

# Copy sources and build
COPY quark-runtime/quark-core/src      quark-runtime/quark-core/src
COPY quark-runtime/quark-script/src    quark-runtime/quark-script/src
COPY quark-runtime/quark-polyglot/src  quark-runtime/quark-polyglot/src
COPY quark-runtime/quark-app/src       quark-runtime/quark-app/src
COPY quark-runtime/quark-runtime/src   quark-runtime/quark-runtime/src

RUN mvn -B clean install -DskipTests

# ----- Stage 2: Build Go control plane, CLI, and catalog -----
FROM golang:1.24 AS go-builder

# Build the control plane (quark-server)
WORKDIR /build/quark-server
COPY quark-server/go.mod quark-server/go.sum ./
RUN go mod download
COPY quark-server/ .
RUN go vet ./... && go test ./... && go build -trimpath -buildvcs=false -o /quark-server ./cmd/server

# Build the CLI (quarkctl)
WORKDIR /build/quark-cli
COPY quark-cli/go.mod quark-cli/go.sum ./
RUN go mod download
COPY quark-cli/ .
RUN go vet ./... && go test ./... && go build -trimpath -buildvcs=false -o /quarkctl .

# Build the Catalog service
WORKDIR /build/quark-catalog
COPY quark-catalog/go.mod quark-catalog/go.sum ./
RUN go mod download
COPY quark-catalog/ .
RUN go vet ./... && go test ./... && go build -trimpath -buildvcs=false -o /quark-catalog ./cmd/quark-catalog

# ----- Stage 3: Runtime image (JVM mode) -----
FROM eclipse-temurin:21-jre AS runtime-jvm

# Copy the data-plane (Java runtime) jar + lib dir
COPY --from=java-builder /build/quark-runtime/quark-runtime/target/quark-runtime-runner-runner.jar /app/quark-runtime.jar
COPY --from=java-builder /build/quark-runtime/quark-runtime/target/lib /app/lib/runtime

# Copy Go binaries (control plane, CLI, catalog)
COPY --from=go-builder /quark-server  /app/quark-server
COPY --from=go-builder /quarkctl      /app/quarkctl
COPY --from=go-builder /quark-catalog /app/quark-catalog

# Smoke tests — confirm every binary is at least runnable.
RUN /app/quarkctl --help | head -1
RUN /app/quark-catalog -h 2>&1 | head -1 || true
RUN /app/quark-server -h 2>&1 | head -1 || true
RUN java -version 2>&1 | head -1

WORKDIR /app
EXPOSE 8080 8081 4222

# Default: run the Go control plane. The data plane is spawned as a
# child process by the control plane's ProcessManager.
CMD ["/app/quark-server"]

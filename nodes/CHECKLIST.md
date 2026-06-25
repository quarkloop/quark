# Node Implementation Checklist

Detailed checklist for creating, implementing, building, and pushing a
polyglot Quark node. Followed exactly — no shortcuts.

---

## Phase 1: Design

- [ ] **1. Determine the URI**

  Format: `quark/<domain>/<subdomain>/<node>:<version>`

  The domain must be one of the 18 agreed domains:
  `time`, `net`, `io`, `db`, `data`, `codec`, `text`, `ai`, `notify`,
  `log`, `search`, `stream`, `route`, `compute`, `cloud`, `system`,
  `console`, `crypto`.

  Verify the URI doesn't already exist in the
  [Node Catalog](../docs/content/docs/node-catalog.mdx).

- [ ] **2. Determine the language**

  - **Java** — for system-level, high-performance, or vendor-SDK-dependent
    nodes (JMX, NIO, HttpServer, ScheduledExecutorService, JDBC drivers).
  - **TypeScript** — for logic-heavy, API-calling, or rapid-iteration nodes
    (JSON parsing, field mapping, conditional routing, HTTP fetch).

- [ ] **3. Define the config schema**

  List every config field with:
  - Name (kebab-case)
  - Type (`string`, `integer`, `number`, `boolean`, `array`, `object`)
  - Required (`true` / `false`)
  - Default value (if not required)
  - Description
  - Secret flag (`true` if the value should be masked in logs/API)

- [ ] **4. Define the events**

  List every event this node publishes:
  - Event name
  - Payload shape (what fields, what types)

---

## Phase 2: Scaffold

- [ ] **5. Create the directory**

  ```
  nodes/quark/<domain>/<subdomain>/<node>/<version>/
  ```

  Example: `nodes/quark/io/file/watch/v1/`

- [ ] **6. Create `manifest.json`**

  Required fields:
  - `uri` — the full URI string
  - `domain` — must match the directory
  - `subdomain` — must match the directory
  - `node` — must match the directory
  - `version` — semver, must match the directory
  - `language` — `"java"` or `"typescript"`
  - `description` — one-line description

  Optional fields:
  - `config` — the config schema from step 3
  - `events` — the event list from step 4
  - `dependencies` — per-language dependency list
  - `supportedRuntimes` — `["jvm", "native"]`

- [ ] **7. Create `src/` directory**

  All source code goes here. No exceptions.

  For Java:
  ```
  src/
  ├── node.java         ← the implementation
  ├── config.java       ← typed config record (optional)
  └── node.test.java    ← unit tests (optional)
  ```

  For TypeScript:
  ```
  src/
  ├── node.ts           ← the implementation
  └── node.test.ts      ← unit tests (optional)
  ```

  The Java package is derived from the URI:
  `quark/io/file/watch:v1` → `package quark.io.file.watch;`

- [ ] **8. Create `build.toml`**

  For Java:
  ```toml
  [package]
  format = "zip"
  contentType = "shared-library"

  [build.jvm]
  output = "target/<node>-v<version>.jar"

  [build.native]
  output = "target/<node>-v<version>.so"
  flags = ["--shared", "--no-fallback"]
  ```

  For TypeScript:
  ```toml
  [package]
  format = "zip"
  contentType = "typescript"

  [build.typescript]
  output = "src/"
  ```

- [ ] **9. Create `README.md`**

  Include:
  - Node URI and description
  - Config table (field, type, required, default, description)
  - Events table (event, payload, description)
  - Usage example (a `.quark.ts` snippet)

---

## Phase 3: Implement

- [ ] **10. Implement the node**

  Write the source file in `src/`. The node implements whatever methods
  it needs based on what it does. The engine detects which methods are
  present and calls them accordingly.

  Java `node.java`:
  ```java
  package quark.io.file.watch;

  import com.quarkloop.quark.core.domain.config.NodeConfig;
  import com.quarkloop.quark.core.domain.spi.NodeProvider;
  import com.quarkloop.quark.core.domain.spi.QuarkPublisher;

  public class FileWatchNode implements NodeProvider {

      private FileWatchConfig config;

      public void init(NodeConfig cfg) {
          this.config = FileWatchConfig.from(cfg);
      }

      public void start(QuarkPublisher publisher) {
          // ... watch logic, call publisher.publish("changed", payload)
      }

      public void close() {
          // ... cleanup
      }
  }
  ```

  TypeScript `node.ts`:
  ```typescript
  export default {
      onMessage(message, publisher) {
          const parsed = JSON.parse(message.payload.data);
          publisher.publish("parsed", { data: parsed });
      }
  };
  ```

- [ ] **11. Implement typed config (Java only)**

  Create `src/config.java` — a record with a `from(NodeConfig)` factory:

  ```java
  package quark.io.file.watch;

  import com.quarkloop.quark.core.domain.config.NodeConfig;

  public record FileWatchConfig(
      String path,
      String pattern,
      boolean recursive
  ) {
      public static FileWatchConfig from(NodeConfig config) {
          return new FileWatchConfig(
              config.getString("path", "."),
              config.getString("pattern", "*"),
              config.getBoolean("recursive", false)
          );
      }
  }
  ```

- [ ] **12. Handle errors properly**

  - **Rethrow exceptions** — do not catch and swallow. The engine needs
    to see failures for metrics and NATS nak semantics.
  - **Validate config in `init()`** — throw if required fields are missing.
  - **Log meaningful errors** — include the node URI and config context.

- [ ] **13. Clean up resources in `close()`**

  - Close file handles, sockets, schedulers, database connections.
  - Close GraalJS contexts (for TypeScript nodes with `onStop`).
  - No resource leaks. The engine calls `close()` on undeploy.

---

## Phase 4: Test

- [ ] **14. Write unit tests**

  Create `src/node.test.java` or `src/node.test.ts`.

  - Mock the `QuarkPublisher` / `NodePublisher`.
  - Send stub messages.
  - Assert the correct events are published with the correct payloads.
  - Assert config validation works (missing required fields throw).

- [ ] **15. Test config validation**

  - Missing required fields → should throw.
  - Wrong types → should throw or coerce.
  - Default values → should be applied when field is absent.
  - Secret fields → should be masked in any error messages.

- [ ] **16. Test edge cases**

  - Empty input payload.
  - Null payload fields.
  - Large payload (performance).
  - Concurrent messages (thread safety, if applicable).

---

## Phase 5: Build

- [ ] **17. Build JVM artifact (Java only)**

  ```bash
  quarkctl node build quark/io/file/watch:v1 --runtime jvm
  ```

  Compiles `src/*.java` → `target/watch-v1.0.0.jar`

- [ ] **18. Build native artifact (Java only)**

  ```bash
  quarkctl node build quark/io/file/watch:v1 --runtime native
  ```

  Compiles `src/*.java` via GraalVM `native-image --shared` →
  `target/watch-v1.0.0.so`

- [ ] **19. Verify artifacts exist and are non-empty**

  ```bash
  ls -lh nodes/quark/io/file/watch/v1/target/
  ```

  For TypeScript nodes, no build step — `src/` is used as-is.

---

## Phase 6: Package

- [ ] **20. Package into zip blob**

  ```bash
  quarkctl node build quark/io/file/watch:v1
  ```

  This zips `manifest.json` + the build output (`.jar`, `.so`, or `.ts`)
  into a single `.zip` file.

- [ ] **21. Verify the zip contents**

  ```bash
  unzip -l nodes/quark/io/file/watch/v1/target/watch-v1.0.0.zip
  ```

  Should contain:
  - `manifest.json`
  - `watch-v1.0.0.jar` (or `.so` or `node.ts`)

- [ ] **22. Verify `contentType` matches the language**

  - Java → `contentType = "shared-library"`
  - TypeScript → `contentType = "typescript"`

---

## Phase 7: Push to Catalog

- [ ] **23. Start NATS + Catalog service**

  ```bash
  nats-server --addr 127.0.0.1 --port 4222 &
  ./quark-catalog/quark-catalog -nats nats://localhost:4222 -state ./quark-state &
  ```

- [ ] **24. Push to Catalog**

  ```bash
  quarkctl node push quark/io/file/watch:v1
  ```

  Sends `{ uri, manifest, content (zip bytes), contentType }` to the
  Catalog via `registry.node.push` NATS request.

- [ ] **25. Verify push succeeded**

  ```bash
  quarkctl node info quark/io/file/watch:v1
  ```

  Should return the manifest + metadata (checksum, created date, etc).

- [ ] **26. Verify checksum**

  The Catalog computes a SHA-256 of the content blob. Compare it against
  a local hash of the zip file to verify integrity.

---

## Phase 8: Integration Test

- [ ] **27. Write a test `.quark.ts` system**

  ```typescript
  export default({
      name: "test",
      namespace: "test",
      nodes: {
          watcher: {
              uses: "quark/io/file/watch:v1",
              path: "/tmp",
              events: ["changed", "created"],
          },
          logger: {
              uses: "quark/log/console/stdout:v1",
              listens: ["watcher.changed", "watcher.created"],
          }
      }
  });
  ```

- [ ] **28. Deploy the test system**

  ```bash
  quarkctl apply -f test.quark.ts -n test
  ```

  The data plane should:
  1. Pull `quark/io/file/watch:v1` from the Catalog.
  2. Pull `quark/log/console/stdout:v1` from the Catalog.
  3. Unzip both, load them (`.jar`/`.so`/`.ts`).
  4. Start the watcher, subscribe the logger.
  5. Emit events when files change in `/tmp`.

- [ ] **29. Verify events flow**

  ```bash
  echo "hello" > /tmp/test.txt
  ```

  Check the data plane log — the stdout node should print the change event.

- [ ] **30. Undeploy and verify clean shutdown**

  ```bash
  quarkctl delete system test -n test
  ```

  Verify:
  - The watcher's `close()` was called (file handle released).
  - The stdout node's `close()` was called (if it has one).
  - No leaked threads, file handles, or GraalJS contexts.

---

## Phase 9: Document

- [ ] **31. Update node `README.md`**

  Add any quirks, performance notes, limitations discovered during
  implementation or testing.

- [ ] **32. Add to Node Catalog doc**

  If this is a standard library node, update
  `docs/content/docs/node-catalog.mdx` to mark it as implemented.

- [ ] **33. Add usage example**

  Add a realistic `.quark.ts` snippet showing the node in a pipeline with
  other nodes.

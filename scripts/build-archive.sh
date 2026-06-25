#!/usr/bin/env bash
# =============================================================================
# build-archive.sh — build a versioned quark-platform-v<N>.zip archive
# =============================================================================
# Produces /home/z/my-project/download/quark-platform-v<N>.zip containing
# the full project tree (including .git/ — per the bootstrap convention)
# ready for delivery.
#
# Usage:
#   /home/z/my-project/scripts/build-archive.sh [version]
#
# If version is not given, the next number is computed by scanning
# /home/z/my-project/download/ for existing quark-platform-v*.zip files.
#
# The archive is built by zipping the work tree (including .git/) after
# excluding build artifacts via a zip exclude pattern. The top-level
# directory inside the archive is quark-platform-v<N>/.
# =============================================================================
set -euo pipefail

DOWNLOAD_DIR=/home/z/my-project/download
WORK_DIR=/home/z/my-project/work

# Determine the version number
if [ $# -ge 1 ]; then
    VERSION=$1
else
    LAST_VERSION=0
    if [ -d "$DOWNLOAD_DIR" ]; then
        for f in "$DOWNLOAD_DIR"/quark-platform-v*.zip; do
            [ -e "$f" ] || continue
            v=$(basename "$f" | sed -E 's/quark-platform-v([0-9]+)\.zip/\1/')
            if [ "$v" -gt "$LAST_VERSION" ] 2>/dev/null; then
                LAST_VERSION=$v
            fi
        done
    fi
    VERSION=$((LAST_VERSION + 1))
fi

ARCHIVE_NAME="quark-platform-v${VERSION}"
ARCHIVE_PATH="${DOWNLOAD_DIR}/${ARCHIVE_NAME}.zip"

echo "▶ Building archive: $ARCHIVE_PATH"

mkdir -p "$DOWNLOAD_DIR"
rm -f "$ARCHIVE_PATH"

# Zip the work tree, including .git/ but excluding build artifacts.
# We run from the parent dir so the archive has a single top-level dir.
cd "$WORK_DIR/.."

BASE="$(basename "$WORK_DIR")"

# Use zip -r with --exclude to skip build artifacts:
#   - */target/           (Maven build output)
#   - */quark-state/       (platform state — runtime data)
#   - */dist/              (Go cross-build artifacts)
#   - */node_modules/      (any npm artifacts)
#   - *.jar, *.class       (Java compiled artifacts)
#   - Go binaries: server/quark-server, cli/quarkctl, quark-catalog/quark-catalog
#   - */dataplane-logs/    (data-plane runtime logs)
#   - */json/system-monitor.jsonl  (example output)
#   - **/*.log             (log files)
#   - */.DS_Store          (macOS metadata)
zip -qr "$ARCHIVE_PATH" "$BASE" \
    -x "$BASE/target/*" \
    -x "$BASE/*/target/*" \
    -x "$BASE/*/*/target/*" \
    -x "$BASE/*/*/*/target/*" \
    -x "$BASE/*/*/*/*/target/*" \
    -x "$BASE/quark-state/*" \
    -x "$BASE/dist/*" \
    -x "$BASE/node_modules/*" \
    -x "$BASE/server/quark-server" \
    -x "$BASE/cli/quarkctl" \
    -x "$BASE/quark-catalog/quark-catalog" \
    -x "$BASE/quark-state/dataplane-logs/*" \
    -x "$BASE/example/simple-streaming/json/*" \
    -x "$BASE/.git/logs/*" \
    -x "$BASE/nodes/quark/*/*/v1/target/*.jar" \
    -x "**/*.class" \
    -x "**/*.log" \
    -x "**/.DS_Store"
# Note: do NOT exclude **/*.jar blanketly — that would also exclude
# .mvn/wrapper/maven-wrapper.jar which is required for ./mvnw to work.
# Instead we exclude node build outputs specifically.

# Rename the top-level dir inside the archive to match the version
# (zip doesn't have a flag for this, so we use a temp dir + mv)
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT
cd "$TMPDIR"
unzip -q "$ARCHIVE_PATH"
mv "$(basename "$WORK_DIR")" "$ARCHIVE_NAME"
zip -qr "$ARCHIVE_PATH.new" "$ARCHIVE_NAME"
mv "$ARCHIVE_PATH.new" "$ARCHIVE_PATH"

echo ""
echo "✓ Archive built: $ARCHIVE_PATH"
ls -lh "$ARCHIVE_PATH"
echo ""
echo "Top-level entries:"
unzip -l "$ARCHIVE_PATH" | head -8
echo "..."
echo ""
echo "Total files in archive: $(unzip -l "$ARCHIVE_PATH" | tail -1 | awk '{print $2}')"
echo ""
echo "Includes .git/:"
unzip -l "$ARCHIVE_PATH" | grep -E "\.git/(HEAD|config)$" | head -2

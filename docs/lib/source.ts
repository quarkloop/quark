import "server-only";
import { loader } from "fumadocs-core/source";
import { docs } from "@/.source";

// fumadocs-mdx v11 returns source as { files: () => File[] } (a function),
// but fumadocs-core v15 expects { files: File[] } (an array).
// Unwrap by calling files() once at module load.
const rawSource = docs.toFumadocsSource();
const sourceFiles = typeof rawSource.files === "function"
  ? (rawSource.files as () => unknown[])()
  : rawSource.files;

export const source = loader({
  baseUrl: "/docs",
  source: { files: sourceFiles as never } as never,
});

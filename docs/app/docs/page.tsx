import Link from "next/link";
import { source } from "@/lib/source";
import {
  Cpu,
  Database,
  GitBranch,
  Layers,
  MessageSquare,
  Terminal,
} from "lucide-react";
import { DocCard } from "@/components/doc-card";

const categoryIcons: Record<string, typeof Cpu> = {
  abstraction: Layers,
  cli: Terminal,
  declaration: GitBranch,
  design: Layers,
  "environment-bootstrap": Cpu,
  node: Database,
  "user-story": MessageSquare,
};

export default function DocsIndexPage() {
  const docs = source.getPages();
  return (
    <main className="mx-auto max-w-6xl px-6 py-16">
      <header className="mb-12">
        <div className="inline-flex items-center gap-2 rounded-full border border-border/60 bg-card/40 px-3 py-1 text-xs font-medium text-sand-400">
          <span className="h-1.5 w-1.5 rounded-full bg-ember-500" />
          Documentation
        </div>
        <h1 className="mt-4 font-display text-4xl font-semibold tracking-tight">
          Browse the docs
        </h1>
        <p className="mt-2 text-sand-400 max-w-2xl">
          Everything from the bootstrap environment through the node
          specification and the user story. Pick a topic.
        </p>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {docs.map((doc) => {
          const slug = doc.slugs[0] ?? "";
          const Icon = categoryIcons[slug] ?? Cpu;
          return (
            <DocCard
              key={doc.url}
              href={doc.url}
              icon={Icon}
              title={doc.data.title ?? "Untitled"}
              description={doc.data.description ?? ""}
            />
          );
        })}
      </div>
    </main>
  );
}

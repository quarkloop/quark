import Link from "next/link";
import {
  ArrowRight,
  Boxes,
  Cpu,
  Database,
  GitBranch,
  Layers,
  MessageSquare,
  Terminal,
  Zap,
} from "lucide-react";
import { source } from "@/lib/source";

const services = [
  {
    icon: Cpu,
    name: "Control Plane",
    tagline: "Java / Native · 76 MB",
    description:
      "REST API, deploy orchestration, spawns data-plane processes. No GraalJS — uses a regex-based SimpleSystemParser.",
  },
  {
    icon: Database,
    name: "Catalog",
    tagline: "Go + SQLite · 15 MB",
    description:
      "Standalone metadata store. Pure Go (modernc.org/sqlite), no CGO, no JNI. Stores systems, nodes, events, sources.",
  },
  {
    icon: Zap,
    name: "Data Plane",
    tagline: "Java / Native · 194 MB",
    description:
      "Executes nodes via GraalJS + providers. Includes Truffle via --macro:truffle-svm. Spawned on demand.",
  },
  {
    icon: MessageSquare,
    name: "NATS Broker",
    tagline: "External · Core mode",
    description:
      "Message bus for all inter-service communication. Subjects encode namespace for multi-tenant isolation.",
  },
];

const features = [
  {
    icon: Layers,
    title: "Everything is a Node",
    description:
      "Sources, functions, stores, endpoints, policies — every entity is a Node with a Docker-style URI.",
  },
  {
    icon: GitBranch,
    title: "Multi-tenant by construction",
    description:
      "NATS subjects encode namespace. Two tenants can deploy same-named systems with zero data leakage.",
  },
  {
    icon: Boxes,
    title: "Strict tier separation",
    description:
      "core/ (shared) ← server/ (control plane) and core/ ← runtime/ (data plane). Never server/ ↔ runtime/.",
  },
  {
    icon: Terminal,
    title: "TypeScript is the language",
    description:
      "Users write .quark.ts files. No YAML, no custom DSL, no arrow notation. The file IS the program.",
  },
];

export default function HomePage() {
  const docs = source.getPages();
  return (
    <main className="relative min-h-screen overflow-hidden">
      {/* Hero backdrop */}
      <div className="pointer-events-none absolute inset-0 -z-10">
        <div className="absolute inset-0 bg-grid opacity-50" />
        <div className="absolute inset-x-0 top-0 h-[600px] bg-radial-fade" />
        <div className="absolute left-1/2 top-0 -z-10 h-[480px] w-[840px] -translate-x-1/2 rounded-full bg-accent-500/10 blur-[120px]" />
      </div>

      {/* Nav */}
      <nav className="sticky top-0 z-50 border-b border-border/60 glass-panel">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-6">
          <Link href="/" className="flex items-center gap-2.5 group">
            <div className="relative h-8 w-8 rounded-lg bg-gradient-to-br from-accent-400 to-accent-600 shadow-glow-sm flex items-center justify-center">
              <span className="font-display text-sm font-bold text-ink-950">Q</span>
            </div>
            <span className="font-display text-base font-semibold tracking-tight">
              Quark Platform
            </span>
          </Link>
          <div className="flex items-center gap-1">
            <Link
              href="/docs"
              className="px-3 py-1.5 text-sm text-ink-500 hover:text-ink-900 dark:hover:text-ink-50 transition-colors"
            >
              Docs
            </Link>
            <Link
              href="/docs/environment-bootstrap"
              className="px-3 py-1.5 text-sm text-ink-500 hover:text-ink-900 dark:hover:text-ink-50 transition-colors"
            >
              Bootstrap
            </Link>
            <Link
              href="/docs/declaration"
              className="ml-2 inline-flex items-center gap-1 rounded-lg bg-accent-500 px-3 py-1.5 text-sm font-medium text-ink-950 hover:bg-accent-400 transition-colors shadow-glow-sm"
            >
              Get started
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="mx-auto max-w-7xl px-6 pt-24 pb-20">
        <div className="mx-auto max-w-3xl text-center">
          <div className="inline-flex items-center gap-2 rounded-full border border-border/60 bg-card/40 px-3 py-1 text-xs font-medium text-ink-400 backdrop-blur-sm animate-fade-in">
            <span className="relative flex h-1.5 w-1.5">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-accent-400 opacity-75" />
              <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-accent-500" />
            </span>
            v0.1.0 · Three-service architecture
          </div>

          <h1 className="mt-8 font-display text-5xl sm:text-6xl md:text-7xl font-semibold tracking-tight text-balance animate-slide-up">
            A universal runtime
            <br />
            for{" "}
            <span className="accent-text">programmable nodes</span>
          </h1>

          <p className="mt-6 text-lg sm:text-xl text-ink-400 leading-relaxed text-pretty animate-slide-up [animation-delay:60ms]">
            Declare nodes in TypeScript. Parse them on the control plane.
            Execute them on the data plane with GraalJS. Persist them through
            the Catalog. Wire it all together with NATS.
          </p>

          <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-3 animate-slide-up [animation-delay:120ms]">
            <Link
              href="/docs/declaration"
              className="inline-flex w-full sm:w-auto items-center justify-center gap-2 rounded-xl bg-accent-500 px-6 py-3 text-sm font-semibold text-ink-950 transition-all hover:bg-accent-400 hover:shadow-glow focus-ring"
            >
              Read the spec
              <ArrowRight className="h-4 w-4" />
            </Link>
            <Link
              href="/docs/environment-bootstrap"
              className="inline-flex w-full sm:w-auto items-center justify-center gap-2 rounded-xl border border-border bg-card/60 px-6 py-3 text-sm font-semibold text-ink-200 hover:bg-card transition-all focus-ring"
            >
              <Terminal className="h-4 w-4" />
              Bootstrap environment
            </Link>
          </div>
        </div>
      </section>

      {/* Service grid */}
      <section className="mx-auto max-w-7xl px-6 pb-24">
        <div className="mb-8 flex items-end justify-between">
          <div>
            <h2 className="font-display text-2xl font-semibold tracking-tight">
              Three services, one platform
            </h2>
            <p className="mt-1 text-sm text-ink-400">
              Each service has a single responsibility and communicates via NATS.
            </p>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {services.map((s, i) => (
            <div
              key={s.name}
              className="group relative rounded-2xl glass-panel p-6 transition-all hover:border-accent-500/40 hover:shadow-glow-sm animate-slide-up"
              style={{ animationDelay: `${i * 80}ms` }}
            >
              <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-xl bg-accent-500/10 text-accent-400 group-hover:bg-accent-500/20 transition-colors">
                <s.icon className="h-5 w-5" />
              </div>
              <div className="flex items-baseline justify-between">
                <h3 className="font-display text-base font-semibold">{s.name}</h3>
                <span className="text-[10px] uppercase tracking-wider font-medium text-ink-500">
                  {s.tagline}
                </span>
              </div>
              <p className="mt-2 text-sm text-ink-400 leading-relaxed">
                {s.description}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* Features */}
      <section className="mx-auto max-w-7xl px-6 pb-24">
        <div className="rounded-3xl border border-border/60 glass-panel-strong p-8 sm:p-12">
          <h2 className="font-display text-2xl font-semibold tracking-tight text-center">
            Built on non-negotiable principles
          </h2>
          <div className="mt-10 grid grid-cols-1 md:grid-cols-2 gap-x-12 gap-y-8">
            {features.map((f) => (
              <div key={f.title} className="flex gap-4">
                <div className="flex-shrink-0 mt-0.5 inline-flex h-9 w-9 items-center justify-center rounded-lg bg-accent-500/10 text-accent-400">
                  <f.icon className="h-4.5 w-4.5" />
                </div>
                <div>
                  <h3 className="font-display text-base font-semibold">
                    {f.title}
                  </h3>
                  <p className="mt-1 text-sm text-ink-400 leading-relaxed">
                    {f.description}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Docs directory */}
      <section className="mx-auto max-w-7xl px-6 pb-32">
        <div className="mb-8 flex items-end justify-between">
          <div>
            <h2 className="font-display text-2xl font-semibold tracking-tight">
              Documentation
            </h2>
            <p className="mt-1 text-sm text-ink-400">
              {docs.length} documents · browse by topic
            </p>
          </div>
          <Link
            href="/docs"
            className="text-sm text-accent-400 hover:text-accent-300 inline-flex items-center gap-1"
          >
            View all
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {docs.slice(0, 9).map((doc) => {
            const slug = doc.slugs[0] ?? "";
            const Icon = iconForDoc(slug);
            return (
              <Link
                key={doc.url}
                href={doc.url}
                className="group rounded-xl border border-border/60 bg-card/40 p-5 transition-all hover:border-accent-500/40 hover:bg-card/80 hover:shadow-premium"
              >
                <div className="flex items-start gap-3">
                  <div className="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-accent-500/10 text-accent-400">
                    <Icon className="h-4 w-4" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <h3 className="font-display text-sm font-semibold truncate group-hover:text-accent-300 transition-colors">
                      {doc.data.title}
                    </h3>
                    <p className="mt-1 text-xs text-ink-400 line-clamp-2">
                      {doc.data.description}
                    </p>
                  </div>
                </div>
              </Link>
            );
          })}
        </div>
      </section>

      <footer className="border-t border-border/60 py-8">
        <div className="mx-auto max-w-7xl px-6 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-ink-500">
          <div className="flex items-center gap-2">
            <div className="h-4 w-4 rounded bg-gradient-to-br from-accent-400 to-accent-600" />
            <span>Quark Platform · v0.1.0-SNAPSHOT</span>
          </div>
          <div className="flex items-center gap-4">
            <span>Built with Fumadocs</span>
            <span>·</span>
            <span>Java 21 + Quarkus + Go + NATS + GraalJS</span>
          </div>
        </div>
      </footer>
    </main>
  );
}

function iconForDoc(slug: string) {
  const map: Record<string, typeof Boxes> = {
    abstraction: Layers,
    cli: Terminal,
    declaration: GitBranch,
    design: Boxes,
    "environment-bootstrap": Cpu,
    node: Boxes,
    "user-story": MessageSquare,
  };
  return map[slug] ?? Boxes;
}

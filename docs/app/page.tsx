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
import { Nav } from "@/components/nav";
import { Footer } from "@/components/footer";
import { ServiceCard } from "@/components/service-card";
import { FeatureItem } from "@/components/feature-item";
import { DocCard } from "@/components/doc-card";

const services = [
  {
    icon: Cpu,
    name: "Control Plane",
    tagline: "Java / Native · 76 MB",
    description:
      "REST API, deploy orchestration, spawns data-plane processes. No GraalJS — uses a comment-aware SimpleSystemParser.",
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

const docIcons: Record<string, typeof Boxes> = {
  abstraction: Layers,
  cli: Terminal,
  declaration: GitBranch,
  design: Boxes,
  "environment-bootstrap": Cpu,
  node: Boxes,
  "user-story": MessageSquare,
};

export default function HomePage() {
  const docs = source.getPages();
  return (
    <main className="relative min-h-screen">
      <Nav />

      {/* Hero */}
      <section className="mx-auto max-w-7xl px-6 pt-24 pb-20">
        <div className="mx-auto max-w-3xl text-center">
          <div className="inline-flex items-center gap-2 rounded-full border border-border/60 bg-card/40 px-3 py-1 text-xs font-medium text-sand-400 backdrop-blur-sm animate-fade-in">
            <span className="relative flex h-1.5 w-1.5">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-ember-500 opacity-75" />
              <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-ember-500" />
            </span>
            v0.1.0 · Three-service architecture
          </div>

          <h1 className="mt-8 font-display text-5xl sm:text-6xl md:text-7xl font-semibold tracking-tight text-balance animate-slide-up">
            A universal runtime
            <br />
            for{" "}
            <span className="text-ember-500">programmable nodes</span>
          </h1>

          <p className="mt-6 text-lg sm:text-xl text-sand-400 leading-relaxed text-pretty animate-slide-up [animation-delay:60ms]">
            Declare nodes in TypeScript. Parse them on the control plane.
            Execute them on the data plane with GraalJS. Persist them through
            the Catalog. Wire it all together with NATS.
          </p>

          <div className="mt-10 flex flex-col sm:flex-row items-center justify-center gap-3 animate-slide-up [animation-delay:120ms]">
            <Link
              href="/docs/declaration"
              className="inline-flex w-full sm:w-auto items-center justify-center gap-2 rounded-xl bg-ember-500 px-6 py-3 text-sm font-semibold text-sand-950 transition-all hover:bg-ember-400 hover:shadow-glow focus-ring"
            >
              Read the spec
              <ArrowRight className="h-4 w-4" />
            </Link>
            <Link
              href="/docs/environment-bootstrap"
              className="inline-flex w-full sm:w-auto items-center justify-center gap-2 rounded-xl border border-border bg-card/60 px-6 py-3 text-sm font-semibold text-sand-200 hover:bg-card transition-all focus-ring"
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
            <p className="mt-1 text-sm text-sand-400">
              Each service has a single responsibility and communicates via NATS.
            </p>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {services.map((s, i) => (
            <ServiceCard
              key={s.name}
              icon={s.icon}
              name={s.name}
              tagline={s.tagline}
              description={s.description}
              delay={i * 80}
            />
          ))}
        </div>
      </section>

      {/* Features */}
      <section className="mx-auto max-w-7xl px-6 pb-24">
        <div className="rounded-3xl border border-border/60 card-warm p-8 sm:p-12">
          <h2 className="font-display text-2xl font-semibold tracking-tight text-center">
            Built on non-negotiable principles
          </h2>
          <div className="mt-10 grid grid-cols-1 md:grid-cols-2 gap-x-12 gap-y-8">
            {features.map((f) => (
              <FeatureItem
                key={f.title}
                icon={f.icon}
                title={f.title}
                description={f.description}
              />
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
            <p className="mt-1 text-sm text-sand-400">
              {docs.length} documents · browse by topic
            </p>
          </div>
          <Link
            href="/docs"
            className="text-sm text-ember-500 hover:text-ember-400 inline-flex items-center gap-1"
          >
            View all
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {docs.slice(0, 9).map((doc) => {
            const slug = doc.slugs[0] ?? "";
            const Icon = docIcons[slug] ?? Boxes;
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
      </section>

      <Footer />
    </main>
  );
}

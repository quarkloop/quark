import type { LucideIcon } from "lucide-react";

/**
 * A single service card in the home page service grid.
 * Solid color icon, frosted panel, hover accent border.
 */
export function ServiceCard({
  icon: Icon,
  name,
  tagline,
  description,
  delay = 0,
}: {
  icon: LucideIcon;
  name: string;
  tagline: string;
  description: string;
  delay?: number;
}) {
  return (
    <div
      className="group relative rounded-2xl glass-panel p-6 transition-all hover:border-accent-500/40 hover:shadow-glow-sm animate-slide-up"
      style={{ animationDelay: `${delay}ms` }}
    >
      <div className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-xl bg-accent-500/10 text-accent-500 group-hover:bg-accent-500/20 transition-colors">
        <Icon className="h-5 w-5" />
      </div>
      <div className="flex items-baseline justify-between">
        <h3 className="font-display text-base font-semibold">{name}</h3>
        <span className="text-[10px] uppercase tracking-wider font-medium text-ink-500">
          {tagline}
        </span>
      </div>
      <p className="mt-2 text-sm text-ink-400 leading-relaxed">{description}</p>
    </div>
  );
}

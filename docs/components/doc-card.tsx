import Link from "next/link";
import { ArrowRight } from "lucide-react";
import type { LucideIcon } from "lucide-react";

/**
 * A single doc card in the home page docs directory grid.
 * Links to the doc page.
 */
export function DocCard({
  href,
  icon: Icon,
  title,
  description,
}: {
  href: string;
  icon: LucideIcon;
  title: string;
  description: string;
}) {
  return (
    <Link
      href={href}
      className="group rounded-xl border border-border/60 bg-card/40 p-5 transition-all hover:border-accent-500/40 hover:bg-card/80 hover:shadow-premium"
    >
      <div className="flex items-start gap-3">
        <div className="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-accent-500/10 text-accent-500">
          <Icon className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="font-display text-sm font-semibold truncate group-hover:text-accent-500 transition-colors">
            {title}
          </h3>
          <p className="mt-1 text-xs text-ink-400 line-clamp-2">{description}</p>
        </div>
      </div>
    </Link>
  );
}

/**
 * Export ArrowRight so consumers can build "view all" links.
 */
export { ArrowRight };

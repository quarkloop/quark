import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Logo } from "@/components/logo";

/**
 * Top navigation bar — sticky, frosted glass background.
 * Shown on the home page and other non-docs pages.
 */
export function Nav() {
  return (
    <nav className="sticky top-0 z-50 border-b border-border/60 glass-panel">
      <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-6">
        <Logo />
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
  );
}

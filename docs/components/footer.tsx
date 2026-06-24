import { Logo } from "@/components/logo";

/**
 * Footer — minimal, warm. Just the logo and version.
 * No tech-stack bragging text.
 */
export function Footer() {
  return (
    <footer className="border-t border-border/60 py-8">
      <div className="mx-auto max-w-7xl px-6 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs text-ink-500">
        <Logo size="sm" />
        <span>v0.1.0-SNAPSHOT</span>
      </div>
    </footer>
  );
}

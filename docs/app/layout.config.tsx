import type { BaseLayoutProps } from "fumadocs-ui/layouts/shared";
import { Cpu } from "lucide-react";

/**
 * Shared layout options used by every Fumadocs layout (docs, home, search).
 * Keeps nav consistent across routes.
 */
export const baseOptions: BaseLayoutProps = {
  nav: {
    title: (
      <div className="flex items-center gap-2.5">
        <div className="relative h-7 w-7 rounded-md bg-gradient-to-br from-accent-400 to-accent-600 shadow-glow-sm flex items-center justify-center">
          <span className="font-display text-xs font-bold text-ink-950">Q</span>
        </div>
        <span className="font-display text-sm font-semibold tracking-tight">
          Quark Platform
        </span>
      </div>
    ),
  },
  links: [
    {
      text: "Docs",
      url: "/docs",
      active: "nested-url",
    },
    {
      text: "Bootstrap",
      url: "/docs/environment-bootstrap",
      active: "nested-url",
    },
    {
      text: "Spec",
      url: "/docs/declaration",
      active: "nested-url",
    },
  ],
};

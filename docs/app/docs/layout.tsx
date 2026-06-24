import { DocsLayout } from "fumadocs-ui/layouts/docs";
import type { ReactNode } from "react";
import { baseOptions } from "@/app/layout.config";
import { source } from "@/lib/source";

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <DocsLayout
      tree={source.pageTree}
      {...baseOptions}
      sidebar={{
        defaultOpenLevel: 1,
        banner: (
          <div className="flex items-center gap-2 rounded-lg border border-accent-500/20 bg-accent-500/5 px-3 py-2 text-xs text-ink-300">
            <span className="h-1.5 w-1.5 rounded-full bg-accent-400" />
            <span>v0.1.0-SNAPSHOT</span>
          </div>
        ),
      }}
    >
      {children}
    </DocsLayout>
  );
}

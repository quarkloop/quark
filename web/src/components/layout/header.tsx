"use client";

import { useAgentContext } from "@/context/agent-context";
import { StatusIndicator } from "./status-indicator";
import { Zap } from "lucide-react";

export function Header() {
  const { activeAgent } = useAgentContext();

  return (
    <header className="flex h-14 shrink-0 items-center border-b border-border/60 bg-background px-5">
      <div className="flex items-center gap-2.5">
        <div className="flex size-7 items-center justify-center rounded-lg bg-foreground">
          <Zap className="size-3.5 text-background" strokeWidth={2.5} />
        </div>
        <span className="text-base font-semibold tracking-tight">Quark</span>
      </div>

      {activeAgent && (
        <>
          <div className="mx-4 h-4 w-px bg-border" />
          <div className="flex items-center gap-2 text-sm">
            <StatusIndicator status={activeAgent.status} />
            <span className="font-medium text-foreground/80">
              {activeAgent.name}
            </span>
            <span className="font-mono text-sm text-muted-foreground">
              :{activeAgent.port}
            </span>
          </div>
        </>
      )}
    </header>
  );
}

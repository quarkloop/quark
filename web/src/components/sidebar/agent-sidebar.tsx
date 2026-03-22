"use client";

import { useAgents } from "@/hooks/use-agents";
import { AgentCard } from "./agent-card";
import { AddAgentDialog } from "./add-agent-dialog";
import { SessionSidebar } from "./session-sidebar";
import { Button } from "@/components/ui/button";
import { RefreshCw, Radio } from "lucide-react";

export function AgentSidebar() {
  const { agents, activeAgent, isDiscovering, discover, addAgent, setActive } =
    useAgents();

  return (
    <aside className="flex h-full w-60 shrink-0 flex-col border-r border-border/60 bg-muted/30">
      <div className="flex items-center justify-between px-4 py-3">
        <div className="flex items-center gap-1.5">
          <Radio className="size-3 text-muted-foreground" />
          <span className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
            Agents
          </span>
        </div>
        <Button
          variant="ghost"
          size="icon-xs"
          onClick={discover}
          disabled={isDiscovering}
          aria-label="Discover agents"
          className="size-6 text-muted-foreground hover:text-foreground"
        >
          <RefreshCw
            className={`size-3 ${isDiscovering ? "animate-spin" : ""}`}
          />
        </Button>
      </div>

      <div className="flex flex-1 flex-col overflow-y-auto px-2">
        {agents.length === 0 && !isDiscovering && (
          <div className="flex flex-1 flex-col items-center justify-center gap-2 px-3 py-8">
            <div className="flex size-10 items-center justify-center rounded-xl bg-muted">
              <Radio className="size-4 text-muted-foreground" />
            </div>
            <p className="text-center text-sm text-muted-foreground">
              No agents found
            </p>
            <p className="text-center text-xs text-muted-foreground/60">
              Start an agent or click refresh
            </p>
          </div>
        )}
        <div className="space-y-0.5">
          {agents.map((agent) => (
            <AgentCard
              key={agent.id}
              agent={agent}
              active={activeAgent?.id === agent.id}
              onSelect={() => setActive(agent.id)}
            />
          ))}
        </div>
      </div>

      <SessionSidebar />

      <div className="border-t border-border/60 p-2">
        <AddAgentDialog onAdd={addAgent} />
      </div>
    </aside>
  );
}

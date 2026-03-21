import { cn } from "@/lib/utils";
import type { AgentStatus } from "@/lib/types";

const statusColors: Record<AgentStatus, string> = {
  online: "bg-emerald-500",
  offline: "bg-zinc-300",
  busy: "bg-amber-500",
  unknown: "bg-zinc-300",
};

export function StatusIndicator({
  status,
  className,
}: {
  status: AgentStatus;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-block size-2 rounded-full",
        statusColors[status],
        className,
      )}
      aria-label={status}
    />
  );
}

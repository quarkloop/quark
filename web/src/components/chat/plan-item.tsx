"use client";

import { ClipboardList, ArrowRight, Check, X, Loader2 } from "lucide-react";

interface PlanItemProps {
  eventType: string;
  data?: Record<string, string>;
  timestamp: string;
}

const stepIcons: Record<string, React.ReactNode> = {
  "step.dispatched": <ArrowRight className="size-3 text-blue-500" />,
  "step.completed": <Check className="size-3 text-emerald-500" />,
  "step.failed": <X className="size-3 text-destructive" />,
};

export function PlanItem({ eventType, data, timestamp }: PlanItemProps) {
  if (eventType === "plan.created" || eventType === "masterplan.created") {
    return (
      <div className="mx-4 my-1 flex items-center gap-2 rounded-md border border-border bg-muted/50 px-3 py-2 text-sm">
        <ClipboardList className="size-3 text-muted-foreground" />
        <span className="font-medium">
          {eventType === "masterplan.created" ? "Master plan" : "Plan"} created
        </span>
      </div>
    );
  }

  // step.dispatched, step.completed, step.failed
  const stepId = data?.step || "unknown";
  const icon = stepIcons[eventType] || (
    <Loader2 className="size-3 animate-spin text-muted-foreground" />
  );
  const label =
    eventType === "step.dispatched"
      ? "dispatched"
      : eventType === "step.completed"
        ? "completed"
        : "failed";

  return (
    <div className="mx-4 my-0.5 flex items-center gap-2 px-3 py-1 text-xs text-muted-foreground">
      {icon}
      <span>
        Step <span className="font-mono">{stepId}</span> {label}
      </span>
      {data?.error && (
        <span className="text-destructive">{data.error}</span>
      )}
    </div>
  );
}

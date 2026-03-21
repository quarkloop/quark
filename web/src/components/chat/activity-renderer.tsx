"use client";

import { useMemo } from "react";
import type { ActivityRecord } from "@/lib/types";
import { ACTIVITY_TYPES } from "@/lib/types";
import { MessageItem } from "./message-item";
import { ToolCallItem } from "./tool-call-item";
import { PlanItem } from "./plan-item";
import { SessionItem } from "./session-item";
import { PhaseItem } from "./phase-item";

interface ActivityRendererProps {
  activities: ActivityRecord[];
}

export function ActivityRenderer({ activities }: ActivityRendererProps) {
  // Pair tool.called with tool.completed by step+tool key.
  const paired = useMemo(() => pairToolEvents(activities), [activities]);

  return (
    <>
      {paired.map((item) => {
        if (item.kind === "message") {
          return (
            <MessageItem
              key={item.record.id}
              author={item.record.data?.author ?? "agent"}
              content={item.record.data?.content ?? ""}
              timestamp={item.record.timestamp}
            />
          );
        }

        if (item.kind === "tool") {
          return (
            <ToolCallItem
              key={item.record.id}
              toolName={item.record.data?.tool ?? "unknown"}
              args={item.record.data?.args}
              result={item.completed?.data?.result}
              isError={item.completed?.data?.is_error === "true"}
              timestamp={item.record.timestamp}
            />
          );
        }

        if (item.kind === "plan") {
          return (
            <PlanItem
              key={item.record.id}
              eventType={item.record.type}
              data={item.record.data}
              timestamp={item.record.timestamp}
            />
          );
        }

        if (item.kind === "session") {
          return (
            <SessionItem
              key={item.record.id}
              eventType={item.record.type}
              data={item.record.data}
              timestamp={item.record.timestamp}
            />
          );
        }

        if (item.kind === "phase") {
          return (
            <PhaseItem
              key={item.record.id}
              eventType={item.record.type}
              data={item.record.data}
            />
          );
        }

        // Skip other events (context.compacted, checkpoint.saved, mode.classified).
        return null;
      })}
    </>
  );
}

// ─── Internal types and helpers ───────────────────────────────

type PairedItem =
  | { kind: "message"; record: ActivityRecord }
  | { kind: "tool"; record: ActivityRecord; completed?: ActivityRecord }
  | { kind: "plan"; record: ActivityRecord }
  | { kind: "session"; record: ActivityRecord }
  | { kind: "phase"; record: ActivityRecord }
  | { kind: "other"; record: ActivityRecord };

const MESSAGE_TYPES = new Set<string>([ACTIVITY_TYPES.MESSAGE_ADDED]);
const TOOL_CALL_TYPE = ACTIVITY_TYPES.TOOL_CALLED;
const TOOL_COMPLETED_TYPE = ACTIVITY_TYPES.TOOL_COMPLETED;
const PLAN_TYPES = new Set<string>([
  ACTIVITY_TYPES.PLAN_CREATED,
  ACTIVITY_TYPES.PLAN_UPDATED,
  ACTIVITY_TYPES.MASTERPLAN_CREATED,
  ACTIVITY_TYPES.STEP_DISPATCHED,
  ACTIVITY_TYPES.STEP_COMPLETED,
  ACTIVITY_TYPES.STEP_FAILED,
]);
const SESSION_TYPES = new Set<string>([
  ACTIVITY_TYPES.SESSION_STARTED,
  ACTIVITY_TYPES.SESSION_ENDED,
]);
const PHASE_TYPES = new Set<string>([
  ACTIVITY_TYPES.PHASE_STARTED,
  ACTIVITY_TYPES.PHASE_COMPLETED,
  ACTIVITY_TYPES.PHASE_FAILED,
]);

function pairToolEvents(activities: ActivityRecord[]): PairedItem[] {
  // Pair each tool.called with the next unmatched tool.completed in order.
  // Using a queue per step:tool key so multiple calls with the same key
  // (e.g. two "ask:bash" calls) pair correctly by order.
  const completionQueues = new Map<string, ActivityRecord[]>();
  const completedIds = new Set<string>();

  for (const a of activities) {
    if (a.type === TOOL_COMPLETED_TYPE && a.data) {
      const key = `${a.data.step}:${a.data.tool}`;
      let queue = completionQueues.get(key);
      if (!queue) {
        queue = [];
        completionQueues.set(key, queue);
      }
      queue.push(a);
      completedIds.add(a.id);
    }
  }

  // Track consumption index per key.
  const consumed = new Map<string, number>();

  const result: PairedItem[] = [];
  for (const a of activities) {
    if (completedIds.has(a.id)) continue;

    if (MESSAGE_TYPES.has(a.type)) {
      result.push({ kind: "message", record: a });
    } else if (a.type === TOOL_CALL_TYPE) {
      const key = `${a.data?.step}:${a.data?.tool}`;
      const queue = completionQueues.get(key);
      const idx = consumed.get(key) ?? 0;
      const completed = queue?.[idx];
      if (completed) consumed.set(key, idx + 1);
      result.push({ kind: "tool", record: a, completed });
    } else if (PLAN_TYPES.has(a.type)) {
      result.push({ kind: "plan", record: a });
    } else if (SESSION_TYPES.has(a.type)) {
      result.push({ kind: "session", record: a });
    } else if (PHASE_TYPES.has(a.type)) {
      result.push({ kind: "phase", record: a });
    } else {
      result.push({ kind: "other", record: a });
    }
  }

  return result;
}

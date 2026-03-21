"use client";

import { useState } from "react";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Plus } from "lucide-react";
import type { AgentConnection } from "@/lib/types";

interface AddAgentDialogProps {
  onAdd: (agent: AgentConnection) => void;
}

export function AddAgentDialog({ onAdd }: AddAgentDialogProps) {
  const [open, setOpen] = useState(false);
  const [port, setPort] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const p = parseInt(port, 10);
    if (isNaN(p) || p < 1 || p > 65535) return;
    onAdd({
      id: `manual-${p}`,
      name: `Agent :${p}`,
      mode: "direct",
      baseUrl: `http://127.0.0.1:${p}`,
      port: p,
      status: "unknown",
    });
    setPort("");
    setOpen(false);
  };

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        render={
          <Button variant="ghost" size="sm" className="w-full justify-start gap-2" />
        }
      >
        <Plus className="size-4" />
        Add agent
      </SheetTrigger>
      <SheetContent side="left" className="w-72">
        <SheetHeader>
          <SheetTitle>Add Agent</SheetTitle>
        </SheetHeader>
        <form onSubmit={handleSubmit} className="mt-4 space-y-3">
          <Input
            type="number"
            placeholder="Port (e.g. 7100)"
            value={port}
            onChange={(e) => setPort(e.target.value)}
            min={1}
            max={65535}
          />
          <Button type="submit" size="sm" className="w-full">
            Connect
          </Button>
        </form>
      </SheetContent>
    </Sheet>
  );
}

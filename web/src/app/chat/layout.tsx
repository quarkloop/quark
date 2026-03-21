"use client";

import { AgentProvider } from "@/context/agent-context";
import { Header } from "@/components/layout/header";

export default function ChatLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AgentProvider>
      <div className="flex h-full flex-col">
        <Header />
        <div className="flex min-h-0 flex-1">{children}</div>
      </div>
    </AgentProvider>
  );
}

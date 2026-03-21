"use client";

import { AgentSidebar } from "@/components/sidebar/agent-sidebar";
import { ChatContainer } from "@/components/chat/chat-container";

export default function ChatPage() {
  return (
    <>
      <AgentSidebar />
      <main className="flex flex-1 flex-col overflow-hidden">
        <ChatContainer />
      </main>
    </>
  );
}

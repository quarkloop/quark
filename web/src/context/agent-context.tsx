"use client";

import {
  createContext,
  useContext,
  useReducer,
  useCallback,
  type ReactNode,
} from "react";
import type { AgentConnection, SessionRecord } from "@/lib/types";
import { useLocalStorage } from "@/hooks/use-local-storage";

interface AgentState {
  agents: AgentConnection[];
  activeAgentId: string | null;
  isDiscovering: boolean;
  sessions: SessionRecord[];
  activeSessionKey: string | null;
}

type AgentAction =
  | { type: "DISCOVER_START" }
  | { type: "DISCOVER_SUCCESS"; agents: AgentConnection[] }
  | { type: "SET_AGENTS"; agents: AgentConnection[] }
  | { type: "ADD_AGENT"; agent: AgentConnection }
  | { type: "REMOVE_AGENT"; id: string }
  | { type: "SET_ACTIVE"; id: string | null }
  | { type: "UPDATE_STATUS"; id: string; status: AgentConnection["status"] }
  | { type: "SET_SESSIONS"; sessions: SessionRecord[] }
  | { type: "SET_ACTIVE_SESSION"; key: string | null }
  | { type: "ADD_SESSION"; session: SessionRecord }
  | { type: "REMOVE_SESSION"; key: string };

function reducer(state: AgentState, action: AgentAction): AgentState {
  switch (action.type) {
    case "DISCOVER_START":
      return { ...state, isDiscovering: true };
    case "DISCOVER_SUCCESS": {
      // Merge: keep manually-added agents, update discovered ones.
      const manual = state.agents.filter(
        (a) => !action.agents.some((d) => d.port === a.port),
      );
      return {
        ...state,
        agents: [...action.agents, ...manual],
        isDiscovering: false,
      };
    }
    case "SET_AGENTS":
      return { ...state, agents: action.agents };
    case "ADD_AGENT":
      if (state.agents.some((a) => a.id === action.agent.id)) return state;
      return { ...state, agents: [...state.agents, action.agent] };
    case "REMOVE_AGENT":
      return {
        ...state,
        agents: state.agents.filter((a) => a.id !== action.id),
        activeAgentId:
          state.activeAgentId === action.id ? null : state.activeAgentId,
      };
    case "SET_ACTIVE":
      return { ...state, activeAgentId: action.id, sessions: [], activeSessionKey: null };
    case "UPDATE_STATUS":
      return {
        ...state,
        agents: state.agents.map((a) =>
          a.id === action.id ? { ...a, status: action.status } : a,
        ),
      };
    case "SET_SESSIONS": {
      // Auto-select main session if none active.
      let activeKey = state.activeSessionKey;
      if (!activeKey && action.sessions.length > 0) {
        const main = action.sessions.find((s) => s.type === "main");
        activeKey = main?.key ?? action.sessions[0].key;
      }
      return { ...state, sessions: action.sessions, activeSessionKey: activeKey };
    }
    case "SET_ACTIVE_SESSION":
      return { ...state, activeSessionKey: action.key };
    case "ADD_SESSION":
      return {
        ...state,
        sessions: [...state.sessions, action.session],
        activeSessionKey: action.session.key,
      };
    case "REMOVE_SESSION":
      return {
        ...state,
        sessions: state.sessions.filter((s) => s.key !== action.key),
        activeSessionKey:
          state.activeSessionKey === action.key
            ? state.sessions.find((s) => s.type === "main")?.key ?? null
            : state.activeSessionKey,
      };
    default:
      return state;
  }
}

interface AgentContextValue {
  state: AgentState;
  dispatch: React.Dispatch<AgentAction>;
  activeAgent: AgentConnection | undefined;
  activeSession: SessionRecord | undefined;
}

const AgentContext = createContext<AgentContextValue | null>(null);

export function AgentProvider({ children }: { children: ReactNode }) {
  const [savedAgents, setSavedAgents] = useLocalStorage<AgentConnection[]>(
    "quark-agents",
    [],
  );

  const [state, dispatch] = useReducer(reducer, {
    agents: savedAgents,
    activeAgentId: null,
    isDiscovering: false,
    sessions: [],
    activeSessionKey: null,
  });

  // Sync agents to localStorage on changes.
  const wrappedDispatch = useCallback(
    (action: AgentAction) => {
      dispatch(action);
      // After certain actions, persist to localStorage.
      if (
        action.type === "DISCOVER_SUCCESS" ||
        action.type === "ADD_AGENT" ||
        action.type === "REMOVE_AGENT" ||
        action.type === "SET_AGENTS"
      ) {
        // We use a microtask to get the updated state after the reducer runs.
        queueMicrotask(() => {
          // Re-read from the reducer isn't possible here, so we compute it.
          // For simplicity, just let the next render persist via effect.
        });
      }
      // Persist on every dispatch for simplicity.
      if (action.type === "DISCOVER_SUCCESS") {
        const manual = savedAgents.filter(
          (a) => !action.agents.some((d) => d.port === a.port),
        );
        setSavedAgents([...action.agents, ...manual]);
      } else if (action.type === "ADD_AGENT") {
        setSavedAgents((prev) =>
          prev.some((a) => a.id === action.agent.id)
            ? prev
            : [...prev, action.agent],
        );
      } else if (action.type === "REMOVE_AGENT") {
        setSavedAgents((prev) => prev.filter((a) => a.id !== action.id));
      }
    },
    [savedAgents, setSavedAgents],
  );

  const activeAgent = state.agents.find((a) => a.id === state.activeAgentId);
  const activeSession = state.sessions.find((s) => s.key === state.activeSessionKey);

  return (
    <AgentContext value={{ state, dispatch: wrappedDispatch, activeAgent, activeSession }}>
      {children}
    </AgentContext>
  );
}

export function useAgentContext() {
  const ctx = useContext(AgentContext);
  if (!ctx) throw new Error("useAgentContext must be used within AgentProvider");
  return ctx;
}

"use client";

import { useCallback, useEffect, useRef } from "react";
import { useAgentContext } from "@/context/agent-context";
import { discoverAgents } from "@/lib/api-client";
import type { AgentConnection } from "@/lib/types";

export function useAgents() {
  const { state, dispatch, activeAgent } = useAgentContext();
  const discoveredOnce = useRef(false);

  const discover = useCallback(async () => {
    dispatch({ type: "DISCOVER_START" });
    try {
      const agents = await discoverAgents();
      dispatch({ type: "DISCOVER_SUCCESS", agents });
    } catch {
      dispatch({ type: "DISCOVER_SUCCESS", agents: [] });
    }
  }, [dispatch]);

  const addAgent = useCallback(
    (agent: AgentConnection) => {
      dispatch({ type: "ADD_AGENT", agent });
    },
    [dispatch],
  );

  const removeAgent = useCallback(
    (id: string) => {
      dispatch({ type: "REMOVE_AGENT", id });
    },
    [dispatch],
  );

  const setActive = useCallback(
    (id: string | null) => {
      dispatch({ type: "SET_ACTIVE", id });
    },
    [dispatch],
  );

  // Discover once on mount.
  useEffect(() => {
    if (discoveredOnce.current) return;
    discoveredOnce.current = true;
    discover();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return {
    agents: state.agents,
    activeAgent,
    isDiscovering: state.isDiscovering,
    discover,
    addAgent,
    removeAgent,
    setActive,
  };
}

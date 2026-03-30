"use client";

import { QueryProvider } from "@/providers/query-provider";
import type { ReactNode } from "react";

export function Providers({ children }: { children: ReactNode }) {
  return <QueryProvider>{children}</QueryProvider>;
}

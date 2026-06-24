import type { BaseLayoutProps } from "fumadocs-ui/layouts/shared";
import { Logo } from "@/components/logo";

/**
 * Shared layout options used by every Fumadocs layout (docs, home, search).
 * Keeps nav consistent across routes.
 */
export const baseOptions: BaseLayoutProps = {
  nav: {
    title: <Logo size="sm" />,
  },
  links: [
    {
      text: "Docs",
      url: "/docs",
      active: "nested-url",
    },
    {
      text: "Bootstrap",
      url: "/docs/environment-bootstrap",
      active: "nested-url",
    },
    {
      text: "Spec",
      url: "/docs/declaration",
      active: "nested-url",
    },
  ],
};

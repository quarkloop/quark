import {
  CodeBlock,
  type CodeBlockProps,
  Pre,
} from "fumadocs-ui/components/codeblock";
import { Callout } from "fumadocs-ui/components/callout";
import { Steps, Step } from "fumadocs-ui/components/steps";
import { Tab, Tabs } from "fumadocs-ui/components/tabs";
import { TypeTable } from "fumadocs-ui/components/type-table";
import { Card, Cards } from "fumadocs-ui/components/card";
import { File, Files } from "fumadocs-ui/components/files";
import FumadocsMdx from "fumadocs-ui/mdx";
import type { MDXComponents } from "mdx/types";
import type { ComponentProps, ReactNode } from "react";

type PreProps = ComponentProps<typeof Pre>;

function _PreComponent({ children, ...props }: PreProps) {
  return (
    <CodeBlock {...(props as CodeBlockProps)}>
      <Pre>{children}</Pre>
    </CodeBlock>
  );
}

/**
 * Default MDX components used across all docs pages.
 * Premium tech aesthetic — accent borders, generous spacing,
 * type-safe tables, and proper syntax highlighting.
 */
const defaultMdxComponents: MDXComponents = {
  ...FumadocsMdx,
  pre: _PreComponent as unknown as (props: PreProps) => ReactNode,
  Callout,
  Steps,
  Step,
  Tab,
  Tabs,
  TypeTable,
  Card,
  Cards,
  File,
  Files,
  // Style overrides for premium feel
  h1: ({ children }) => (
    <h1 className="font-display mt-2 text-3xl font-semibold tracking-tight text-balance">
      {children}
    </h1>
  ),
  h2: ({ children }) => (
    <h2 className="font-display mt-10 mb-3 text-xl font-semibold tracking-tight border-b border-border/60 pb-2 scroll-mt-24">
      {children}
    </h2>
  ),
  h3: ({ children }) => (
    <h3 className="font-display mt-8 mb-2 text-base font-semibold tracking-tight scroll-mt-24">
      {children}
    </h3>
  ),
  p: ({ children }) => (
    <p className="text-[0.95rem] leading-[1.7] text-sand-300 my-4">
      {children}
    </p>
  ),
  a: ({ children, href }) => (
    <a
      href={href}
      className="text-ember-400 hover:text-ember-300 underline decoration-ember-500/30 underline-offset-2 hover:decoration-ember-400 transition-colors"
      target={href?.startsWith("http") ? "_blank" : undefined}
      rel={href?.startsWith("http") ? "noopener noreferrer" : undefined}
    >
      {children}
    </a>
  ),
  ul: ({ children }) => (
    <ul className="my-4 ml-1 space-y-2 text-[0.95rem] leading-[1.7] text-sand-300">
      {children}
    </ul>
  ),
  ol: ({ children }) => (
    <ol className="my-4 ml-1 space-y-2 text-[0.95rem] leading-[1.7] text-sand-300 list-decimal pl-5 marker:text-ember-400 marker:font-medium">
      {children}
    </ol>
  ),
  li: ({ children }) => (
    <li className="pl-1 relative before:content-[''] before:absolute before:left-[-1rem] before:top-[0.7rem] before:h-1 before:w-1 before:rounded-full before:bg-ember-500/60">
      {children}
    </li>
  ),
  blockquote: ({ children }) => (
    <blockquote className="my-6 border-l-2 border-ember-500/40 pl-4 italic text-sand-400">
      {children}
    </blockquote>
  ),
  code: ({ children }) => (
    <code className="rounded-md bg-ember-500/10 border border-ember-500/20 px-1.5 py-0.5 text-[0.85em] font-mono text-ember-300">
      {children}
    </code>
  ),
  table: ({ children }) => (
    <div className="my-6 overflow-hidden rounded-xl border border-border/60">
      <table className="w-full text-sm">{children}</table>
    </div>
  ),
  thead: ({ children }) => (
    <thead className="border-b border-border/60 bg-card/50">{children}</thead>
  ),
  th: ({ children }) => (
    <th className="px-4 py-2.5 text-left font-display font-semibold text-sand-200">
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td className="border-t border-border/40 px-4 py-2.5 text-sand-400 align-top">
      {children}
    </td>
  ),
  hr: () => <hr className="my-8 border-border/40" />,
  strong: ({ children }) => (
    <strong className="font-semibold text-sand-100">{children}</strong>
  ),
};

export default defaultMdxComponents;

import { source } from "@/lib/source";
import {
  DocsPage,
  DocsBody,
  DocsDescription,
  DocsTitle,
} from "fumadocs-ui/page";
import { notFound } from "next/navigation";
import defaultMdxComponents from "@/mdx-components";
import type { ComponentType } from "react";

export default async function Page({
  params,
}: {
  params: Promise<{ slug?: string[] }>;
}) {
  const { slug } = await params;
  const page = source.getPage(slug);
  if (!page) notFound();

  // The body is the compiled MDX component — cast for type safety.
  const data = page.data as {
    body: ComponentType<{ components?: Record<string, unknown> }>;
    toc?: unknown;
    full?: boolean;
  };
  const MDX = data.body;
  const toc = Array.isArray(data.toc) ? data.toc : undefined;
  const full = data.full;

  return (
    <DocsPage
      toc={toc as never}
      full={full}
      tableOfContent={{ header: "On this page" }}
      article={{ className: "max-w-3xl" }}
    >
      <DocsTitle className="font-display tracking-tight">
        {page.data.title}
      </DocsTitle>
      <DocsDescription className="text-base text-ink-400">
        {page.data.description}
      </DocsDescription>
      <DocsBody>
        <MDX components={{ ...defaultMdxComponents }} />
      </DocsBody>
    </DocsPage>
  );
}

export async function generateStaticParams() {
  return source.generateParams();
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug?: string[] }>;
}) {
  const { slug } = await params;
  const page = source.getPage(slug);
  if (!page) return {};
  return {
    title: page.data.title,
    description: page.data.description,
  };
}

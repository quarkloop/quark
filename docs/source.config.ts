import { defineDocs, defineConfig } from "fumadocs-mdx/config";
import { z } from "zod";

export const docs = defineDocs({
  dir: "content/docs",
  docs: {
    schema: z.object({
      title: z.string().default("Untitled"),
      description: z.string().default(""),
      status: z.string().optional(),
      date: z.string().optional(),
    }),
  },
});

export default defineConfig();

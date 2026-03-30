<!-- BEGIN:nextjs-agent-rules -->
# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` before writing code. Heed deprecation notices.
<!-- END:nextjs-agent-rules -->

## Component layer

Two-tier structure using shadcn (base-nova style, @base-ui/react):

```
components/
  ui/         ← shadcn primitives (managed by `npx shadcn add`). Never hand-edit.
  themed/     ← import boundary. All consumers import from here.
```

- **`ui/`**: Generated/updated by the shadcn CLI. Updated via `npx shadcn@latest add <component> --overwrite`.
- **`themed/`**: Re-exports from `ui/`. When custom brand variants are needed, they are defined here. Import path for all consumer code: `@/components/themed/...`.
- Theme is driven by CSS variables in `globals.css` (`:root` + `@theme inline`).

**Rule:** Never import from `@/components/ui/` directly in page or component code. Always use `@/components/themed/`.

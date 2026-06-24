import type { LucideIcon } from "lucide-react";

/**
 * A single feature item in the home page features section.
 * Icon + title + description, horizontal layout.
 */
export function FeatureItem({
  icon: Icon,
  title,
  description,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
}) {
  return (
    <div className="flex gap-4">
      <div className="flex-shrink-0 mt-0.5 inline-flex h-9 w-9 items-center justify-center rounded-lg bg-accent-500/10 text-accent-500">
        <Icon className="h-4 w-4" />
      </div>
      <div>
        <h3 className="font-display text-base font-semibold">{title}</h3>
        <p className="mt-1 text-sm text-ink-400 leading-relaxed">{description}</p>
      </div>
    </div>
  );
}

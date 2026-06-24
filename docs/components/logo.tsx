import Link from "next/link";

/**
 * The Quark Platform logo — a solid amber square with the letter "Q".
 * Used in the nav bar and footer.
 *
 * No gradients — solid color only, per the design system.
 */
export function Logo({
  size = "md",
  withText = true,
}: {
  size?: "sm" | "md" | "lg";
  withText?: boolean;
}) {
  const dimensions = {
    sm: { box: "h-7 w-7", text: "text-xs", label: "text-sm" },
    md: { box: "h-8 w-8", text: "text-sm", label: "text-base" },
    lg: { box: "h-10 w-10", text: "text-base", label: "text-lg" },
  }[size];

  return (
    <Link href="/" className="flex items-center gap-2.5 group">
      <div
        className={`relative ${dimensions.box} rounded-lg bg-accent-500 shadow-glow-sm flex items-center justify-center transition-colors group-hover:bg-accent-400`}
      >
        <span className={`font-display ${dimensions.text} font-bold text-ink-950`}>
          Q
        </span>
      </div>
      {withText && (
        <span className={`font-display ${dimensions.label} font-semibold tracking-tight`}>
          Quark Platform
        </span>
      )}
    </Link>
  );
}

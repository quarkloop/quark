import Link from "next/link";

export default function NotFound() {
  return (
    <main className="min-h-screen flex items-center justify-center px-6">
      <div className="text-center">
        <p className="font-display text-7xl font-semibold text-accent-500">404</p>
        <h1 className="mt-4 font-display text-2xl font-semibold tracking-tight">
          Page not found
        </h1>
        <p className="mt-2 text-ink-400">
          The page you're looking for doesn't exist or has been moved.
        </p>
        <Link
          href="/"
          className="mt-6 inline-flex items-center gap-2 rounded-lg bg-accent-500 px-4 py-2 text-sm font-semibold text-ink-950 hover:bg-accent-400 transition-colors"
        >
          ← Back home
        </Link>
      </div>
    </main>
  );
}

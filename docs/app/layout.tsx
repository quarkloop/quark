import type { ReactNode } from "react";
import { Inter, JetBrains_Mono } from "next/font/google";
import { RootProvider } from "fumadocs-ui/provider";
import "./global.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
});

const jetbrains = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains",
  display: "swap",
});

export const metadata = {
  title: {
    default: "Quark Platform — Documentation",
    template: "%s · Quark Platform",
  },
  description:
    "A universal runtime for programmable nodes. Three-service architecture with a Java/Native control plane, Go + SQLite Catalog, and a GraalJS-powered data plane.",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${jetbrains.variable}`}
      suppressHydrationWarning
    >
      <head>
        {/* Theme bootstrap — avoid FOUC by setting the class before hydration */}
        <script
          dangerouslySetInnerHTML={{
            __html: `
              (function() {
                try {
                  const theme = localStorage.getItem('theme');
                  const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
                  if (theme === 'dark' || (!theme && systemDark)) {
                    document.documentElement.classList.add('dark');
                  }
                } catch (e) {}
              })();
            `,
          }}
        />
      </head>
      <body className="font-sans antialiased">
        <RootProvider>{children}</RootProvider>
      </body>
    </html>
  );
}

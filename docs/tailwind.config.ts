import type { Config } from "tailwindcss";

/**
 * Tailwind configuration for the Quark Platform documentation.
 *
 * Design language: warm, editorial, premium. Solid colors only — no gradients.
 * The accent is amber/orange (warm); the neutral base is a warm-tinted stone.
 */
const config: Config = {
  content: [
    "./app/**/*.{ts,tsx,mdx}",
    "./components/**/*.{ts,tsx}",
    "./content/**/*.{md,mdx}",
    "./lib/**/*.{ts,tsx}",
    "./mdx-components.tsx",
    "./node_modules/fumadocs-ui/dist/**/*.js",
  ],
  darkMode: "class",
  theme: {
    extend: {
      fontFamily: {
        sans: ["var(--font-inter)", "system-ui", "sans-serif"],
        mono: ["var(--font-jetbrains)", "ui-monospace", "monospace"],
        display: ["var(--font-inter)", "system-ui", "sans-serif"],
      },
      colors: {
        // Warm neutral base — stone with a slight amber undertone
        ink: {
          50: "#faf9f7",
          100: "#f3f1ec",
          200: "#e8e4dc",
          300: "#d4cdc0",
          400: "#a89e8d",
          500: "#7a7060",
          600: "#5c5448",
          700: "#463f36",
          800: "#2c2823",
          900: "#1a1814",
          950: "#0e0d0a",
        },
        // Warm accent — amber/orange
        accent: {
          50: "#fff8ed",
          100: "#ffefd4",
          200: "#fedba8",
          300: "#fdc070",
          400: "#fb9e3c",
          500: "#f97316",
          600: "#ea5a0c",
          700: "#c2410c",
          800: "#9a3412",
          900: "#7c2d12",
          950: "#431407",
        },
      },
      boxShadow: {
        // Warm glow using accent color (no gradient, just a solid color shadow)
        glow: "0 0 40px -10px rgba(249,115,22,0.30)",
        "glow-sm": "0 0 20px -5px rgba(249,115,22,0.22)",
        premium:
          "0 1px 0 0 rgba(255,255,255,0.04) inset, 0 1px 2px 0 rgba(0,0,0,0.4), 0 8px 24px -8px rgba(0,0,0,0.6)",
      },
      transitionTimingFunction: {
        premium: "cubic-bezier(0.16, 1, 0.3, 1)",
      },
      animation: {
        "fade-in": "fade-in 0.5s cubic-bezier(0.16, 1, 0.3, 1) both",
        "slide-up": "slide-up 0.5s cubic-bezier(0.16, 1, 0.3, 1) both",
        "pulse-slow": "pulse 4s cubic-bezier(0.4, 0, 0.6, 1) infinite",
      },
      keyframes: {
        "fade-in": {
          "0%": { opacity: "0" },
          "100%": { opacity: "1" },
        },
        "slide-up": {
          "0%": { opacity: "0", transform: "translateY(8px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
      },
    },
  },
};

export default config;

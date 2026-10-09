import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}", "./lib/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        brand: {
          50: "#ecfdf5",
          100: "#d1fae5",
          500: "#059669",
          600: "#059669",
          700: "#047857",
        },
        accent: {
          400: "#F59E0B",
          500: "#F59E0B",
          600: "#d97706",
        },
      },
    },
  },
  plugins: [],
};

export default config;

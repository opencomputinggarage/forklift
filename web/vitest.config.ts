import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: "./test/setup.ts",
    include: ["test/**/*.{test,spec}.{ts,tsx}"],
    exclude: ["test/e2e/**"],
    passWithNoTests: true,
    clearMocks: true,
    restoreMocks: true,
  },
});

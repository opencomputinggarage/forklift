import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

// The build output is embedded into the Go binary (internal/webui/dist).
export default defineConfig({
  plugins: [
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
      generatedRouteTree: "./src/generated/route-tree.gen.ts",
    }),
    tailwindcss(),
    react(),
  ],
  build: {
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    host: "127.0.0.1",
    // During `npm run dev`, proxy API and package routes to the Go server.
    proxy: {
      "/api": "http://localhost:8080",
      "/api-docs": "http://localhost:8080",
      "/auth": "http://localhost:8080",
      "/openapi.yaml": "http://localhost:8080",
      "/maven": "http://localhost:8080",
      "/npm": "http://localhost:8080",
      "/cargo": "http://localhost:8080",
      "/go": "http://localhost:8080",
    },
  },
});

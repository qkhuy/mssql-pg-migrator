import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

// Wails serves the built assets from frontend/dist.
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});

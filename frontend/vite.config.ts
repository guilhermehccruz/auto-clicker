import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

// Wails serves the built assets from frontend/dist; `wails3 dev` runs this dev
// server on WAILS_VITE_PORT and points the webview at it.
export default defineConfig({
  plugins: [wails("./bindings"), react(), tailwindcss()],
  build: { outDir: "dist", emptyOutDir: true },
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
});

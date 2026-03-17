import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: {
    chunkSizeWarningLimit: 800,
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8484",
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on("error", (err, _req, res) => {
            if ("code" in err && err.code === "ECONNREFUSED" && res && "writeHead" in res) {
              res.writeHead(503, { "Content-Type": "application/json", "Retry-After": "1" });
              res.end(JSON.stringify({ error: "API server not ready" }));
            }
          });
        },
      },
    },
  },
});

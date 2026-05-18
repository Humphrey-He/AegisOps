import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: {
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) {
            return undefined;
          }
          if (
            id.includes("/react/") ||
            id.includes("/react-dom/") ||
            id.includes("/react-router/") ||
            id.includes("/react-router-dom/") ||
            id.includes("/scheduler/")
          ) {
            return "vendor-react";
          }
          if (id.includes("@tanstack")) {
            return "vendor-query";
          }
          if (id.includes("/antd/") || id.includes("@ant-design") || id.includes("/rc-") || id.includes("@rc-component")) {
            return "vendor-antd";
          }
          if (
            id.includes("/dayjs/") ||
            id.includes("/classnames/") ||
            id.includes("/lodash") ||
            id.includes("/async-validator/") ||
            id.includes("@babel/runtime")
          ) {
            return "vendor-utils";
          }
          return undefined;
        },
      },
    },
  },
  server: {
    host: "0.0.0.0",
    port: 4173,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
      },
    },
  },
});

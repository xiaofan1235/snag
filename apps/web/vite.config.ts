/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";

// Vite + React + Vitest（jsdom）。别名 @ → src。原生 EventSource，不引三方 SSE 库（UI 文档 §16.32）。
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  // 开发代理：前端 same-origin 的 /v1 请求转发到本地 snagd（默认 127.0.0.1:8787）。
  // 生产由 snagd 或反向代理同源托管，故 client.ts 默认 baseUrl="" 即可。
  server: {
    proxy: {
      "/v1": { target: "http://127.0.0.1:8787", changeOrigin: false },
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    css: false,
  },
});

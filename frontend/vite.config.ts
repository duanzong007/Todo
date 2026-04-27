import { defineConfig, loadEnv } from "vite";
import vue from "@vitejs/plugin-vue";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const backendURL = env.TODO_BACKEND_URL || "http://localhost:8080";

  return {
    plugins: [vue()],
    server: {
      port: 5173,
      strictPort: false,
      proxy: {
        "/dashboard": {
          target: backendURL,
          changeOrigin: true,
        },
        "/events": {
          target: backendURL,
          changeOrigin: true,
          ws: false,
        },
        "/tasks": {
          target: backendURL,
          changeOrigin: true,
        },
        "/imports": {
          target: backendURL,
          changeOrigin: true,
        },
        "/sms": {
          target: backendURL,
          changeOrigin: true,
        },
        "/me": {
          target: backendURL,
          changeOrigin: true,
        },
        "/login": {
          target: backendURL,
          changeOrigin: true,
        },
        "/logout": {
          target: backendURL,
          changeOrigin: true,
        },
      },
    },
  };
});

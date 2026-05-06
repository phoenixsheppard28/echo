import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), wails("./bindings")],
  // Bind to IPv4 explicitly. The Wails3 dev proxy forces tcp4 connections
  // to localhost (build_dev.go), but Node binds to IPv6 (::1) by default
  // on macOS, which causes "connection refused" proxy errors.
  server: {
    host: "127.0.0.1",
  },
});

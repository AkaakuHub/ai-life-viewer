import react from "@vitejs/plugin-react";
import { defineConfig } from "vite-plus";

export default defineConfig({
  plugins: [react()],
  build: {
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: "calendar",
              test: /node_modules\/(?:@fullcalendar|@full-ui|temporal-polyfill|temporal-spec)\//,
            },
            { name: "mantine", test: /node_modules\/(?:@mantine|@floating-ui)\// },
          ],
        },
      },
    },
  },
  server: { host: "127.0.0.1", proxy: { "/api": "http://127.0.0.1:4180" } },
});

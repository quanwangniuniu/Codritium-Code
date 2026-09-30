import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

// Pure-module unit tests. No DOM: components are covered by the build and
// the smoke test; a test that needs a browser environment should opt in
// per-file with `// @vitest-environment jsdom` once a DOM lib is added.
export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    passWithNoTests: true,
  },
});

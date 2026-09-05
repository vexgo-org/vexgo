import { defineConfig } from "orval";

export default defineConfig({
  vexgo: {
    input: {
      target: "../docs/openapi.json",
    },
    output: {
      mode: "tags-split",
      client: "axios",
      target: "src/api/generated/endpoints.ts",
      schemas: "src/api/generated/model",
      clean: true,
      baseURL: "import.meta.env.VITE_API_URL || '/api'",
    },
  },
});

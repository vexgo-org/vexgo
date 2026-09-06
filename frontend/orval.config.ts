import { defineConfig } from "orval";

export default defineConfig({
  vexgo: {
    input: { target: "../docs/swagger.json" },
    output: {
      mode: "split",
      client: "axios",
      target: "src/api/generated/endpoints.ts",
      schemas: "src/api/generated/model",
      clean: true,
      override: {
        mutator: {
          path: "./src/api/customAxios.ts",
          name: "customInstance",
        },
      },
    },
  },
});

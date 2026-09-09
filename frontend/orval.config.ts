import { defineConfig } from "orval";

export default defineConfig({
  "vexgo-sdk": {
    input: { target: "../docs/swagger.json" },
    output: {
      // Axios-style call shape; the mutator below implements the same
      // config contract on top of ofetch and returns unwrapped bodies.
      mode: "split",
      client: "axios",
      formatter: "prettier",
      target: "packages/sdk/src/generated/endpoints.ts",
      tsconfig: {
        compilerOptions: {
          target: "es2022",
        },
      },
      schemas: "packages/sdk/src/generated/model",
      clean: true,
      override: {
        mutator: {
          path: "./packages/sdk/src/_mutator.ts",
          name: "customFetchInstance",
        },
      },
    },
  },
});

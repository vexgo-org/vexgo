import { defineConfig } from "orval";

export default defineConfig({
  vexgo: {
    input: { target: "../docs/swagger.json" },
    output: {
      mode: "split",
      client: "axios",
      formatter: "prettier",
      target: "src/api/generated/endpoints.ts",
      tsconfig: {
        compilerOptions: {
          target: "es2022",
        },
      },
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
  "vexgo-sdk": {
    input: { target: "../docs/swagger.json" },
    output: {
      // Same axios-style call shape as the legacy frontend target so the
      // handwritten SDK layer can mirror it; the mutator below implements
      // the same config contract on top of ofetch and returns unwrapped
      // bodies (no `unwrap(response.data)` needed).
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

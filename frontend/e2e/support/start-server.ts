import { spawn } from "node:child_process";
import { mkdir, rm } from "node:fs/promises";
import { dirname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const supportDir = dirname(fileURLToPath(import.meta.url));
const frontendDir = resolve(supportDir, "../..");
const repositoryDir = resolve(frontendDir, "..");
const runtimeDir = resolve(frontendDir, ".playwright");
const dataDir = resolve(runtimeDir, "vexgo-data");
const binaryPath = resolve(runtimeDir, "vexgo-e2e");

if (!dataDir.startsWith(`${runtimeDir}${sep}`)) {
  throw new Error(`Refusing to reset data outside ${runtimeDir}`);
}

type CommandOptions = {
  cwd: string;
  env?: NodeJS.ProcessEnv;
};

function runCommand(
  command: string,
  args: string[],
  options: CommandOptions,
): Promise<void> {
  return new Promise((resolveCommand, rejectCommand) => {
    const child = spawn(command, args, {
      cwd: options.cwd,
      env: options.env ?? process.env,
      stdio: "inherit",
    });

    child.once("error", rejectCommand);
    child.once("exit", (code, signal) => {
      if (code === 0) {
        resolveCommand();
        return;
      }

      rejectCommand(
        new Error(
          `${command} exited with ${signal ? `signal ${signal}` : `code ${code}`}`,
        ),
      );
    });
  });
}

await mkdir(runtimeDir, { recursive: true });
await rm(dataDir, { recursive: true, force: true });
await mkdir(dataDir, { recursive: true });

await runCommand(process.execPath, ["run", "build"], {
  cwd: frontendDir,
});

await runCommand(
  "go",
  [
    "build",
    "-trimpath",
    "-tags",
    "nomsgpack",
    "-o",
    binaryPath,
    "backend/cmd/vexgo/main.go",
  ],
  { cwd: repositoryDir },
);

const serverEnvironment: NodeJS.ProcessEnv = {
  ...process.env,
  ADDR: "127.0.0.1",
  PORT: "3101",
  DATA_DIR: dataDir,
  BASE_URL: "http://127.0.0.1:3101",
  DB_TYPE: "sqlite",
  DB_HOST: "",
  DB_PORT: "",
  DB_USER: "",
  DB_PASSWORD: "",
  DB_NAME: "",
  DB_SSL_MODE: "",
  JWT_SECRET: "vexgo-playwright-e2e-jwt-secret",
  SETTINGS_ENCRYPTION_KEY: "vexgo-playwright-e2e-encryption-key",
  ALLOW_LOCAL_LOGIN: "true",
  AUTH_RATE_LIMIT_PER_MINUTE: "0",
  CAPTCHA_RATE_LIMIT_PER_MINUTE: "0",
  CACHE_ENABLED: "false",
  VALKEY_ENABLED: "false",
  VALKEY_URL: "",
  S3_ENABLED: "false",
  S3_ENDPOINT: "",
  S3_REGION: "",
  S3_BUCKET: "",
  S3_ACCESS_KEY: "",
  S3_SECRET_KEY: "",
  OIDC_ENABLED: "false",
  OIDC_ISSUER_URL: "",
  OIDC_CLIENT_ID: "",
  OIDC_CLIENT_SECRET: "",
  GITHUB_CLIENT_ID: "",
  GITHUB_CLIENT_SECRET: "",
  GOOGLE_CLIENT_ID: "",
  GOOGLE_CLIENT_SECRET: "",
  GIN_MODE: "release",
  BEHIND_REVERSE_PROXY: "false",
  TRUSTED_PROXIES: "",
  FRONTEND_URL: "http://127.0.0.1:3101",
};

const server = spawn(
  binaryPath,
  ["server", "--addr", "127.0.0.1", "--port", "3101", "--data", dataDir],
  {
    cwd: repositoryDir,
    env: serverEnvironment,
    stdio: "inherit",
  },
);

let shutdownRequested = false;

const forwardSignal = (signal: NodeJS.Signals) => {
  shutdownRequested = true;
  if (!server.killed) {
    server.kill(signal);
  }
};

process.once("SIGINT", () => forwardSignal("SIGINT"));
process.once("SIGTERM", () => forwardSignal("SIGTERM"));

server.once("error", (error) => {
  throw error;
});

const exitCode = await new Promise<number>((resolveExit) => {
  server.once("exit", (code, signal) => {
    if (shutdownRequested || signal === "SIGINT" || signal === "SIGTERM") {
      resolveExit(0);
      return;
    }

    resolveExit(code ?? 1);
  });
});

process.exitCode = exitCode;

import { cp } from "node:fs/promises";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";

// Next's standalone server needs its static assets next to the traced build.
// Bind locally unless the operator explicitly selects another host.
const args = process.argv.slice(2);
let port = process.env.PORT || "3000";
let host = process.env.FRONTEND_HOST || "127.0.0.1";
for (let i = 0; i < args.length; i += 2) {
  if (!args[i + 1]) throw new Error("Missing value for " + args[i]);
  if (args[i] === "--port") port = args[i + 1];
  else if (args[i] === "--hostname") host = args[i + 1];
  else throw new Error("Unknown option: " + args[i]);
}
await cp(
  new URL("../.next/static", import.meta.url),
  new URL("../.next/standalone/.next/static", import.meta.url),
  { recursive: true },
);
const child = spawn(
  process.execPath,
  [fileURLToPath(new URL("../.next/standalone/server.js", import.meta.url))],
  {
    stdio: "inherit",
    env: {
      ...process.env,
      NODE_ENV: "production",
      NEXT_TELEMETRY_DISABLED: "1",
      HOSTNAME: host,
      PORT: port,
    },
  },
);
for (const signal of ["SIGINT", "SIGTERM"])
  process.on(signal, () => child.kill(signal));
child.on("error", (error) => {
  console.error(error.message);
  process.exit(1);
});
child.on("exit", (code, signal) =>
  process.exit(code ?? (signal === "SIGINT" ? 130 : 143)),
);

const cp = require("node:child_process"),
  path = require("node:path");
const root = path.resolve(__dirname, ".."),
  command = process.argv[2] || "api";
const modes = {
  api: {
    args: [path.join(__dirname, "go.cjs"), "run", "./cmd/community"],
    cwd: root,
  },
  web: {
    args: [
      path.join(root, "web/community/node_modules/vite/bin/vite.js"),
      "--host",
      "127.0.0.1",
    ],
    cwd: path.join(root, "web/community"),
  },
  "review-api": {
    args: [path.join(__dirname, "go.cjs"), "run", "./cmd/review"],
    cwd: root,
  },
  "review-web": {
    args: [
      path.join(root, "web/review/node_modules/vite/bin/vite.js"),
      "--host",
      "127.0.0.1",
    ],
    cwd: path.join(root, "web/review"),
  },
  mail: {
    args: [path.join(__dirname, "go.cjs"), "run", "./cmd/devmail"],
    cwd: root,
  },
};
const mode = modes[command];
if (!mode) {
  console.error(
    "usage: node scripts/dev.cjs api|web|review-api|review-web|mail",
  );
  process.exit(1);
}
const env = {
  ...process.env,
  APP_ENV: "development",
  HTTP_ADDR: "127.0.0.1:8080",
  APP_ORIGIN: "http://localhost:5173",
  REVIEW_ADDR: "127.0.0.1:8081",
  REVIEW_ORIGIN: "http://localhost:5174",
  MYSQL_DSN:
    process.env.MYSQL_DSN ||
    "talko:talko_local@tcp(127.0.0.1:3307)/talko?parseTime=true&multiStatements=true&charset=utf8mb4&loc=UTC",
  SMTP_ADDR: process.env.SMTP_ADDR || "127.0.0.1:1025",
  SMTP_FROM: "noreply@talko.local",
};
const p = cp.spawn(process.execPath, mode.args, {
  cwd: mode.cwd,
  env,
  stdio: "inherit",
  windowsHide: true,
});
p.on("error", (e) => {
  console.error(e.message);
  process.exit(1);
});
p.on("exit", (code) => process.exit(code ?? 1));

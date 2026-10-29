// Development launcher. Leaves explicitly requested services running, loopback only.
const fs = require("node:fs"),
  path = require("node:path"),
  cp = require("node:child_process"),
  net = require("node:net");
const root = path.resolve(__dirname, ".."),
  tmp = path.join(root, ".tmp");
fs.mkdirSync(tmp, { recursive: true });
const services = [
  { id: "api", url: "http://localhost:8080/readyz", port: 8080 },
  { id: "web", url: "http://localhost:5173", port: 5173 },
  { id: "review-api", url: "http://localhost:8081/readyz", port: 8081 },
  { id: "review-web", url: "http://localhost:5174", port: 5174 },
  { id: "mail", url: "http://localhost:8025", port: 8025 },
];
const healthy = async (s) => {
  try {
    const r = await fetch(s.url, { signal: AbortSignal.timeout(1500) });
    const text = await r.text();
    return (
      r.ok &&
      (s.id.endsWith("api")
        ? text.includes('"status":"ready"')
        : text.includes("Talko") || text.includes("inbox"))
    );
  } catch {
    return false;
  }
};
const available = (port) =>
  new Promise((resolve) => {
    const server = net.createServer();
    server.once("error", () => resolve(false));
    server.listen(port, "127.0.0.1", () => server.close(() => resolve(true)));
  });
(async () => {
  const command = process.argv[2] || "status";
  if (!["start", "status"].includes(command))
    throw Error("usage: node scripts/local.cjs start|status");
  const [major, minor] = process.versions.node.split(".").map(Number);
  if (major < 22 || (major === 22 && minor < 12))
    throw Error("Node 22.12+ required; Node 24 recommended");
  let state = {};
  try {
    state = JSON.parse(
      fs.readFileSync(path.join(tmp, "services.json"), "utf8"),
    );
  } catch {}
  if (command === "start") {
    const migration = cp.spawnSync(
      process.execPath,
      [path.join(root, "scripts/run.cjs"), "run", "./cmd/community", "migrate"],
      { cwd: root, stdio: "inherit", windowsHide: true },
    );
    if (migration.status !== 0)
      throw Error("Migration failed. Start the configured local MySQL first.");
    for (const s of services) {
      if (await healthy(s)) {
        console.log(s.id + " already reachable");
        continue;
      }
      if (!(await available(s.port)))
        throw Error(
          "Port " +
            s.port +
            " is occupied by another or unhealthy service; inspect it before restarting.",
        );
      const fd = fs.openSync(path.join(tmp, s.id + ".log"), "a");
      const p = cp.spawn(
        process.execPath,
        [path.join(root, "scripts/dev.cjs"), s.id],
        {
          cwd: root,
          stdio: ["ignore", fd, fd],
          detached: true,
          windowsHide: true,
        },
      );
      await new Promise((resolve, reject) => {
        p.once("spawn", resolve);
        p.once("error", reject);
      });
      fs.closeSync(fd);
      p.unref();
      state[s.id] = { pid: p.pid, started_at: new Date().toISOString() };
      fs.writeFileSync(
        path.join(tmp, "services.json"),
        JSON.stringify(state, null, 2),
      );
    }
  }
  let ready = true;
  for (const s of services) {
    let ok = await healthy(s);
    if (command === "start") {
      for (let attempt = 0; !ok && attempt < 20; attempt++) {
        await new Promise((r) => setTimeout(r, 500));
        ok = await healthy(s);
      }
    }
    console.log((ok ? "READY " : "DOWN  ") + s.id + " " + s.url);
    if (!ok) ready = false;
  }
  if (!ready) process.exitCode = 1;
})().catch((e) => {
  console.error(e.message);
  process.exitCode = 1;
});

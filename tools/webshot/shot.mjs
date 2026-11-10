// Screenshots cohub.live in the states of compare/states.json, signed in
// with the cohub CLI's account through a host bridge shim (the token stays
// in memory). Custom Space themes are off (?theme=safe), as the app's default.
//
//   node tools/webshot/shot.mjs [name-filter]
import { chromium } from "playwright";
import { execFileSync } from "node:child_process";
import { readFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";

const root = new URL("../../compare/", import.meta.url).pathname;
const states = JSON.parse(readFileSync(join(root, "states.json"), "utf8"));
const vars = JSON.parse(readFileSync(join(root, "vars.json"), "utf8"));
const only = process.argv[2] ? new RegExp(process.argv[2]) : null;
const fill = (s) => s.replace(/\{(\w+)\}/g, (_, k) => vars[k] ?? `{${k}}`);
const token = execFileSync("cohub", ["auth", "token"], { encoding: "utf8" }).trim();
const who = JSON.parse(execFileSync("cohub", ["auth", "whoami", "--json"], { encoding: "utf8" }));
mkdirSync(join(root, "web"), { recursive: true });

const browser = await chromium.launch({ executablePath: process.env.CHROME ?? "/usr/bin/google-chrome", headless: true });
for (const s of states) {
  if (only && !only.test(s.name)) continue;
  const [w, h] = s.size ?? [1440, 900];
  const ctx = await browser.newContext({ viewport: { width: w, height: h }, colorScheme: s.dark ? "dark" : "light", locale: "zh-CN", timezoneId: "Asia/Shanghai" });
  await ctx.addInitScript(({ token, user, dark }) => {
    try { localStorage.setItem("cohub-theme", dark ? "dark" : "light"); } catch {}
    const ls = new Set();
    const send = (m) => setTimeout(() => ls.forEach((l) => l({ data: JSON.stringify(m) })), 0);
    const handle = (r) => {
      const ok = (result) => send({ type: "response", id: r.id, result });
      switch (r.method) {
        case "host.describe": return ok({ version: 1, platform: "linux", hostId: "webshot", capabilities: ["auth.token", "auth.session"] });
        case "auth.getAccessToken": return ok(token);
        case "auth.getSessionVersion": return ok(1);
        case "auth.getSession": return ok({ authenticated: true, userUuid: user.uuid ?? null, subject: null, email: null });
      }
      send({ type: "response", id: r.id, error: { code: "unsupported", message: "unsupported" } });
    };
    Object.defineProperty(window, "cohubHost", { value: Object.freeze({
      postMessage: (m) => handle(JSON.parse(m)),
      addEventListener: (t, l) => t === "message" && ls.add(l),
      removeEventListener: (t, l) => ls.delete(l),
    }) });
  }, { token, user: who.user ?? {}, dark: !!s.dark });
  const page = await ctx.newPage();
  const path = fill(s.path);
  const safe = s.custom ? "" : (path.includes("?") ? "&" : "?") + "theme=safe";
  await page.goto("https://cohub.live" + path + safe, { waitUntil: "networkidle", timeout: 60000 }).catch(() => {});
  await page.waitForTimeout(2500);
  for (const a of s.actions ?? []) {
    try {
      if (a.click) await page.getByText(fill(a.click), { exact: true }).first().click({ timeout: 5000 });
      if (a.clickLabel) await page.getByLabel(fill(a.clickLabel), { exact: true }).first().click({ timeout: 5000 });
      if (a.textPart) await page.getByText(fill(a.textPart)).first().click({ timeout: 5000 });
      if (a.css) await page.locator(a.css).nth(a.nth ?? 0).click({ timeout: 5000 });
      if (a.scroll) { await page.mouse.move(w / 2, h / 2); await page.mouse.wheel(0, a.scroll); }
      if (a.hover) await page.getByText(fill(a.hover), { exact: true }).first().hover({ timeout: 5000 });
      if (a.key) await page.keyboard.press(a.key);
      if (a.type) await page.keyboard.type(a.type);
    } catch (e) { console.error(s.name, JSON.stringify(a), e.message.split("\n")[0]); }
    await page.waitForTimeout(a.wait ?? 700);
  }
  await page.mouse.move(w - 1, h - 1);
  await page.waitForTimeout(400);
  await page.screenshot({ path: join(root, "web", s.name + ".png") });
  console.log(s.name, page.url().replace("https://cohub.live", ""));
  await ctx.close();
}
await browser.close();

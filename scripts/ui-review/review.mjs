// Revisión de la interfaz en un navegador real (Chromium, Chrome o Edge).
// Recorre cliente, operador y personal en escritorio y móvil; corre axe
// (WCAG 2.0/2.1 A y AA), mide desborde horizontal, junta errores de consola y
// registra el orden de foco con Tab. Las credenciales se leen del entorno y
// nunca se escriben en el informe. Sale con código 1 si encuentra problemas.
//
//   BASE_URL=https://localhost BROWSER="<ruta al navegador>" \
//   ADMIN_TOKEN=... CLIENT_TOKEN=... [CLIENT_ID=cli-001] [OUT_DIR=...] \
//   node scripts/ui-review/review.mjs
import { chromium } from "playwright-core";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const base = (process.env.BASE_URL || "https://localhost").replace(/\/$/, "");
const out = process.env.OUT_DIR || path.join(here, "../../output/ui-review");
const { BROWSER: exe, ADMIN_TOKEN: admin, CLIENT_TOKEN: client } = process.env;
const clientID = process.env.CLIENT_ID || "cli-001";
if (!exe || !admin || !client) {
  console.error("Definir BROWSER, ADMIN_TOKEN y CLIENT_TOKEN.");
  process.exit(2);
}
const axeSource = fs.readFileSync(path.join(here, "node_modules/axe-core/axe.min.js"), "utf8");
fs.mkdirSync(out, { recursive: true });

async function api(route, token) {
  const response = await fetch(base + route, { headers: { Authorization: `Bearer ${token}` } });
  return response.ok ? response.json() : null;
}
const employees = await api("/v1/admin/employees?page=1&pageSize=1", admin);
const clinic = await api("/v1/employees/reports/clinic?page=1&pageSize=100", admin);
const employeeID = employees?.items?.[0]?.employeeId;
const requestID = clinic?.items?.find((i) => i.request)?.request.requestId;

const viewports = {
  desktop: { viewport: { width: 1366, height: 900 } },
  mobile: { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true },
};
const report = { base, at: new Date().toISOString(), screens: [], keyboard: {}, consoleErrors: {} };

async function settle(page) {
  await page.waitForLoadState("networkidle").catch(() => {});
  await page.waitForFunction(() => !document.querySelector(".loading"), null, { timeout: 8000 }).catch(() => {});
  await page.waitForTimeout(250);
}
async function go(page, route) {
  // La credencial vive en memoria: se navega dentro de la SPA sin recargar.
  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate"));
  }, route);
  await settle(page);
}
async function audit(page, viewport, name) {
  await page.addScriptTag({ content: axeSource });
  const violations = await page.evaluate(async () => {
    const r = await window.axe.run(document, {
      runOnly: { type: "tag", values: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"] },
    });
    return r.violations.map((v) => ({ id: v.id, impact: v.impact, nodes: v.nodes.length, sample: v.nodes.slice(0, 3).map((n) => n.target.join(" ")) }));
  });
  const overflowPx = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  const file = `${viewport}-${name}.png`;
  await page.screenshot({ path: path.join(out, file), fullPage: true });
  report.screens.push({ viewport, name, route: new URL(page.url()).pathname, overflowPx, violations, file });
}
async function focusTrail(page, steps) {
  const trail = [];
  for (let i = 0; i < steps; i++) {
    await page.keyboard.press("Tab");
    trail.push(
      await page.evaluate(() => {
        const el = document.activeElement;
        if (!el || el === document.body) return { tag: "body" };
        const style = getComputedStyle(el);
        return {
          tag: el.tagName.toLowerCase(),
          label: (el.getAttribute("aria-label") || el.labels?.[0]?.textContent || el.textContent || "").trim().slice(0, 40),
          visibleFocus: style.outlineStyle !== "none" && style.outlineWidth !== "0px" ? "outline" : style.boxShadow !== "none" ? "box-shadow" : "none",
        };
      }),
    );
  }
  return trail;
}

const browser = await chromium.launch({ executablePath: exe, headless: true });
for (const [viewport, options] of Object.entries(viewports)) {
  // bypassCSP solo permite inyectar axe en este navegador de prueba; la política del sitio no cambia.
  const context = await browser.newContext({ ...options, ignoreHTTPSErrors: true, bypassCSP: true, locale: "es-AR" });
  const page = await context.newPage();
  const errors = [];
  page.on("console", (m) => m.type() === "error" && errors.push(m.text().slice(0, 160)));

  await page.goto(`${base}/acceso`);
  await settle(page);
  await audit(page, viewport, "cliente-acceso");
  if (viewport === "desktop") report.keyboard.clienteAcceso = await focusTrail(page, 6);
  await page.getByLabel("Identificador de cliente").fill(clientID);
  await page.getByLabel("Credencial de acceso").fill(client);
  await page.getByRole("button", { name: /Empezar mi viaje/ }).click();
  await settle(page);
  await audit(page, viewport, "cliente-catalogo");
  const car = page.getByRole("link", { name: /^Ver / }).first();
  if (await car.count()) {
    await car.click();
    await settle(page);
    await audit(page, viewport, "cliente-ficha");
  }
  await go(page, "/reservas");
  await audit(page, viewport, "cliente-reservas");

  await go(page, "/operador/acceso");
  await audit(page, viewport, "operador-acceso");
  if (viewport === "desktop") report.keyboard.operadorAcceso = await focusTrail(page, 4);
  await page.getByLabel("Credencial del operador").fill(admin);
  await page.getByRole("button", { name: /Entrar al panel/ }).click();
  await settle(page);
  await audit(page, viewport, "operador-resumen");
  if (viewport === "desktop") report.keyboard.operadorResumen = await focusTrail(page, 10);
  const routes = [
    ["operador-flota", "/operador/flota"],
    ["operador-clientes", "/operador/clientes"],
    ["operador-reservas", "/operador/reservas"],
    ["operador-mantenimiento", "/operador/mantenimiento"],
    ["personal-lista", "/operador/personal"],
    ["personal-alta", "/operador/personal/nuevo"],
    ["gimnasio", "/operador/gimnasio"],
    ["gimnasio-evidencia", "/operador/gimnasio?period=2026-11&empleado=employee-001"],
    ["hospital", "/operador/hospital"],
  ];
  if (employeeID) routes.push(["personal-ficha", `/operador/personal/${encodeURIComponent(employeeID)}`]);
  if (requestID) routes.push(["hospital-turno-detalle", `/operador/hospital/${encodeURIComponent(requestID)}`]);
  for (const [name, route] of routes) {
    await go(page, route);
    await audit(page, viewport, name);
  }
  if (viewport === "mobile") {
    await go(page, "/operador/personal");
    await page.getByRole("button", { name: "Abrir menú del operador" }).click();
    await page.waitForTimeout(300);
    await audit(page, viewport, "operador-menu-abierto");
  }
  report.consoleErrors[viewport] = errors;
  await context.close();
}
await browser.close();

fs.writeFileSync(path.join(out, "report.json"), JSON.stringify(report, null, 2));
const failing = report.screens.filter((s) => s.violations.length || s.overflowPx > 0);
for (const s of report.screens)
  console.log(`${s.viewport.padEnd(7)} ${s.name.padEnd(24)} desborde=${s.overflowPx}px axe=${s.violations.map((v) => `${v.id}(${v.nodes})`).join(",") || "ok"}`);
const consoleErrors = Object.values(report.consoleErrors).flat();
const unfocused = Object.values(report.keyboard).flat().filter((f) => f.tag !== "body" && f.visibleFocus === "none");
console.log(`${report.screens.length} pantallas · ${failing.length} con problemas · ${consoleErrors.length} errores de consola · ${unfocused.length} focos invisibles · informe en ${out}`);
process.exit(failing.length || consoleErrors.length || unfocused.length ? 1 : 0);

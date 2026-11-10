// Captura el tablero de Grafana para la evidencia de observabilidad.
//   BROWSER=... GRAFANA_PASSWORD=... [GRAFANA_URL=http://127.0.0.1:3000] OUT=archivo.png node grafana-shot.mjs
import { chromium } from "playwright-core";
const base = process.env.GRAFANA_URL || "http://127.0.0.1:3000";
const browser = await chromium.launch({ executablePath: process.env.BROWSER, headless: true });
const page = await browser.newPage({ viewport: { width: 1600, height: 1250 } });
await page.goto(`${base}/login`);
await page.locator("input[name=user]").fill("admin");
await page.locator("input[name=password]").fill(process.env.GRAFANA_PASSWORD);
await page.getByRole("button", { name: /log in/i }).click();
await page.waitForURL((u) => !u.pathname.startsWith("/login"));
await page.goto(`${base}/d/car-cruds?orgId=1&from=now-10m&to=now&kiosk`);
await page.waitForTimeout(6000);
await page.screenshot({ path: process.env.OUT, fullPage: true });
await browser.close();

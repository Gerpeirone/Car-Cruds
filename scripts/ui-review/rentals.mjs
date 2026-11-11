// Browser check of "Mis alquileres" and /acceso against a disposable lab with
// the demo seed loaded. Only a loopback HTTPS origin; the rental credential is
// read from the environment and never printed.
import assert from "node:assert/strict";
import { mkdirSync } from "node:fs";
import { chromium } from "playwright-core";

const base = new URL(process.env.RENTALS_TEST_URL || "https://localhost");
if (base.protocol !== "https:" || !["localhost", "127.0.0.1"].includes(base.hostname) || base.pathname !== "/")
  throw new Error("RENTALS_TEST_URL debe ser un origen HTTPS de laboratorio en loopback.");
for (const key of ["BROWSER", "AUTH_CLIENT_TOKEN"])
  if (!process.env[key]) throw new Error(`Definir ${key}.`);
const customer = process.env.AUTH_CLIENT_CUSTOMER_ID || "cli-001";
const shots = process.env.SCREENSHOTS_DIR;
if (shots) mkdirSync(shots, { recursive: true });
const shot = async (page, name) => shots && page.screenshot({ path: `${shots}/${name}.png`, fullPage: true });

const browser = await chromium.launch({ executablePath: process.env.BROWSER, headless: true });
try {
  for (const viewport of [{ width: 1366, height: 900 }, { width: 390, height: 844 }]) {
    const context = await browser.newContext({ baseURL: base.origin, ignoreHTTPSErrors: true, viewport });
    const page = await context.newPage();
    const suffix = viewport.width < 600 ? "-movil" : "";
    await page.goto(new URL("/acceso", base).href);
    await page.getByRole("heading", { name: "¿Todavía no tenés una cuenta?" }).waitFor();
    await shot(page, `acceso${suffix}`);
    await page.getByRole("link", { name: /Crear cuenta gratis/ }).click();
    await page.getByRole("heading", { name: "Creá tu cuenta" }).waitFor();
    await page.goto(new URL("/acceso", base).href);
    await page.getByRole("link", { name: /Explorar catálogo/ }).click();
    await page.waitForURL(/\/(iniciar-sesion|catalogo)/);

    await page.goto(new URL("/mis-alquileres", base).href);
    await page.getByLabel("Identificador de cliente").fill(customer);
    await page.getByLabel("Credencial de acceso").fill(process.env.AUTH_CLIENT_TOKEN);
    await page.getByRole("button", { name: /Empezar mi viaje/ }).click();
    await page.getByRole("heading", { name: "Mis alquileres" }).waitFor();
    const cards = page.locator("article.rental-card");
    await cards.first().waitFor();
    await page.getByRole("heading", { name: "Civic" }).waitFor();
    const ids = await page.locator(".rental-facts dd.break-word").allTextContents();
    assert.ok(ids.length >= 2, "faltan reservas de la credencial");
    // Every card belongs to the authenticated driver: check each id on the API.
    const token = process.env.AUTH_CLIENT_TOKEN;
    for (const id of ids) {
      const response = await page.request.get(`/v1/reservations/${id}`, { headers: { Authorization: `Bearer ${token}` } });
      assert.equal(response.status(), 200);
      assert.equal((await response.json()).customerId, customer);
    }
    const other = await page.request.get("/v1/reservations?customerId=demo-cli-reserva-futura", { headers: { Authorization: `Bearer ${token}` } });
    assert.equal(other.status(), 403, "otro conductor visible");
    const active = cards.filter({ hasText: "Renegade" }).filter({ hasText: "En curso" }).first();
    await active.waitFor();
    assert.equal(await active.getByRole("button", { name: "Cancelar reserva" }).count(), 0);
    await shot(page, `mis-alquileres${suffix}`);

    if (!suffix) {
      const future = cards.filter({ hasText: "Civic" }).filter({ hasText: "Reservada" }).first();
      await future.getByRole("button", { name: "Cancelar reserva" }).click();
      const dialog = page.getByRole("dialog");
      await dialog.getByText("No se puede deshacer", { exact: false }).waitFor();
      await shot(page, "cancelar-confirmacion");
      await dialog.getByRole("button", { name: "Sí, cancelar" }).click();
      await page.getByText(/Cancelamos la reserva/).waitFor();
      await cards.filter({ hasText: "Civic" }).filter({ hasText: "Cancelada" }).first().waitFor();
      await shot(page, "cancelada");
      const cancelledId = (await cards.filter({ hasText: "Civic" }).filter({ hasText: "Cancelada" }).first().locator(".rental-facts dd.break-word").textContent()).trim();
      const again = await page.request.delete(`/v1/reservations/${cancelledId}`, { headers: { Authorization: `Bearer ${token}` } });
      assert.equal(again.status(), 200, "la cancelación repetida debe ser idempotente");
      assert.equal((await again.json()).status, "CANCELLED");

      await page.getByRole("link", { name: "Explorar autos" }).click();
      await page.getByLabel(/Buscar/).first().fill("Kangoo");
      await page.keyboard.press("Enter");
      await page.getByRole("link", { name: /Kangoo/ }).first().waitFor();
      await shot(page, "busqueda-demo");
    }
    await context.close();
  }
  console.log("PASS: /acceso, Mis alquileres (escritorio y móvil), propiedad, cancelación con confirmación, repetición idempotente y búsqueda");
} finally {
  await browser.close();
}

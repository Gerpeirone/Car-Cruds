// Browser integration with the real gateway and a disposable local database.
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { chromium, request } from "playwright-core";

const base = new URL(process.env.ACCOUNT_TEST_URL || "https://localhost");
if (base.protocol !== "https:" || !["localhost", "127.0.0.1"].includes(base.hostname) ||
    base.username || base.password || base.search || base.hash || base.pathname !== "/") {
  throw new Error("ACCOUNT_TEST_URL debe ser un origen HTTPS de laboratorio en loopback.");
}
if (!process.env.BROWSER) throw new Error("Definir BROWSER con la ruta al navegador.");
const browser = await chromium.launch({ executablePath: process.env.BROWSER, headless: true });
const context = await browser.newContext({ baseURL: base.origin, ignoreHTTPSErrors: true });
const email = `cuenta-${randomUUID()}@example.test`;
const password = `Frase ficticia #${randomUUID()}`;
try {
  const page = await context.newPage();
  await page.goto(base.href);
  await page.getByRole("navigation", { name: "Cuenta" }).getByRole("link", { name: "Crear cuenta" }).click();
  await page.getByLabel("Nombre", { exact: true }).fill("Ana");
  await page.getByLabel("Apellido", { exact: true }).fill("Laboratorio");
  await page.getByLabel("Correo electrónico").fill(email);
  await page.getByLabel("Contraseña", { exact: true }).fill(password);
  await page.getByLabel("Confirmar contraseña").fill(password);
  await page.getByRole("button", { name: "Crear cuenta" }).click();
  await page.getByRole("heading", { name: "Bienvenido de nuevo" }).waitFor();
  await page.getByLabel("Correo electrónico").fill(email);
  await page.getByLabel("Contraseña", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL("**/catalogo");
  await page.reload();
  await page.getByRole("link", { name: /^Ver / }).first().click();
  await page.waitForURL("**/catalogo/*");
  await page.getByText("Para reservar necesitás una credencial de alquiler.").waitFor();
  await page.getByRole("link", { name: "Mi perfil" }).click();
  await page.getByText(email, { exact: true }).waitFor();

  assert.equal((await page.request.get("/v1/admin/employees")).status(), 401);
  assert.equal((await page.request.get("/v1/vehicles/auto-001/quote")).status(), 401);
  assert.equal((await page.request.post("/v1/employees/membership-evidence", { data: {} })).status(), 401);
  const cookie = (await context.cookies()).find((value) => value.name === "__Host-carcruds-account");
  assert.ok(cookie?.httpOnly && cookie.secure && cookie.sameSite === "Strict");
  await page.getByRole("button", { name: "Cerrar sesión" }).click();
  await page.waitForURL(base.href);
  assert.equal((await page.request.get("/v1/auth/me")).status(), 401);
  const replay = await request.newContext({ baseURL: base.origin, ignoreHTTPSErrors: true });
  try {
    assert.equal((await replay.get("/v1/auth/me", { headers: { Cookie: `${cookie.name}=${cookie.value}` } })).status(), 401);
  } finally {
    await replay.dispose();
  }
  console.log("PASS: registro, login, recarga, ficha, perfil, aislamiento y revocación de cookie por HTTPS local");
} finally {
  await browser.close();
}

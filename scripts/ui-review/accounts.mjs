// Browser integration with the real Caddy, gateway and a disposable local database.
// Path: create account -> log in -> catalog -> vehicle detail -> back to the
// catalog -> profile -> reload -> new tab -> log out, plus credential and
// permission checks. Only a loopback HTTPS origin and a fictitious user.
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
const password = `  Frase ficticia #${randomUUID()}  `;
const cookieName = "__Host-carcruds-account";
const steps = [];
const step = (name) => steps.push(name);
// Every API response the page receives: no 401 may appear while the session lives.
const unauthorized = [];
context.on("response", (response) => {
  const url = new URL(response.url());
  if (url.pathname.startsWith("/v1/") && url.pathname !== "/v1/auth/me" && url.pathname !== "/v1/auth/login" && response.status() === 401)
    unauthorized.push(`${response.request().method()} ${url.pathname}`);
});
const loginForm = (page) => page.getByRole("heading", { name: "Bienvenido de nuevo" });
async function expectSignedIn(page) {
  const nav = page.getByRole("navigation", { name: "Cuenta" });
  await nav.getByRole("button", { name: "Cerrar sesión" }).waitFor();
  assert.equal(await loginForm(page).count(), 0, `se pidió iniciar sesión en ${page.url()}`);
}

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
  await loginForm(page).waitFor();
  assert.equal(await page.getByLabel("Correo electrónico").inputValue(), email, "correo no precargado");
  await page.getByText("no creamos otra ni cambiamos su contraseña").waitFor();
  step("registro");

  // A second registration with another password must not replace the first.
  const api = await request.newContext({ baseURL: base.origin, ignoreHTTPSErrors: true });
  const other = "Otra frase ficticia distinta";
  const repeated = await api.post("/v1/auth/register", { data: { firstName: "Otra", lastName: "Persona", email: email.toUpperCase(), password: other, passwordConfirmation: other } });
  assert.equal(repeated.status(), 202);
  assert.equal((await api.post("/v1/auth/login", { data: { email, password: other } })).status(), 401);
  assert.equal((await api.post("/v1/auth/login", { data: { email, password: password.trim() } })).status(), 401, "la contraseña se recortó");
  await api.dispose();
  step("duplicado sin cambios y contraseña exacta");

  await page.getByLabel("Contraseña", { exact: true }).fill("contraseña incorrecta de prueba");
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.getByText("El correo o la contraseña no son correctos.").waitFor();
  step("contraseña incorrecta");

  await page.getByLabel("Correo electrónico").fill(`  ${email.toUpperCase()}  `);
  await page.getByLabel("Contraseña", { exact: true }).fill(password);
  await page.getByRole("button", { name: "Iniciar sesión" }).click();
  await page.waitForURL("**/catalogo");
  await expectSignedIn(page);
  const cookie = (await context.cookies()).find((value) => value.name === cookieName);
  assert.ok(cookie?.httpOnly && cookie.secure && cookie.sameSite === "Strict" && cookie.path === "/" && !cookie.domain.startsWith("."));
  assert.ok(cookie.expires * 1000 > Date.now() + 60 * 60 * 1000, "la cookie no conserva la vigencia de la sesión");
  assert.equal(await page.evaluate(() => document.cookie.includes("carcruds-account")), false, "cookie legible desde JavaScript");
  assert.equal(await page.evaluate(() => JSON.stringify({ ...localStorage }) + JSON.stringify({ ...sessionStorage })).then((s) => s.includes(cookie.value)), false);
  step("login con correo normalizado y cookie segura");

  // Several catalog/detail round trips exercise both Flota replicas behind NGINX.
  for (let i = 0; i < 6; i++) {
    await page.getByRole("link", { name: /^Ver / }).first().click();
    await page.waitForURL("**/catalogo/*");
    await page.getByText("Para reservar necesitás una credencial de alquiler.").waitFor();
    await expectSignedIn(page);
    await page.getByRole("link", { name: "Volver a la flota" }).click();
    await page.waitForURL("**/catalogo");
    await page.getByRole("link", { name: /^Ver / }).first().waitFor();
    await expectSignedIn(page);
  }
  step("catálogo → ficha → catálogo (x6)");

  await page.getByRole("link", { name: "Mi perfil" }).click();
  await page.getByText(email, { exact: true }).waitFor();
  await page.reload();
  await page.getByText(email, { exact: true }).waitFor();
  await page.getByRole("navigation", { name: "Cuenta" }).getByRole("link", { name: "Catálogo", exact: true }).click();
  await page.getByRole("link", { name: /^Ver / }).first().click();
  await page.waitForURL("**/catalogo/*");
  const detailUrl = page.url();
  await page.reload();
  await page.getByText("Para reservar necesitás una credencial de alquiler.").waitFor();
  await expectSignedIn(page);
  await page.goto(base.href);
  await page.getByRole("link", { name: "Explorar el catálogo" }).waitFor();
  step("perfil, recarga de perfil y de ficha, portada");

  const tab = await context.newPage();
  await tab.goto(detailUrl);
  await tab.getByText("Para reservar necesitás una credencial de alquiler.").waitFor();
  await expectSignedIn(tab);
  await tab.close();
  step("nueva pestaña");

  assert.deepEqual(unauthorized, [], "respuestas 401 durante la sesión");
  for (const [method, path] of [["GET", "/v1/admin/employees"], ["GET", "/v1/employees/clinic/doctors"], ["GET", "/v1/vehicles/auto-001/quote"], ["GET", "/v1/customers/cli-001"], ["POST", "/v1/employees/membership-evidence"], ["POST", "/v1/reservations"]]) {
    const response = method === "GET" ? await page.request.get(path) : await page.request.post(path, { data: {} });
    assert.equal(response.status(), 401, `${method} ${path} con la cookie de cliente`);
  }
  step("rutas administrativas bloqueadas");

  await page.getByRole("navigation", { name: "Cuenta" }).getByRole("button", { name: "Cerrar sesión" }).click();
  await page.waitForURL(base.href);
  await page.getByRole("heading", { name: "Encontrá tu próximo auto" }).waitFor();
  assert.equal((await page.request.get("/v1/auth/me")).status(), 401);
  await page.goto(new URL("/catalogo", base).href);
  await loginForm(page).waitFor();
  const replay = await request.newContext({ baseURL: base.origin, ignoreHTTPSErrors: true });
  try {
    const headers = { Cookie: `${cookie.name}=${cookie.value}` };
    assert.equal((await replay.get("/v1/auth/me", { headers })).status(), 401);
    assert.equal((await replay.get("/v1/vehicles", { headers })).status(), 401);
  } finally {
    await replay.dispose();
  }
  step("cierre de sesión y cookie revocada");
  console.log(`PASS: ${steps.join(" · ")}`);
} finally {
  await browser.close();
}

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";
import App from "./App";

const account = {
  id: "acc-fixture-1",
  firstName: "Ana",
  lastName: "Demo",
  email: "ana@example.test",
  role: "client",
};
const vehicle = {
  vehicleId: "auto-001",
  brand: "Toyota",
  model: "Corolla",
  category: "sedan",
  transmission: "automatic",
  seats: 5,
  dailyRateMinor: 5000000,
  currency: "ARS",
  active: true,
  version: 1,
};

let session: "valid" | "revoked";
let detail: "ok" | "unauthorized";
let calls: { path: string; init: RequestInit }[];
beforeEach(() => {
  session = "valid";
  detail = "ok";
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init: RequestInit = {}) => {
      calls.push({ path, init });
      const unauthorized = () =>
        Response.json({ code: "UNAUTHORIZED" }, { status: 401 });
      if (path === "/v1/auth/me")
        return session === "valid" ? Response.json(account) : unauthorized();
      if (path === "/v1/auth/logout") {
        session = "revoked";
        return new Response(null, { status: 204 });
      }
      if (session !== "valid") return unauthorized();
      if (
        path.startsWith("/v1/vehicles/search?") ||
        path.startsWith("/v1/vehicles?")
      )
        return Response.json({
          items: [vehicle],
          page: 1,
          pageSize: 6,
          total: 1,
        });
      if (path === "/v1/vehicles/auto-001")
        return detail === "ok" ? Response.json(vehicle) : unauthorized();
      return Response.json({ code: "NOT_FOUND" }, { status: 404 });
    }),
  );
});

function mount(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  );
}
const accountNav = () => screen.getByRole("navigation", { name: "Cuenta" });

it("mantiene la sesión en catálogo, ficha, regreso, perfil y recarga", async () => {
  const user = userEvent.setup();
  const view = mount("/catalogo");
  await user.click(
    (await screen.findAllByRole("link", { name: /Corolla/ }))[0],
  );
  await screen.findByText(
    "Para reservar necesitás una credencial de alquiler.",
  );
  expect(within(accountNav()).getByText("Ana")).toBeTruthy();
  await user.click(screen.getByRole("link", { name: "Volver a la flota" }));
  await screen.findAllByRole("link", { name: /Corolla/ });
  await user.click(
    within(accountNav()).getByRole("link", { name: "Mi perfil" }),
  );
  await screen.findByText("ana@example.test");
  // A full reload starts from an empty React state and asks /v1/auth/me again.
  view.unmount();
  mount("/catalogo/auto-001");
  await screen.findByText(
    "Para reservar necesitás una credencial de alquiler.",
  );
  expect(
    screen.queryByRole("heading", { name: "Bienvenido de nuevo" }),
  ).toBeNull();
  for (const { path, init } of calls) {
    expect(init.credentials, path).toBe("same-origin");
    expect(new Headers(init.headers).has("Authorization"), path).toBe(false);
  }
});

it("no cierra la sesión si una lectura falla con 401 pero la sesión sigue vigente", async () => {
  detail = "unauthorized";
  const user = userEvent.setup();
  mount("/catalogo");
  await user.click(
    (await screen.findAllByRole("link", { name: /Corolla/ }))[0],
  );
  expect((await screen.findByRole("alert")).textContent).toMatch(
    /no está disponible/,
  );
  expect(
    screen.queryByRole("heading", { name: "Bienvenido de nuevo" }),
  ).toBeNull();
  expect(
    within(accountNav()).getByRole("button", { name: "Cerrar sesión" }),
  ).toBeTruthy();
  expect(calls.filter((c) => c.path === "/v1/auth/me").length).toBe(2);
});

it("vuelve al inicio de sesión solo cuando /v1/auth/me confirma que la sesión terminó", async () => {
  const user = userEvent.setup();
  mount("/catalogo");
  await screen.findAllByRole("link", { name: /Corolla/ });
  session = "revoked";
  await user.click(
    (await screen.findAllByRole("link", { name: /Corolla/ }))[0],
  );
  await screen.findByRole("heading", { name: "Bienvenido de nuevo" });
});

it("cierra la sesión y no vuelve a entrar con la cookie revocada", async () => {
  const user = userEvent.setup();
  const view = mount("/perfil");
  await screen.findByText("ana@example.test");
  await user.click(
    within(accountNav()).getByRole("button", { name: "Cerrar sesión" }),
  );
  await screen.findByRole("heading", { name: "Encontrá tu próximo auto" });
  view.unmount();
  mount("/catalogo");
  await screen.findByRole("heading", { name: "Bienvenido de nuevo" });
});

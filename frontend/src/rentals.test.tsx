import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";
import App from "./App";

const token = "fixture-client-not-a-real-secret";
const customer = {
  id: "cli-001",
  name: "Conductor de prueba",
  active: true,
  licenseValidUntil: "2099-12-31",
};
const vehicle = {
  vehicleId: "demo-toyota-corolla",
  brand: "Toyota",
  model: "Corolla",
  category: "sedan",
  transmission: "automatic",
  seats: 5,
  dailyRateMinor: 5200000,
  currency: "ARS",
  active: true,
  version: 1,
};
const reservation = (id: string, status: string) => ({
  id,
  status,
  vehicleId: vehicle.vehicleId,
  customerId: "cli-001",
  startsOn: "2030-02-01",
  endsOn: "2030-02-04",
  days: 3,
  dailyRateMinor: 5200000,
  totalMinor: 15600000,
  currency: "ARS",
});

let rows: ReturnType<typeof reservation>[];
let cancelFails: boolean;
let calls: { path: string; init: RequestInit }[];
beforeEach(() => {
  rows = [
    reservation("res-futura", "RESERVED"),
    reservation("res-activa", "ACTIVE"),
  ];
  cancelFails = false;
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init: RequestInit = {}) => {
      calls.push({ path, init });
      if (path === "/v1/customers/cli-001") return Response.json(customer);
      if (path.startsWith("/v1/reservations?"))
        return Response.json({
          items: rows,
          page: 1,
          pageSize: 12,
          total: rows.length,
        });
      if (path === "/v1/reservations/res-futura" && init.method === "DELETE") {
        if (cancelFails) throw new TypeError("respuesta perdida");
        rows = rows.map((r) =>
          r.id === "res-futura" ? { ...r, status: "CANCELLED" } : r,
        );
        return Response.json(rows[0]);
      }
      if (path === `/v1/vehicles/${vehicle.vehicleId}`)
        return Response.json(vehicle);
      if (path.startsWith("/v1/vehicles"))
        return Response.json({
          items: [vehicle],
          page: 1,
          pageSize: 6,
          total: 1,
        });
      if (path === "/v1/auth/me")
        return Response.json({ code: "UNAUTHORIZED" }, { status: 401 });
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
async function login(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Identificador de cliente"), "cli-001");
  await user.type(screen.getByLabelText("Credencial de acceso"), token);
  await user.click(screen.getByRole("button", { name: /Empezar mi viaje/ }));
}
const card = (id: string) =>
  screen.getByText(id).closest("article") as HTMLElement;

it("lista las reservas propias con datos reales y sin enviar un customerId", async () => {
  const user = userEvent.setup();
  mount("/mis-alquileres");
  await login(user);
  await screen.findByRole("heading", { name: "Mis alquileres" });
  await screen.findAllByRole("heading", { name: "Corolla" });
  const future = card("res-futura");
  expect(within(future).getByText("Reservada")).toBeTruthy();
  expect(within(future).getByText(vehicle.vehicleId)).toBeTruthy();
  expect(within(future).getByText(/156\.000/)).toBeTruthy();
  expect(
    within(future).getByRole("button", { name: "Cancelar reserva" }),
  ).toBeTruthy();
  // An active rental is returned by the operator; it offers no cancellation.
  const active = card("res-activa");
  expect(
    within(active).queryByRole("button", { name: "Cancelar reserva" }),
  ).toBeNull();
  expect(
    within(active).getByText(/la devolución la registra el operador/),
  ).toBeTruthy();
  const list = calls.find((c) => c.path.startsWith("/v1/reservations?"))!;
  expect(list.path).not.toMatch(/customerId/);
  expect(new Headers(list.init.headers).get("Authorization")).toBe(
    `Bearer ${token}`,
  );
});

it("cancela solo tras confirmar, informa consecuencias y refresca la lista", async () => {
  const user = userEvent.setup();
  mount("/mis-alquileres");
  await login(user);
  await screen.findAllByRole("heading", { name: "Corolla" });
  await user.click(
    within(card("res-futura")).getByRole("button", {
      name: "Cancelar reserva",
    }),
  );
  const dialog = await screen.findByRole("dialog", { hidden: true });
  expect(dialog.textContent).toMatch(/No se puede deshacer/);
  expect(calls.some((c) => c.init.method === "DELETE")).toBe(false);
  await user.click(
    within(dialog).getByRole("button", {
      name: "Mantener reserva",
      hidden: true,
    }),
  );
  expect(calls.some((c) => c.init.method === "DELETE")).toBe(false);
  await user.click(
    within(card("res-futura")).getByRole("button", {
      name: "Cancelar reserva",
    }),
  );
  await user.click(
    within(await screen.findByRole("dialog", { hidden: true })).getByRole(
      "button",
      { name: "Sí, cancelar", hidden: true },
    ),
  );
  await screen.findByText("Cancelamos la reserva res-futura.");
  expect(await within(card("res-futura")).findByText("Cancelada")).toBeTruthy();
  expect(
    within(card("res-futura")).queryByRole("button", {
      name: "Cancelar reserva",
    }),
  ).toBeNull();
  expect(calls.filter((c) => c.init.method === "DELETE").length).toBe(1);
  expect(
    calls.filter((c) => c.path.startsWith("/v1/reservations?")).length,
  ).toBeGreaterThanOrEqual(2);
});

it("ante un resultado incierto no permite reintentar hasta actualizar", async () => {
  cancelFails = true;
  const user = userEvent.setup();
  mount("/mis-alquileres");
  await login(user);
  await screen.findAllByRole("heading", { name: "Corolla" });
  await user.click(
    within(card("res-futura")).getByRole("button", {
      name: "Cancelar reserva",
    }),
  );
  await user.click(
    within(await screen.findByRole("dialog", { hidden: true })).getByRole(
      "button",
      { name: "Sí, cancelar", hidden: true },
    ),
  );
  await screen.findByText(/Revisá el estado antes de intentar de nuevo/);
  expect(
    within(card("res-futura")).queryByRole("button", {
      name: "Cancelar reserva",
    }),
  ).toBeNull();
  expect(
    within(card("res-futura")).getByText(
      "Actualizá el estado antes de volver a intentar.",
    ),
  ).toBeTruthy();
});

it("muestra el estado vacío con acceso al catálogo", async () => {
  rows = [];
  const user = userEvent.setup();
  mount("/mis-alquileres");
  await login(user);
  await screen.findByText("Todavía no tenés autos alquilados.");
  expect(
    screen.getByText(
      "Explorá nuestro catálogo y encontrá tu próximo vehículo.",
    ),
  ).toBeTruthy();
  await user.click(screen.getByRole("link", { name: /Explorar catálogo/ }));
  await screen.findAllByRole("link", { name: /Corolla/ });
});

it("desde /acceso lleva al registro y al catálogo sin confundir los permisos", async () => {
  const user = userEvent.setup();
  const view = mount("/acceso");
  expect(
    screen.getByRole("heading", { name: "¿Todavía no tenés una cuenta?" }),
  ).toBeTruthy();
  expect(
    screen.getByText(/Crear una cuenta no la otorga automáticamente/),
  ).toBeTruthy();
  await user.click(screen.getByRole("link", { name: /Crear cuenta gratis/ }));
  await screen.findByRole("heading", { name: "Creá tu cuenta" });
  view.unmount();
  mount("/acceso");
  await user.click(screen.getByRole("link", { name: /Explorar catálogo/ }));
  // Without a personal session the catalog asks to log in with the account.
  await screen.findByRole("heading", { name: "Bienvenido de nuevo" });
});

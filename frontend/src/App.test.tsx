import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";
import App from "./App";
import type { Reservation } from "./domain";

const token = "fixture-client-not-a-real-secret";
const customer = {
  id: "cli-001",
  name: "Conductor de prueba",
  active: true,
  licenseValidUntil: "2099-12-31",
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
const availability = {
  vehicleId: "auto-001",
  startsOn: "2030-01-02",
  endsOn: "2030-01-05",
  available: true,
  days: 3,
  dailyRateMinor: 5000000,
  totalMinor: 15000000,
  currency: "ARS",
};
const reservation: Reservation = {
  ...availability,
  id: "res-fixture-001",
  status: "RESERVED",
  customerId: "cli-001",
};
let postCalls: { body: string; key: string }[];
let status: Reservation["status"];
let failure: "none" | "lost" | "price" = "none";
let requests: string[];
function mount(path = "/acceso") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  );
}
beforeEach(() => {
  postCalls = [];
  requests = [];
  status = "RESERVED";
  failure = "none";
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init: RequestInit = {}) => {
      requests.push(path);
      if (path === "/v1/customers/cli-001") return Response.json(customer);
      if (path === "/v1/customers/cli-other")
        return Response.json({ code: "NOT_FOUND" }, { status: 404 });
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
      if (path === "/v1/vehicles/auto-001") return Response.json(vehicle);
      if (path.startsWith("/v1/availability?"))
        return Response.json(availability);
      if (path === "/v1/reservations") {
        postCalls.push({
          body: String(init.body),
          key: (init.headers as Record<string, string>)["Idempotency-Key"],
        });
        if (failure === "lost" && postCalls.length === 1)
          throw new TypeError("respuesta perdida");
        if (failure === "price")
          return Response.json({ code: "PRICE_CHANGED" }, { status: 409 });
        return Response.json({ ...reservation, status }, { status: 201 });
      }
      if (path === "/v1/reservations/res-fixture-001") {
        if (init.method === "DELETE") status = "CANCELLED";
        return Response.json({ ...reservation, status });
      }
      return Response.json({ code: "NOT_FOUND" }, { status: 404 });
    }),
  );
});

it.each(["ACTIVE", "COMPLETED"] as const)(
  "no ofrece cancelar una reserva %s",
  async (value) => {
    status = value;
    const user = userEvent.setup();
    mount("/reservas/res-fixture-001");
    await login(user);
    await screen.findByRole("heading", { name: "res-fixture-001" });
    expect(
      screen.queryByRole("button", { name: "Cancelar reserva" }),
    ).toBeNull();
  },
);
async function login(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Identificador de cliente"), "cli-001");
  await user.type(screen.getByLabelText("Credencial de acceso"), token);
  await user.click(screen.getByRole("button", { name: /Empezar mi viaje/ }));
}
async function toCheckout(user: ReturnType<typeof userEvent.setup>) {
  await login(user);
  await user.click(
    await screen.findByRole("link", { name: "Ver Toyota Corolla" }),
  );
  await screen.findByRole("button", { name: /Consultar disponibilidad/ });
  fireEvent.change(screen.getByLabelText("Fecha de retiro"), {
    target: { value: "2030-01-02" },
  });
  fireEvent.change(screen.getByLabelText("Fecha de devolución"), {
    target: { value: "2030-01-05" },
  });
  await user.click(
    screen.getByRole("button", { name: /Consultar disponibilidad/ }),
  );
  await user.click(
    await screen.findByRole("button", { name: /Revisar mi reserva/ }),
  );
}

it("distingue una referencia de error 401 de un código de reserva", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
    Response.json({ code: "UNAUTHORIZED" }, {
      status: 401,
      headers: { "X-Request-ID": "support-fixture-001" },
    }),
  ));
  const user = userEvent.setup();
  mount();
  await login(user);
  await screen.findByRole("button", { name: /Copiar referencia de soporte/ });
  expect(screen.getByText(/Esta referencia identifica el error/)).toBeTruthy();
  expect(screen.getByRole("button", { name: /Empezar mi viaje/ })).toBeTruthy();
  expect(screen.queryByRole("heading", { name: "CÓDIGO DE RESERVA" })).toBeNull();
  expect(postCalls).toHaveLength(0);
});

it("completa acceso, disponibilidad, creación y cancelación con confirmación explícita", async () => {
  const user = userEvent.setup();
  mount();
  await toCheckout(user);
  await user.dblClick(
    screen.getByRole("button", { name: /Confirmar reserva/ }),
  );
  await screen.findByRole("heading", { name: "res-fixture-001" });
  expect(postCalls).toHaveLength(1);
  expect(JSON.parse(postCalls[0].body)).toEqual({
    vehicleId: "auto-001",
    customerId: "cli-001",
    startsOn: "2030-01-02",
    endsOn: "2030-01-05",
    expectedTotalMinor: 15000000,
  });
  await user.click(screen.getByRole("button", { name: "Cancelar reserva" }));
  expect(status).toBe("RESERVED");
  await user.click(screen.getByRole("button", { name: "Sí, cancelar" }));
  await screen.findByRole("heading", { name: "Tu reserva está cancelada." });
  expect(status).toBe("CANCELLED");
  expect(screen.queryByRole("button", { name: "Cancelar reserva" })).toBeNull();
});
it("recupera una respuesta perdida tras recarga con exactamente el mismo cuerpo y clave", async () => {
  failure = "lost";
  const user = userEvent.setup();
  const view = mount();
  await toCheckout(user);
  await user.click(screen.getByRole("button", { name: /Confirmar reserva/ }));
  await screen.findByRole("button", { name: /Recuperar resultado/ });
  expect(postCalls).toHaveLength(1);
  expect(sessionStorage.getItem("carcruds.pending.v1:cli-001")).not.toContain(
    token,
  );
  view.unmount();
  mount("/reservar");
  expect(
    screen.getByRole("heading", { name: /Tu próximo viaje/ }),
  ).toBeTruthy();
  await login(user);
  await user.click(
    await screen.findByRole("button", { name: /Recuperar resultado/ }),
  );
  await screen.findByRole("heading", { name: "res-fixture-001" });
  expect(postCalls).toHaveLength(2);
  expect(postCalls[1]).toEqual(postCalls[0]);
  expect(sessionStorage.getItem("carcruds.pending.v1:cli-001")).toBeNull();
});
it("un precio cambiado exige volver a cotizar en lugar de mostrar éxito", async () => {
  failure = "price";
  const user = userEvent.setup();
  mount();
  await toCheckout(user);
  await user.click(screen.getByRole("button", { name: /Confirmar reserva/ }));
  await screen.findByText(/La tarifa cambió/);
  expect(screen.queryByText("res-fixture-001")).toBeNull();
  expect(
    screen.queryByRole("button", { name: /Confirmar reserva/ }),
  ).toBeNull();
  expect(sessionStorage.getItem("carcruds.pending.v1:cli-001")).toBeNull();
});
it("invalida la cotización al modificar fechas", async () => {
  const user = userEvent.setup();
  mount();
  await login(user);
  await user.click(
    await screen.findByRole("link", { name: "Ver Toyota Corolla" }),
  );
  await screen.findByRole("button", { name: /Consultar disponibilidad/ });
  fireEvent.change(screen.getByLabelText("Fecha de retiro"), {
    target: { value: "2030-01-02" },
  });
  fireEvent.change(screen.getByLabelText("Fecha de devolución"), {
    target: { value: "2030-01-05" },
  });
  await user.click(
    screen.getByRole("button", { name: /Consultar disponibilidad/ }),
  );
  await screen.findByRole("button", { name: /Revisar mi reserva/ });
  fireEvent.change(screen.getByLabelText("Fecha de devolución"), {
    target: { value: "2030-01-06" },
  });
  expect(
    screen.queryByRole("button", { name: /Revisar mi reserva/ }),
  ).toBeNull();
});
it("no convierte una reserva ajena en datos visibles", async () => {
  const user = userEvent.setup();
  mount("/reservas/other-reservation");
  await login(user);
  await screen.findByText(/No encontramos ese recurso/);
  expect(screen.queryByText("res-fixture-001")).toBeNull();
  expect(screen.queryByRole("button", { name: "Cancelar reserva" })).toBeNull();
});
it("cerrar el acceso elimina perfil y token sin guardarlos en el navegador", async () => {
  const user = userEvent.setup();
  mount();
  await login(user);
  await screen.findByRole("link", { name: "Ver Toyota Corolla" });
  await user.click(screen.getByRole("button", { name: "Cerrar acceso" }));
  await screen.findByRole("button", { name: /Empezar mi viaje/ });
  expect(screen.queryByText(customer.name)).toBeNull();
  expect(JSON.stringify({ ...localStorage, ...sessionStorage })).not.toContain(
    token,
  );
});
it("no envía POST si el navegador impide guardar la recuperación", async () => {
  const user = userEvent.setup();
  mount();
  await toCheckout(user);
  vi.spyOn(Storage.prototype, "setItem").mockImplementationOnce(() => {
    throw new Error("blocked");
  });
  await user.click(screen.getByRole("button", { name: /Confirmar reserva/ }));
  await screen.findByText(/No se pudo guardar la referencia/);
  expect(postCalls).toHaveLength(0);
});
it("ofrece un catálogo explícito sin q si el motor de búsqueda falla", async () => {
  const original = globalThis.fetch;
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) =>
      path.startsWith("/v1/vehicles/search?")
        ? Promise.resolve(
            Response.json({ code: "SEARCH_UNAVAILABLE" }, { status: 503 }),
          )
        : original(path, init),
    ),
  );
  const user = userEvent.setup();
  mount("/autos?q=Corolla");
  await login(user);
  await user.click(
    await screen.findByRole("button", { name: /Ver catálogo sin búsqueda/ }),
  );
  await screen.findByRole("link", { name: "Ver Toyota Corolla" });
  expect(
    requests.some((p) => p.startsWith("/v1/vehicles?") && !p.includes("q=")),
  ).toBe(true);
});
it("descarta respuestas de una búsqueda anterior que llega tarde", async () => {
  const original = globalThis.fetch;
  let release: (r: Response) => void = () => {};
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) =>
      path.includes("q=vieja")
        ? new Promise<Response>((resolve) => {
            release = resolve;
          })
        : path.includes("q=nueva")
          ? Promise.resolve(
              Response.json({
                items: [{ ...vehicle, model: "Modelo vigente" }],
                page: 1,
                pageSize: 6,
                total: 1,
              }),
            )
          : original(path, init),
    ),
  );
  const user = userEvent.setup();
  mount();
  await login(user);
  await screen.findByRole("link", { name: "Ver Toyota Corolla" });
  const input = screen.getByLabelText("Buscar por marca o modelo");
  await user.type(input, "vieja");
  await user.click(screen.getByRole("button", { name: "Buscar" }));
  await user.clear(input);
  await user.type(input, "nueva");
  await user.click(screen.getByRole("button", { name: "Buscar" }));
  await screen.findByRole("link", { name: "Ver Toyota Modelo vigente" });
  await act(async () =>
    release(
      Response.json({ items: [vehicle], page: 1, pageSize: 6, total: 1 }),
    ),
  );
  expect(screen.queryByRole("link", { name: "Ver Toyota Corolla" })).toBeNull();
});

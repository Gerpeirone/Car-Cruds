import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";
import App from "../App";
import { parseRate } from "./shared";

const vehicle = {
  vehicleId: "auto-001",
  brand: "Toyota",
  model: "Corolla",
  category: "sedan",
  transmission: "automatic",
  seats: 5,
  dailyRateMinor: 1234500,
  currency: "ARS",
  active: true,
  version: 1,
};
const customer = {
  id: "cli-001",
  name: "Cliente de prueba",
  active: true,
  licenseValidUntil: "2099-12-31",
};
let role = "admin";
let status = "RESERVED";
let conflict = false;
let maintenanceLost = false;
let savedVehicle = { ...vehicle };
let writes: { path: string; method: string; body: unknown }[] = [];
beforeEach(() => {
  role = "admin";
  status = "RESERVED";
  conflict = false;
  maintenanceLost = false;
  savedVehicle = { ...vehicle };
  writes = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init: RequestInit = {}) => {
      if (init.method && init.method !== "GET")
        writes.push({
          path,
          method: init.method,
          body: init.body ? JSON.parse(String(init.body)) : undefined,
        });
      if (path === "/v1/session") return Response.json({ role });
      if (path.startsWith("/v1/vehicles?"))
        return Response.json({
          items: [savedVehicle],
          total: 1,
          page: 1,
          pageSize: 20,
        });
      if (path === "/v1/vehicles" && init.method === "POST") {
        savedVehicle = {
          ...vehicle,
          ...JSON.parse(String(init.body)),
          version: 1,
        };
        return Response.json(savedVehicle, { status: 201 });
      }
      if (path.startsWith("/v1/vehicles/")) {
        if (init.method === "PUT") {
          if (conflict)
            return Response.json({ code: "VERSION_CONFLICT" }, { status: 409 });
          savedVehicle = {
            ...savedVehicle,
            ...JSON.parse(String(init.body)),
            version: savedVehicle.version + 1,
          };
        }
        return Response.json(savedVehicle);
      }
      if (path === "/v1/customers" && init.method === "POST")
        return Response.json(JSON.parse(String(init.body)), { status: 201 });
      if (path === "/v1/customers") return Response.json({ items: [customer] });
      if (path.startsWith("/v1/customers/")) return Response.json(customer);
      if (path.startsWith("/v1/reservations/")) {
        if (init.method === "POST")
          status = path.endsWith("/pickup") ? "ACTIVE" : "COMPLETED";
        if (init.method === "DELETE") status = "CANCELLED";
        return Response.json({
          id: "res-001",
          vehicleId: "auto-001",
          customerId: "cli-001",
          startsOn: "2030-01-02",
          endsOn: "2030-01-05",
          days: 3,
          dailyRateMinor: 1234500,
          totalMinor: 3703500,
          currency: "ARS",
          status,
        });
      }
      if (path === "/v1/maintenance") {
        if (maintenanceLost) throw new TypeError("respuesta perdida");
        return Response.json(
          { id: "maintenance-001", ...JSON.parse(String(init.body)) },
          { status: 201 },
        );
      }
      return Response.json({ code: "NOT_FOUND" }, { status: 404 });
    }),
  );
});
function mount(path = "/operador") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  );
}
async function login(user: ReturnType<typeof userEvent.setup>) {
  await user.type(
    screen.getByLabelText("Credencial del operador"),
    "admin-fixture-not-a-real-secret",
  );
  await user.click(screen.getByRole("button", { name: /Entrar al panel/ }));
}
it.each(["client", "service", "gym"])(
  "rechaza acceso de rol %s sin abrir el panel",
  async (value) => {
    role = value;
    const user = userEvent.setup();
    mount();
    await login(user);
    await screen.findByText(/Este acceso no tiene permiso/);
    expect(
      screen.queryByRole("navigation", { name: "Navegación del operador" }),
    ).toBeNull();
    expect(writes).toHaveLength(0);
  },
);
it("acceso administrativo usa la sesión y no guarda credenciales", async () => {
  const user = userEvent.setup();
  mount();
  await login(user);
  await screen.findByRole("heading", { name: "Todo en su lugar." });
  expect(sessionStorage.length).toBe(0);
  expect(localStorage.length).toBe(0);
  await user.click(screen.getByRole("button", { name: "Cerrar acceso" }));
  await screen.findByLabelText("Credencial del operador");
});
it("convierte pesos a centavos exactamente y valida límites", () => {
  expect(parseRate("12345,67")).toBe(1234567);
  expect(parseRate("0.01")).toBe(1);
  expect(() => parseRate("1.001")).toThrow();
  expect(() => parseRate("10000000.01")).toThrow();
});
it("crea un vehículo con tarifa en centavos", async () => {
  const user = userEvent.setup();
  mount("/operador/flota/nuevo");
  await login(user);
  await screen.findByLabelText("Marca");
  await user.type(screen.getByLabelText("Marca"), "Fiat");
  await user.type(screen.getByLabelText("Modelo"), "Cronos");
  await user.type(screen.getByLabelText("Categoría"), "sedan");
  await user.type(
    screen.getByLabelText("Tarifa diaria en pesos (ARS)"),
    "54321,09",
  );
  await user.click(screen.getByRole("button", { name: /Registrar vehículo/ }));
  await screen.findByText("Vehículo registrado.");
  expect(writes[0]).toMatchObject({
    path: "/v1/vehicles",
    method: "POST",
    body: {
      brand: "Fiat",
      model: "Cronos",
      dailyRateMinor: 5432109,
      currency: "ARS",
    },
  });
});
it("espera la ficha después del alta antes de permitir editar la tarifa", async () => {
  const original = globalThis.fetch;
  let release: ((response: Response) => void) | undefined;
  vi.stubGlobal(
    "fetch",
    vi.fn((path: string, init?: RequestInit) =>
      path.startsWith("/v1/vehicles/") &&
      (!init?.method || init.method === "GET")
        ? new Promise<Response>((resolve) => {
            release = resolve;
          })
        : original(path, init),
    ),
  );
  const user = userEvent.setup();
  mount("/operador/flota/nuevo");
  await login(user);
  await user.type(await screen.findByLabelText("Marca"), "Fiat");
  await user.type(screen.getByLabelText("Modelo"), "Cronos");
  await user.type(screen.getByLabelText("Categoría"), "sedan");
  await user.type(
    screen.getByLabelText("Tarifa diaria en pesos (ARS)"),
    "12345,00",
  );
  await user.click(screen.getByRole("button", { name: /Registrar vehículo/ }));
  await screen.findByText("Consultando información…");
  expect(screen.queryByLabelText("Tarifa diaria en pesos (ARS)")).toBeNull();
  await waitFor(() => expect(release).toBeTypeOf("function"));
  await act(async () => release!(Response.json(savedVehicle)));
  const price = await screen.findByLabelText("Tarifa diaria en pesos (ARS)");
  expect((price as HTMLInputElement).value).toBe("12345.00");
  await user.clear(price);
  await user.type(price, "15000,25");
  await user.click(screen.getByRole("button", { name: /Guardar cambios/ }));
  expect(writes[1]).toMatchObject({
    method: "PUT",
    body: { dailyRateMinor: 1500025 },
  });
});

it("un conflicto de versión conserva el borrador hasta que el operador recargue", async () => {
  conflict = true;
  const user = userEvent.setup();
  mount("/operador/flota/auto-001");
  await login(user);
  const model = await screen.findByLabelText("Modelo");
  await user.clear(model);
  await user.type(model, "Mi cambio");
  await user.click(screen.getByRole("button", { name: /Guardar cambios/ }));
  await screen.findByText(/Tus cambios se conservaron/);
  expect((screen.getByLabelText("Modelo") as HTMLInputElement).value).toBe(
    "Mi cambio",
  );
  expect(writes).toHaveLength(1);
  await user.click(
    screen.getByRole("button", { name: /Descartar mis cambios/ }),
  );
  await screen.findByDisplayValue("Corolla");
});
it("registra un cliente y no expone edición inexistente", async () => {
  const user = userEvent.setup();
  mount("/operador/clientes/nuevo");
  await login(user);
  await user.type(
    await screen.findByLabelText("Nombre completo"),
    "Conductor nuevo",
  );
  fireEvent.change(screen.getByLabelText("Licencia válida hasta"), {
    target: { value: "2030-12-31" },
  });
  await user.click(screen.getByRole("button", { name: /Registrar cliente/ }));
  await screen.findByText(/Perfil disponible/);
  expect(writes[0]).toMatchObject({
    path: "/v1/customers",
    body: {
      name: "Conductor nuevo",
      active: true,
      licenseValidUntil: "2030-12-31",
    },
  });
  expect(
    screen.queryByRole("button", { name: /Eliminar cliente|Editar cliente/ }),
  ).toBeNull();
});
it("retiro y devolución exigen confirmación y siguen los estados del servidor", async () => {
  const user = userEvent.setup();
  mount("/operador/reservas?id=res-001");
  await login(user);
  await user.click(
    await screen.findByRole("button", { name: /Registrar retiro/ }),
  );
  expect(writes).toHaveLength(0);
  await user.click(screen.getByRole("button", { name: /Confirmar operación/ }));
  await screen.findByText("Retiro registrado. El alquiler está en curso.");
  await user.click(
    await screen.findByRole("button", { name: /Registrar devolución/ }),
  );
  await user.click(screen.getByRole("button", { name: /Confirmar operación/ }));
  await screen.findByText("Devolución registrada. El alquiler finalizó.");
  expect(status).toBe("COMPLETED");
  expect(writes.map((w) => w.path)).toEqual([
    "/v1/reservations/res-001/pickup",
    "/v1/reservations/res-001/return",
  ]);
  expect(
    screen.queryByRole("button", { name: /Registrar devolución/ }),
  ).toBeNull();
});
it("mantenimiento incierto sobrevive al nuevo acceso y no repite POST", async () => {
  maintenanceLost = true;
  const user = userEvent.setup();
  const view = mount("/operador/mantenimiento");
  await login(user);
  await user.type(
    await screen.findByLabelText("Identificador del vehículo"),
    "auto-001",
  );
  fireEvent.change(screen.getByLabelText("Inicio del mantenimiento"), {
    target: { value: "2030-01-10" },
  });
  fireEvent.change(screen.getByLabelText("Fin del mantenimiento"), {
    target: { value: "2030-01-12" },
  });
  await user.click(
    screen.getByRole("button", { name: /Registrar mantenimiento/ }),
  );
  await screen.findByText(/Hay un bloqueo enviado/);
  expect(writes).toHaveLength(1);
  view.unmount();
  mount("/operador/mantenimiento");
  await login(user);
  await screen.findByText(/Hay un bloqueo enviado/);
  expect(
    (
      screen.getByRole("button", {
        name: /Registrar mantenimiento/,
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true);
  expect(writes).toHaveLength(1);
  expect(
    sessionStorage.getItem("carcruds.operator.maintenance.v1"),
  ).not.toContain("secret");
});

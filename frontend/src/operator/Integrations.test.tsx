import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";
import App from "../App";

// The real gym evidence persisted in Azure for employee-001, used as fixture.
const gymEvidence = {
  employeeId: "employee-001",
  period: "2026-11",
  sourceReference: "15ae8f6d-ce74-4453-a694-13b23cdf7c64",
  version: 1,
  observedAt: "2026-11-10T15:00:00Z",
  member: true,
  used: null,
  costMinor: null,
  spendingMinor: null,
  currency: "",
};
const employees = [
  { employeeId: "employee-001", displayName: "Persona Uno", active: true },
  { employeeId: "employee-002", displayName: "Persona Dos", active: true },
  { employeeId: "employee-009", displayName: "Persona de baja", active: false },
];
const request = (state: string, person: string, classification: string) => ({
  requestId: `req-${person}`,
  localReference: `ref-${person}`,
  personKind: "employee",
  personId: person,
  patientId: "pat-001",
  doctorId: "doc-001",
  requestedStartAt: "2026-11-20T13:00:00Z",
  createdAt: "2026-11-10T22:32:49Z",
  attemptedAt: state === "ready" ? null : "2026-11-10T22:32:59Z",
  observedAt: state === "ready" ? null : "2026-11-10T22:33:00Z",
  state,
  classification,
  lastKnownAppointment: null,
});
let calls: { path: string; method: string }[] = [];
beforeEach(() => {
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init: RequestInit = {}) => {
      calls.push({ path, method: init.method ?? "GET" });
      if (path === "/v1/session") return Response.json({ role: "admin" });
      if (path.startsWith("/v1/admin/employees?"))
        return Response.json({
          items: employees,
          page: 1,
          pageSize: 100,
          total: 3,
        });
      if (path === "/v1/employees/employee-001")
        return Response.json(employees[0]);
      if (path.startsWith("/v1/employees/reports/clinic?"))
        return Response.json({
          page: 1,
          pageSize: 100,
          total: 3,
          items: [
            {
              personKind: "employee",
              personId: "employee-001",
              displayName: "Persona Uno",
              active: true,
              classification: "confirmed",
              request: request("booked", "employee-001", "confirmed"),
            },
            {
              personKind: "employee",
              personId: "employee-002",
              displayName: "Persona Dos",
              active: true,
              classification: "pending_verification",
              request: request(
                "unknown",
                "employee-002",
                "pending_verification",
              ),
            },
            {
              personKind: "employee",
              personId: "employee-009",
              displayName: "Persona de baja",
              active: false,
              classification: "pending_verification",
              request: null,
            },
          ],
        });
      if (path.startsWith("/v1/employees/reports/memberships?"))
        return Response.json({
          period: "2026-11",
          page: 1,
          pageSize: 100,
          total: 2,
          missingEvidence: 1,
          incompleteEvidence: 1,
          totals: [],
          items: [
            {
              ...employees[0],
              sourceApp: "gimnasio-pablo",
              evidence: gymEvidence,
            },
            { ...employees[1], evidence: null },
          ],
        });
      if (path.startsWith("/v1/employees/employee-001/membership-evidence?"))
        return Response.json({
          employeeId: "employee-001",
          period: "2026-11",
          items: [
            {
              ...gymEvidence,
              sourceApp: "gimnasio-pablo",
              receivedAt: "2026-11-10T15:00:02Z",
              employeeId: undefined,
              period: undefined,
            },
          ],
        });
      if (path === "/v1/employees/clinic/doctors")
        return Response.json({
          doctors: [{ id: "doc-001", durationMinutes: 30 }],
        });
      if (path.startsWith("/v1/employees/clinic/usage?"))
        return Response.json({ code: "NOT_CONFIGURED" }, { status: 503 });
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
  await user.type(
    screen.getByLabelText("Credencial del operador"),
    "admin-fixture-not-a-real-secret",
  );
  await user.click(screen.getByRole("button", { name: /Entrar al panel/ }));
}

it("agrupa el menú y navega a Personal, Hospital y Gimnasio", async () => {
  const user = userEvent.setup();
  mount("/operador");
  await login(user);
  const nav = await screen.findByRole("navigation", {
    name: "Navegación del operador",
  });
  for (const title of ["RESUMEN", "OPERACIONES", "GESTIÓN DE PERSONAL"])
    expect(within(nav).getByText(title)).toBeTruthy();
  await user.click(within(nav).getByRole("link", { name: "Hospital" }));
  await screen.findByRole("heading", {
    name: "Hospital – Gestión de turnos médicos",
  });
  await user.click(within(nav).getByRole("link", { name: "Gimnasio" }));
  await screen.findByRole("heading", {
    name: "Gimnasio – Membresías de empleados",
  });
  await user.click(within(nav).getByRole("link", { name: "Personal" }));
  await screen.findByRole("heading", {
    name: "El equipo detrás de cada viaje.",
  });
});
it("redirige las rutas anteriores de turnos y membresías", async () => {
  const user = userEvent.setup();
  mount("/operador/personal/turnos");
  await login(user);
  await screen.findByRole("heading", {
    name: "Hospital – Gestión de turnos médicos",
  });
});
it("muestra indicadores del hospital calculados con el informe real", async () => {
  const user = userEvent.setup();
  mount("/operador/hospital");
  await login(user);
  const card = async (label: string) =>
    (await screen.findByText(label)).parentElement!.querySelector("strong")!
      .textContent;
  expect(await card("CON SOLICITUD")).toBe("2");
  expect(await card("CONFIRMADAS")).toBe("1");
  expect(await card("A CONCILIAR")).toBe("1");
  expect(await card("REGISTRADAS SIN ENVIAR")).toBe("0");
  expect(screen.getAllByText("Resultado desconocido").length).toBeGreaterThan(
    0,
  );
});
it("precarga el empleado y no guarda nada hasta confirmar el resumen", async () => {
  const user = userEvent.setup();
  mount("/operador/hospital?empleado=employee-002");
  await login(user);
  const employee = (await screen.findByLabelText(
    "Empleado",
  )) as HTMLSelectElement;
  expect(employee.value).toBe("employee-002");
  expect(within(employee).queryByText(/Persona de baja/)).toBeNull();
  await user.type(
    screen.getByLabelText("ID de paciente en el hospital"),
    "pat-003",
  );
  await user.selectOptions(
    screen.getByLabelText("Médico del hospital"),
    "doc-001",
  );
  const date = screen.getByLabelText("Fecha y hora (Buenos Aires)");
  await user.type(date, "2099-01-10T10:00");
  await user.click(screen.getByRole("button", { name: /Revisar solicitud/ }));
  const dialog = screen.getByRole("dialog", {
    hidden: true,
    name: "Confirmar solicitud",
  });
  expect(within(dialog).getByText("pat-003")).toBeTruthy();
  expect(within(dialog).getByText("10/01/2099, 10:00")).toBeTruthy();
  await user.click(
    within(dialog).getByRole("button", { name: "Volver", hidden: true }),
  );
  expect(calls.filter((c) => c.method !== "GET")).toHaveLength(0);
});
it("muestra la evidencia real del gimnasio con su historial", async () => {
  const user = userEvent.setup();
  mount("/operador/gimnasio?period=2026-11");
  await login(user);
  await screen.findByText("gimnasio-pablo");
  await user.click(
    screen.getByRole("button", { name: "Ver evidencia de Persona Uno" }),
  );
  const detail = await screen.findByRole("region", { name: "Persona Uno" });
  expect(
    within(detail).getAllByText("15ae8f6d-ce74-4453-a694-13b23cdf7c64")[0],
  ).toBeTruthy();
  expect(
    within(detail).getByText("VERSIÓN VIGENTE").nextSibling?.textContent,
  ).toBe("1");
  expect(within(detail).getByText("USO").nextSibling?.textContent).toBe(
    "Sin dato",
  );
  expect(await within(detail).findByText("Vigente")).toBeTruthy();
  await user.click(
    within(detail).getByRole("button", { name: /Cerrar detalle/ }),
  );
  await user.click(
    screen.getByRole("button", { name: "Ver evidencia de Persona Dos" }),
  );
  expect(
    await screen.findByText(/No llegó evidencia del gimnasio/),
  ).toBeTruthy();
  expect(calls.every((c) => c.method === "GET")).toBe(true);
});
it("la ficha del empleado resume sus integraciones con enlaces", async () => {
  const user = userEvent.setup();
  mount("/operador/personal/employee-001");
  await login(user);
  expect(await screen.findByText("Turno confirmado")).toBeTruthy();
  expect(await screen.findByText("Socio: Sí")).toBeTruthy();
  expect(
    screen.getByRole("link", { name: /Solicitar turno/ }).getAttribute("href"),
  ).toBe("/operador/hospital?empleado=employee-001");
  expect(
    screen.getByRole("link", { name: "Ver solicitud" }).getAttribute("href"),
  ).toBe("/operador/hospital/req-employee-001");
});
it("el navegador solo habla con el gateway propio", async () => {
  const user = userEvent.setup();
  mount("/operador/hospital");
  await login(user);
  await screen.findByText("CONFIRMADAS");
  expect(calls.length).toBeGreaterThan(0);
  expect(calls.every((c) => c.path.startsWith("/v1/"))).toBe(true);
});
it("el código del frontend no contiene la API ni credenciales del hospital", () => {
  const files: string[] = [];
  const walk = (dir: string) => {
    for (const name of readdirSync(dir)) {
      const full = join(dir, name);
      if (statSync(full).isDirectory()) walk(full);
      else if (/\.(ts|tsx)$/.test(name) && !name.endsWith(".test.tsx"))
        files.push(full);
    }
  };
  walk(join(process.cwd(), "src"));
  expect(files.length).toBeGreaterThan(5);
  for (const file of files) {
    const text = readFileSync(file, "utf8");
    expect(text, file).not.toMatch(
      /api-turnos|hstgr\.cloud|X-API-Key|CLINIC_API_KEY/,
    );
  }
});

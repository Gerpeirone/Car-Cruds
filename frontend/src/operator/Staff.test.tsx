import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, expect, it, vi } from "vitest";
import App from "../App";
import { toInstant, validKey } from "./Staff";

const employee = {
  employeeId: "emp-001",
  displayName: "Persona ficticia Uno",
  active: true,
};
const clinicRequest: Record<string, unknown> = {
  requestId: "req-001",
  localReference: "turno-ref",
  personKind: "employee",
  personId: "emp-001",
  patientId: "pac-001",
  doctorId: "doc-001",
  requestedStartAt: "2099-01-10T13:00:00Z",
  createdAt: "2026-11-09T12:00:00Z",
  attemptedAt: null,
  observedAt: null,
  state: "ready",
  classification: "pending_verification",
  lastKnownAppointment: null,
};
let saved = { ...employee };
let request = { ...clinicRequest };
let requestLost = false;
let submitResult: "booked" | "not-configured" = "booked";
let writes: { path: string; method: string; body: unknown }[] = [];
beforeEach(() => {
  saved = { ...employee };
  request = { ...clinicRequest };
  requestLost = false;
  submitResult = "booked";
  writes = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init: RequestInit = {}) => {
      const body = init.body ? JSON.parse(String(init.body)) : undefined;
      if (init.method && init.method !== "GET")
        writes.push({ path, method: init.method, body });
      if (path === "/v1/session") return Response.json({ role: "admin" });
      if (path.startsWith("/v1/admin/employees?"))
        return Response.json({ items: [saved], page: 1, pageSize: 20, total: 1 });
      if (path === "/v1/employees" && init.method === "POST") {
        saved = { employeeId: "emp-002", ...body };
        return Response.json(saved, { status: 201 });
      }
      if (path.startsWith("/v1/employees/reports/memberships?"))
        return Response.json({
          period: "2026-10",
          page: 1,
          pageSize: 20,
          total: 2,
          missingEvidence: 1,
          incompleteEvidence: 1,
          items: [
            {
              ...employee,
              sourceApp: "gimnasio",
              evidence: {
                employeeId: "emp-001",
                period: "2026-10",
                sourceReference: "gym-ref-1",
                version: 2,
                observedAt: "2026-10-28T12:00:00Z",
                member: true,
                used: null,
                costMinor: 1500000,
                spendingMinor: null,
                currency: "ARS",
              },
            },
            {
              employeeId: "emp-003",
              displayName: "Persona sin datos",
              active: false,
              evidence: null,
            },
          ],
          totals: [
            {
              currency: "ARS",
              costMinor: 1500000,
              spendingMinor: null,
              costRecords: 1,
              spendingRecords: 0,
            },
          ],
        });
      if (path === "/v1/employees/clinic/patient-links")
        return new Response(null, { status: 204 });
      if (path === "/v1/employees/clinic/requests") {
        if (requestLost) throw new TypeError("respuesta perdida");
        request = {
          ...clinicRequest,
          localReference: body.localReference,
          requestedStartAt: body.startAt,
        };
        return Response.json(request);
      }
      if (path.startsWith("/v1/employees/reports/clinic?"))
        return Response.json({
          items: [
            {
              personKind: "employee",
              personId: "emp-001",
              displayName: employee.displayName,
              active: true,
              classification: request.classification,
              request,
            },
          ],
          page: 1,
          pageSize: 20,
          total: 1,
        });
      if (path.startsWith("/v1/employees/clinic/requests/")) {
        if (path.endsWith("/submit")) {
          if (submitResult === "not-configured")
            return Response.json({ code: "NOT_CONFIGURED" }, { status: 503 });
          request = {
            ...request,
            state: "booked",
            classification: "confirmed",
            attemptedAt: "2026-11-09T12:01:00Z",
            observedAt: "2026-11-09T12:01:00Z",
            lastKnownAppointment: {
              id: "apt-1",
              startAt: request.requestedStartAt,
              endAt: request.requestedStartAt,
              status: "booked",
            },
          };
        }
        return Response.json(request);
      }
      if (path.startsWith("/v1/employees/")) {
        if (init.method === "PUT") saved = { ...saved, ...body };
        if (init.method === "DELETE") saved = { ...saved, active: false };
        return Response.json(saved);
      }
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
async function fillClinic(user: ReturnType<typeof userEvent.setup>) {
  await user.type(await screen.findByLabelText("ID del empleado"), "emp-001");
  await user.type(screen.getByLabelText("ID de paciente en el hospital"), "pac-001");
  await user.type(screen.getByLabelText("ID del profesional"), "doc-001");
  fireEvent.change(screen.getByLabelText("Fecha y hora (Buenos Aires)"), {
    target: { value: "2099-01-10T10:00" },
  });
}

it("valida claves opacas y convierte la hora de Buenos Aires a UTC", () => {
  expect(validKey("emp-001")).toBe(true);
  expect(validKey(" emp")).toBe(false);
  expect(validKey("a\u0007b")).toBe(false);
  expect(validKey("x".repeat(101))).toBe(false);
  expect(toInstant("2099-01-10T10:00")).toBe("2099-01-10T13:00:00Z");
  expect(() => toInstant("2000-01-01T10:00")).toThrow();
});
it("lista el personal y registra un empleado sin reintentar el alta", async () => {
  const user = userEvent.setup();
  mount("/operador/personal");
  await login(user);
  await screen.findByText("Persona ficticia Uno");
  expect(screen.getByText("1 empleados · Página 1")).toBeTruthy();
  await user.click(screen.getByRole("link", { name: /Registrar empleado/ }));
  await user.type(
    await screen.findByLabelText("Nombre administrativo"),
    "  Persona nueva  ",
  );
  await user.click(screen.getByRole("button", { name: /Registrar empleado/ }));
  await screen.findByText(/Empleado registrado: Persona nueva/);
  expect(writes).toEqual([
    {
      path: "/v1/employees",
      method: "POST",
      body: { displayName: "Persona nueva", active: true },
    },
  ]);
  expect(
    (screen.getByRole("button", { name: /Registrar empleado/ }) as HTMLButtonElement)
      .disabled,
  ).toBe(true);
});
it("edita la ficha y da de baja con confirmación explícita", async () => {
  const user = userEvent.setup();
  mount("/operador/personal/emp-001");
  await login(user);
  const name = await screen.findByDisplayValue("Persona ficticia Uno");
  await user.clear(name);
  await user.type(name, "Persona renombrada");
  await user.click(screen.getByRole("button", { name: /Guardar cambios/ }));
  await screen.findByText(/Ficha actualizada: Persona renombrada/);
  await user.click(screen.getByRole("button", { name: /Dar de baja/ }));
  const dialog = screen.getByRole("dialog", { hidden: true });
  await user.click(
    within(dialog).getByRole("button", { name: /Confirmar baja/, hidden: true }),
  );
  await screen.findByText(/Baja registrada/);
  expect(writes.map((w) => w.method)).toEqual(["PUT", "DELETE"]);
  expect(writes[0].body).toEqual({
    displayName: "Persona renombrada",
    active: true,
  });
});
it("muestra el informe de membresías sin convertir faltantes en cero", async () => {
  const user = userEvent.setup();
  mount("/operador/personal/membresias?period=2026-10");
  await login(user);
  await screen.findByText("Persona sin datos");
  expect(screen.getByText("Sin evidencia")).toBeTruthy();
  expect(screen.getByText("TOTAL ARS")).toBeTruthy();
  expect(screen.getAllByText(/Sin dato/).length).toBeGreaterThan(0);
  expect(screen.getByText(/v2 ·/)).toBeTruthy();
  expect(writes).toHaveLength(0);
});
it("vincula, guarda la intención y la envía al hospital", async () => {
  const user = userEvent.setup();
  mount("/operador/personal/turnos");
  await login(user);
  await fillClinic(user);
  await user.click(screen.getByRole("button", { name: /Guardar solicitud/ }));
  await screen.findByText(/Solicitud guardada. Todavía no se envió/);
  expect(writes[0]).toEqual({
    path: "/v1/employees/clinic/patient-links",
    method: "POST",
    body: { personKind: "employee", personId: "emp-001", patientId: "pac-001" },
  });
  expect(writes[1]).toMatchObject({
    path: "/v1/employees/clinic/requests",
    body: { personId: "emp-001", doctorId: "doc-001", startAt: "2099-01-10T13:00:00Z" },
  });
  expect(sessionStorage.length).toBe(0);
  await user.click(
    await screen.findByRole("button", { name: /Enviar al hospital/ }),
  );
  const dialog = screen.getByRole("dialog", { hidden: true });
  await user.click(
    within(dialog).getByRole("button", { name: /Confirmar envío/, hidden: true }),
  );
  await screen.findByText("El hospital confirmó el turno.");
  expect(writes[2]).toMatchObject({
    path: "/v1/employees/clinic/requests/req-001/submit",
    method: "POST",
  });
});
it("recupera una intención incierta con la misma referencia tras volver a entrar", async () => {
  requestLost = true;
  const user = userEvent.setup();
  const first = mount("/operador/personal/turnos");
  await login(user);
  await fillClinic(user);
  await user.click(screen.getByRole("button", { name: /Guardar solicitud/ }));
  await screen.findByText(/La solicitud pudo haberse guardado/);
  const reference = (writes[1].body as { localReference: string }).localReference;
  first.unmount();
  requestLost = false;
  mount("/operador/personal/turnos");
  await login(user);
  await screen.findByText(/La solicitud pudo haberse guardado/);
  expect(screen.getByText(reference)).toBeTruthy();
  await user.click(
    screen.getByRole("button", { name: /reintentar con la misma referencia/ }),
  );
  await user.click(screen.getByRole("button", { name: /Guardar solicitud/ }));
  await screen.findByText(/Solicitud guardada/);
  const creations = writes.filter((w) => w.path === "/v1/employees/clinic/requests");
  expect(creations).toHaveLength(2);
  expect(creations[1].body).toEqual(creations[0].body);
});
it("informa la falta de configuración del hospital sin marcar resultado incierto", async () => {
  submitResult = "not-configured";
  const user = userEvent.setup();
  mount("/operador/personal/turnos/req-001");
  await login(user);
  await user.click(await screen.findByRole("button", { name: /Enviar al hospital/ }));
  const dialog = screen.getByRole("dialog", { hidden: true });
  await user.click(
    within(dialog).getByRole("button", { name: /Confirmar envío/, hidden: true }),
  );
  await screen.findByText(/no está configurada en este entorno/);
  expect(screen.queryByText(/No sabemos si el hospital recibió/)).toBeNull();
});

import { afterAll, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import process from "node:process";
import App from "./App";
import { dateLabel, today } from "./domain";

// Opt-in. Usa exclusivamente un gateway local y fixtures propios de esta ejecución.
const enabled = process.env.FRONTEND_LIVE_TEST === "true";
const nativeFetch = globalThis.fetch;
it.runIf(enabled)(
  "recorrido React con gateway y persistencia reales: reservar, recuperar y cancelar",
  async () => {
    const base = process.env.FRONTEND_TEST_BASE_URL || "http://127.0.0.1:8080";
    const url = new URL(base);
    if (
      url.protocol !== "http:" ||
      !["127.0.0.1", "localhost"].includes(url.hostname) ||
      url.pathname !== "/" ||
      url.search ||
      url.username ||
      url.password
    )
      throw new Error("Solo se permite el gateway local de laboratorio.");
    const client = process.env.FRONTEND_TEST_CLIENT_TOKEN;
    const admin = process.env.FRONTEND_TEST_ADMIN_TOKEN;
    const customerID = process.env.FRONTEND_TEST_CUSTOMER_ID || "cli-001";
    if (!client || !admin || client === admin)
      throw new Error("Faltan credenciales distintas del laboratorio.");
    const vehicleID = "web-" + crypto.randomUUID();
    const model = "Web-" + vehicleID.slice(-8);
    const call = async (
      path: string,
      token: string,
      method = "GET",
      body?: unknown,
    ) => {
      const response = await nativeFetch(new URL(path, base), {
        method,
        headers: {
          Authorization: `Bearer ${token}`,
          ...(body ? { "Content-Type": "application/json" } : {}),
        },
        body: body ? JSON.stringify(body) : undefined,
        redirect: "error",
      });
      const data = await response.json();
      if (!response.ok)
        throw new Error(
          `Fixture API ${method}: ${response.status} ${data.code}`,
        );
      return data;
    };
    const vehicle = {
      vehicleId: vehicleID,
      brand: "PruebaFrontend",
      model,
      category: "sedan",
      transmission: "automatic",
      seats: 5,
      dailyRateMinor: 1234500,
      currency: "ARS",
      active: true,
    };
    let reservationID: string | undefined;
    let lost = false;
    const posts: { key: string; body: string }[] = [];
    try {
      await call("/v1/vehicles", admin, "POST", vehicle);
      let indexed = false;
      for (let attempt = 0; attempt < 25; attempt++) {
        const data = await call(`/v1/vehicles/search?q=${model}`, client);
        if (
          data.items.some(
            (v: { vehicleId: string }) => v.vehicleId === vehicleID,
          )
        ) {
          indexed = true;
          break;
        }
        await new Promise((resolve) => setTimeout(resolve, 300));
      }
      expect(indexed).toBe(true);
      globalThis.fetch = (async (
        path: string | URL | Request,
        init: RequestInit = {},
      ) => {
        if (typeof path !== "string") throw new Error("Ruta inesperada");
        const response = await nativeFetch(new URL(path, base), init);
        if (path === "/v1/reservations" && init.method === "POST") {
          posts.push({
            key: (init.headers as Record<string, string>)["Idempotency-Key"],
            body: String(init.body),
          });
          const data = await response.clone().json();
          if (response.ok) reservationID = data.id;
          if (response.ok && !lost) {
            lost = true;
            throw new TypeError(
              "Pérdida de respuesta provocada después del commit.",
            );
          }
        }
        return response;
      }) as typeof fetch;
      const user = userEvent.setup();
      const login = async () => {
        await user.type(
          screen.getByLabelText("Identificador de cliente"),
          customerID,
        );
        await user.type(screen.getByLabelText("Credencial de acceso"), client);
        await user.click(
          screen.getByRole("button", { name: /Empezar mi viaje/ }),
        );
      };
      render(
        <MemoryRouter initialEntries={[`/autos?q=${model}`]}>
          <App />
        </MemoryRouter>,
      );
      await login();
      await user.click(
        await screen.findByRole("link", {
          name: `Ver PruebaFrontend ${model}`,
        }),
      );
      await screen.findByRole("button", { name: /Consultar disponibilidad/ });
      const start = new Date(`${today()}T00:00:00Z`);
      start.setUTCDate(start.getUTCDate() + 1);
      const end = new Date(start);
      end.setUTCDate(end.getUTCDate() + 3);
      const startsOn = start.toISOString().slice(0, 10),
        endsOn = end.toISOString().slice(0, 10);
      fireEvent.change(screen.getByLabelText("Fecha de retiro"), {
        target: { value: startsOn },
      });
      fireEvent.change(screen.getByLabelText("Fecha de devolución"), {
        target: { value: endsOn },
      });
      await user.click(
        screen.getByRole("button", { name: /Consultar disponibilidad/ }),
      );
      await user.click(
        await screen.findByRole("button", { name: /Revisar mi reserva/ }),
      );
      await user.click(
        screen.getByRole("button", { name: /Confirmar reserva/ }),
      );
      await screen.findByRole("button", { name: /Recuperar resultado/ });
      expect(reservationID).toBeTruthy();
      cleanup();
      render(
        <MemoryRouter initialEntries={["/reservar"]}>
          <App />
        </MemoryRouter>,
      );
      await login();
      await user.click(
        await screen.findByRole("button", { name: /Recuperar resultado/ }),
      );
      await screen.findByRole("heading", { name: reservationID });
      expect(posts).toHaveLength(2);
      expect(posts[1]).toEqual(posts[0]);
      expect(
        (await call(`/v1/reservations/${reservationID}`, client)).status,
      ).toBe("RESERVED");
      await user.click(
        screen.getByRole("button", { name: "Cancelar reserva" }),
      );
      await user.click(screen.getByRole("button", { name: "Sí, cancelar" }));
      await screen.findByRole("heading", {
        name: "Tu reserva está cancelada.",
      });
      expect(
        (await call(`/v1/reservations/${reservationID}`, client)).status,
      ).toBe("CANCELLED");
    } finally {
      globalThis.fetch = nativeFetch;
      cleanup();
      if (reservationID)
        await call(`/v1/reservations/${reservationID}`, admin, "DELETE");
      const current = await call(`/v1/vehicles/${vehicleID}`, admin);
      await call(`/v1/vehicles/${vehicleID}`, admin, "PUT", {
        ...vehicle,
        version: current.version,
        active: false,
      });
    }
  },
  60000,
);
afterAll(() => {
  globalThis.fetch = nativeFetch;
});

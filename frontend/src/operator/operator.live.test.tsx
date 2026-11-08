import { afterAll, expect, it } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import process from "node:process";
import App from "../App";
import { today } from "../domain";

const nativeFetch = globalThis.fetch;
it.runIf(process.env.OPERATOR_LIVE_TEST === "true")(
  "panel operador con servicios reales: flota, cliente, retiro, devolución y mantenimiento",
  async () => {
    const base = process.env.OPERATOR_TEST_BASE_URL || "http://127.0.0.1:8081";
    const url = new URL(base);
    if (
      url.protocol !== "http:" ||
      !["localhost", "127.0.0.1"].includes(url.hostname) ||
      url.pathname !== "/" ||
      url.search ||
      url.username ||
      url.password
    )
      throw new Error("Solo se admite el conjunto local.");
    const token = process.env.OPERATOR_TEST_TOKEN;
    if (!token) throw new Error("Falta la credencial administrativa local.");
    const call = async (path: string, method = "GET", body?: unknown) => {
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
        throw new Error(`Fixture API ${response.status}: ${data.code}`);
      return data;
    };
    let vehicleID: string | undefined;
    let reservationID: string | undefined;
    const rates: { sent: number; received: number }[] = [];
    const day = (offset: number) => {
      const d = new Date(`${today()}T00:00:00Z`);
      d.setUTCDate(d.getUTCDate() + offset);
      return d.toISOString().slice(0, 10);
    };
    try {
      globalThis.fetch = (async (
        path: string | URL | Request,
        options?: RequestInit,
      ) => {
        if (typeof path !== "string") throw new Error("Ruta inesperada");
        const response = await nativeFetch(new URL(path, base), options);
        if (path.startsWith("/v1/vehicles/") && options?.method === "PUT") {
          const data = await response.clone().json();
          rates.push({
            sent: JSON.parse(String(options.body)).dailyRateMinor,
            received: data.dailyRateMinor,
          });
        }
        return response;
      }) as typeof fetch;
      const user = userEvent.setup();
      render(
        <MemoryRouter initialEntries={["/operador/flota/nuevo"]}>
          <App />
        </MemoryRouter>,
      );
      await user.type(screen.getByLabelText("Credencial del operador"), token);
      await user.click(screen.getByRole("button", { name: /Entrar al panel/ }));
      vehicleID = (
        (await screen.findByLabelText("Identificador")) as HTMLInputElement
      ).value;
      await user.type(screen.getByLabelText("Marca"), "PruebaOperador");
      await user.type(
        screen.getByLabelText("Modelo"),
        "Panel-" + vehicleID.slice(-8),
      );
      await user.type(screen.getByLabelText("Categoría"), "sedan");
      await user.type(
        screen.getByLabelText("Tarifa diaria en pesos (ARS)"),
        "54321,09",
      );
      await user.click(
        screen.getByRole("button", { name: /Registrar vehículo/ }),
      );
      await screen.findByText("Vehículo registrado.");
      const price = await screen.findByLabelText(
        "Tarifa diaria en pesos (ARS)",
      );
      await user.clear(price);
      await user.type(price, "15000,25");
      await user.click(screen.getByRole("button", { name: /Guardar cambios/ }));
      await screen.findByText("Ficha actualizada.");
      expect(
        (await call(`/v1/vehicles/${vehicleID}`)).dailyRateMinor,
        JSON.stringify({
          rates,
          quote: await call(`/v1/vehicles/${vehicleID}/quote`),
        }),
      ).toBe(1500025);
      await user.click(screen.getByRole("link", { name: "Clientes" }));
      await user.click(
        await screen.findByRole("link", { name: /Registrar cliente/ }),
      );
      const customerID = (
        (await screen.findByLabelText(
          "Identificador del cliente",
        )) as HTMLInputElement
      ).value;
      await user.type(
        screen.getByLabelText("Nombre completo"),
        "Cliente prueba operador",
      );
      fireEvent.change(screen.getByLabelText("Licencia válida hasta"), {
        target: { value: "2030-12-31" },
      });
      await user.click(
        screen.getByRole("button", { name: /Registrar cliente/ }),
      );
      await screen.findByText(/Perfil disponible/);
      expect((await call(`/v1/customers/${customerID}`)).name).toBe(
        "Cliente prueba operador",
      );
      const response = await nativeFetch(new URL("/v1/reservations", base), {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
          "Idempotency-Key": crypto.randomUUID(),
        },
        body: JSON.stringify({
          vehicleId: vehicleID,
          customerId: customerID,
          startsOn: day(0),
          endsOn: day(2),
          expectedTotalMinor: 3000050,
        }),
      });
      const reservation = await response.json();
      expect(response.status).toBe(201);
      reservationID = reservation.id;
      await user.click(screen.getByRole("link", { name: "Reservas" }));
      await user.type(
        await screen.findByLabelText("Código de reserva"),
        reservationID!,
      );
      await user.click(
        screen.getByRole("button", { name: /Consultar reserva/ }),
      );
      await user.click(
        await screen.findByRole("button", { name: /Registrar retiro/ }),
      );
      await user.click(
        screen.getByRole("button", { name: /Confirmar operación/ }),
      );
      await screen.findByText("Retiro registrado. El alquiler está en curso.");
      expect((await call(`/v1/reservations/${reservationID}`)).status).toBe(
        "ACTIVE",
      );
      await user.click(
        await screen.findByRole("button", { name: /Registrar devolución/ }),
      );
      await user.click(
        screen.getByRole("button", { name: /Confirmar operación/ }),
      );
      await screen.findByText("Devolución registrada. El alquiler finalizó.");
      expect((await call(`/v1/reservations/${reservationID}`)).status).toBe(
        "COMPLETED",
      );
      await user.click(screen.getByRole("link", { name: "Mantenimiento" }));
      await user.type(
        await screen.findByLabelText("Identificador del vehículo"),
        vehicleID,
      );
      fireEvent.change(screen.getByLabelText("Inicio del mantenimiento"), {
        target: { value: day(5) },
      });
      fireEvent.change(screen.getByLabelText("Fin del mantenimiento"), {
        target: { value: day(7) },
      });
      await user.click(
        screen.getByRole("button", { name: /Registrar mantenimiento/ }),
      );
      await screen.findByText("Mantenimiento registrado.");
      expect(
        (
          await call(
            `/v1/availability?${new URLSearchParams({ vehicleId: vehicleID, startsOn: day(5), endsOn: day(7) })}`,
          )
        ).available,
      ).toBe(false);
    } finally {
      cleanup();
      globalThis.fetch = nativeFetch;
      if (reservationID) {
        const r = await call(`/v1/reservations/${reservationID}`);
        if (r.status === "ACTIVE")
          await call(`/v1/reservations/${reservationID}/return`, "POST");
        else if (r.status === "RESERVED")
          await call(`/v1/reservations/${reservationID}`, "DELETE");
      }
      if (vehicleID) {
        try {
          const v = await call(`/v1/vehicles/${vehicleID}`);
          await call(`/v1/vehicles/${vehicleID}`, "PUT", {
            vehicleId: v.vehicleId,
            brand: v.brand,
            model: v.model,
            category: v.category,
            transmission: v.transmission,
            seats: v.seats,
            dailyRateMinor: v.dailyRateMinor,
            currency: v.currency,
            version: v.version,
            active: false,
          });
        } catch (err) {
          throw new Error(
            "No se pudo desactivar el vehículo ficticio propio. Revisar limpieza.",
          );
        }
      }
    }
  },
  60000,
);
afterAll(() => {
  globalThis.fetch = nativeFetch;
});

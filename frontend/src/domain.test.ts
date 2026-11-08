import { describe, expect, it, vi } from "vitest";
import {
  clearPending,
  dateError,
  money,
  readPending,
  savePending,
  today,
  validPrice,
} from "./domain";
import type { Pending } from "./domain";
import { ApiError, request } from "./api";

const operation: Pending = {
  version: 1,
  key: "12345678-1234-1234-1234-123456789012",
  body: {
    customerId: "cli-001",
    vehicleId: "auto-001",
    startsOn: "2030-01-02",
    endsOn: "2030-01-05",
    expectedTotalMinor: 15000000,
  },
};
describe("fechas calendario y dinero", () => {
  it("calcula hoy en Buenos Aires aun si UTC ya cambió de día", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-11-09T01:00:00Z"));
    expect(today()).toBe("2026-11-08");
  });
  it.each([
    ["2030-02-30", "2030-03-02"],
    ["2030-01-02", "2030-01-02"],
    ["2030-01-02", "2030-02-02"],
    ["2029-12-31", "2030-01-03"],
  ])("rechaza período inválido %s %s", (s, e) =>
    expect(dateError(s, e, "2030-01-01")).not.toBe(""),
  );
  it("acepta límites de 1 y 30 días y un cambio de mes", () => {
    expect(dateError("2030-01-02", "2030-01-03", "2030-01-01")).toBe("");
    expect(dateError("2030-01-02", "2030-02-01", "2030-01-01")).toBe("");
  });
  it("no muestra centavos como pesos ni redondea enteros inseguros", () => {
    expect(money(12345)).toContain("123,45");
    expect(() => money(Number.MAX_SAFE_INTEGER + 1)).toThrow();
    expect(
      validPrice({ dailyRateMinor: 1, totalMinor: 1, currency: "USD" }),
    ).toBe(false);
  });
});
describe("operación pendiente de reserva", () => {
  it("sobrevive a recarga sin credenciales y permanece aislada por cliente", () => {
    savePending(operation);
    expect(readPending("cli-001")).toEqual(operation);
    expect(readPending("cli-other")).toBeNull();
    expect(sessionStorage.getItem("carcruds.pending.v1:cli-001")).not.toMatch(
      /token|credential|license|name/,
    );
    clearPending("cli-001");
    expect(readPending("cli-001")).toBeNull();
  });
  it("no descarta silenciosamente una referencia dañada", () => {
    sessionStorage.setItem("carcruds.pending.v1:cli-001", '{"broken":true}');
    expect(() => readPending("cli-001")).toThrow();
  });
  it("propaga falta de almacenamiento antes de enviar", () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementationOnce(() => {
      throw new Error("blocked");
    });
    expect(() => savePending(operation)).toThrow("blocked");
  });
});
describe("transporte HTTP", () => {
  it("usa solo la credencial recibida y no reintenta una escritura fallida", async () => {
    const fetcher = vi.fn().mockRejectedValue(new TypeError("offline"));
    vi.stubGlobal("fetch", fetcher);
    await expect(
      request("/v1/reservations", "fixture-client", {
        method: "POST",
        body: "{}",
      }),
    ).rejects.toMatchObject({ code: "NETWORK_ERROR" });
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(fetcher.mock.calls[0][1]).toMatchObject({
      credentials: "omit",
      redirect: "error",
      headers: {
        Authorization: "Bearer fixture-client",
        "Content-Type": "application/json",
      },
    });
  });
  it("tolera respuestas de texto y conserva correlación", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response("no disponible", {
            status: 503,
            headers: { "X-Request-ID": "test-reference" },
          }),
        ),
    );
    await expect(
      request("/v1/vehicles/search", "fixture"),
    ).rejects.toMatchObject({ status: 503, requestID: "test-reference" });
  });
  it("usa código de error y estado sin depender del mensaje", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          Response.json(
            { code: "PRICE_CHANGED", message: "otro texto" },
            { status: 409 },
          ),
        ),
    );
    await expect(request("/v1/reservations", "fixture")).rejects.toEqual(
      new ApiError(409, "PRICE_CHANGED"),
    );
  });
});

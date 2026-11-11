import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { expect, it, vi } from "vitest";
import { AccountPortal } from "./AccountPortal";

it("valida confirmación, registra e inicia sesión usando cookies del mismo origen", async () => {
  const send = vi.fn(async (path: string) => {
    if (path === "/v1/auth/me") return new Response('{"code":"UNAUTHORIZED"}', { status: 401 });
    if (path === "/v1/auth/register") return new Response('{"status":"accepted"}', { status: 202 });
    return new Response('{"id":"1","firstName":"Ana","lastName":"Demo","email":"ana@example.com","role":"client"}');
  });
  vi.stubGlobal("fetch", send);
  render(<MemoryRouter initialEntries={["/registro"]}>
    <AccountPortal catalog={() => <h1>Catálogo de prueba</h1>} />
  </MemoryRouter>);
  const change = (label: string, value: string) =>
    fireEvent.change(screen.getByLabelText(label, { exact: true }), { target: { value } });
  change("Nombre", "Ana");
  change("Apellido", "Demo");
  change("Correo electrónico", "ana@example.com");
  change("Contraseña", "Una contraseña de prueba");
  fireEvent.click(screen.getByRole("button", { name: "Mostrar contraseña" }));
  expect(screen.getByLabelText("Contraseña", { exact: true }).getAttribute("type")).toBe("text");
  change("Confirmar contraseña", "diferente");
  fireEvent.click(screen.getByRole("button", { name: "Crear cuenta" }));
  expect((await screen.findByRole("alert")).textContent).toContain("Las contraseñas no coinciden");
  expect(send.mock.calls.some(([path]) => path === "/v1/auth/register")).toBe(false);
  change("Confirmar contraseña", "Una contraseña de prueba");
  fireEvent.click(screen.getByRole("button", { name: "Crear cuenta" }));
  await screen.findByRole("heading", { name: "Bienvenido de nuevo" });
  change("Correo electrónico", "ana@example.com");
  change("Contraseña", "Una contraseña de prueba");
  fireEvent.click(screen.getByRole("button", { name: "Iniciar sesión" }));
  await screen.findByRole("heading", { name: "Catálogo de prueba" });
  expect(send).toHaveBeenCalledWith("/v1/auth/login", expect.objectContaining({ credentials: "same-origin" }));
  expect(screen.getByRole("link", { name: "Mi perfil" }).getAttribute("href")).toBe("/perfil");
});

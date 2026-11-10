export interface Customer {
  id: string;
  name: string;
  active: boolean;
  licenseValidUntil: string;
}
export interface Vehicle {
  vehicleId: string;
  brand: string;
  model: string;
  category: string;
  transmission: string;
  seats: number;
  dailyRateMinor: number;
  currency: string;
  active: boolean;
  version: number;
}
export interface VehiclePage {
  items: Vehicle[];
  page: number;
  pageSize: number;
  total: number;
}
export interface Availability {
  vehicleId: string;
  startsOn: string;
  endsOn: string;
  available: boolean;
  days: number;
  dailyRateMinor: number;
  totalMinor: number;
  currency: string;
  unavailableReason?: string;
}
export interface Reservation {
  id: string;
  status: "RESERVED" | "ACTIVE" | "COMPLETED" | "CANCELLED";
  vehicleId: string;
  customerId: string;
  startsOn: string;
  endsOn: string;
  days: number;
  dailyRateMinor: number;
  totalMinor: number;
  currency: string;
}
export interface CreateReservation {
  vehicleId: string;
  customerId: string;
  startsOn: string;
  endsOn: string;
  expectedTotalMinor: number;
}
export interface Pending {
  version: 1;
  key: string;
  body: CreateReservation;
}
export interface Quote {
  vehicle: Vehicle;
  availability: Availability;
}

export const validID = (id: string) =>
  /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/.test(id);
export function today() {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Buenos_Aires",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
}
export function dateLabel(date: string) {
  const [y, m, d] = date.split("-");
  return `${d}/${m}/${y}`;
}
export function dateError(
  start: string,
  end: string,
  current = today(),
): string {
  const parse = (s: string) =>
    /^\d{4}-\d{2}-\d{2}$/.test(s) &&
    !Number.isNaN(Date.parse(s)) &&
    new Date(s).toISOString().slice(0, 10) === s;
  if (!parse(start) || !parse(end))
    return "Elegí una fecha de retiro y una de devolución válidas.";
  if (start < current)
    return "El retiro no puede ser anterior a hoy en Argentina.";
  const days = (Date.parse(end) - Date.parse(start)) / 86400000;
  return days < 1 || days > 30
    ? "El alquiler debe durar entre 1 y 30 días. La devolución no se cuenta como día de uso."
    : "";
}
export function money(minor: number, currency = "ARS") {
  if (!Number.isSafeInteger(minor) || minor < 0 || currency !== "ARS")
    throw new Error("Importe inválido");
  return new Intl.NumberFormat("es-AR", {
    style: "currency",
    currency: "ARS",
    maximumFractionDigits: minor % 100 ? 2 : 0,
  }).format(minor / 100);
}
export function validPrice(value: {
  dailyRateMinor: number;
  totalMinor: number;
  currency: string;
}) {
  return (
    value.currency === "ARS" &&
    Number.isSafeInteger(value.dailyRateMinor) &&
    value.dailyRateMinor > 0 &&
    Number.isSafeInteger(value.totalMinor) &&
    value.totalMinor > 0
  );
}
const pendingKey = (id: string) => `carcruds.pending.v1:${id}`;
export function readPending(customerID: string): Pending | null {
  const raw = sessionStorage.getItem(pendingKey(customerID));
  if (!raw) return null;
  const p = JSON.parse(raw) as Pending;
  if (
    p.version !== 1 ||
    !/^[a-zA-Z0-9-]{36}$/.test(p.key) ||
    !p.body ||
    p.body.customerId !== customerID ||
    !validID(p.body.vehicleId) ||
    !Number.isSafeInteger(p.body.expectedTotalMinor) ||
    p.body.expectedTotalMinor < 0 ||
    !/^\d{4}-\d{2}-\d{2}$/.test(p.body.startsOn) ||
    !/^\d{4}-\d{2}-\d{2}$/.test(p.body.endsOn)
  )
    throw new Error(
      "No se pudo leer la operación pendiente. No vuelvas a reservar: pedí ayuda para recuperar su resultado.",
    );
  return p;
}
export function savePending(p: Pending) {
  sessionStorage.setItem(pendingKey(p.body.customerId), JSON.stringify(p));
}
export function clearPending(id: string) {
  sessionStorage.removeItem(pendingKey(id));
}
export const statusLabels = {
  RESERVED: "Reservada",
  ACTIVE: "En curso",
  COMPLETED: "Finalizada",
  CANCELLED: "Cancelada",
};

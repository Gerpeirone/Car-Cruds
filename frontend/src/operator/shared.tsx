import { createContext, useContext, useEffect, useRef, useState } from "react";
import type { ReactNode, ButtonHTMLAttributes } from "react";
import { ArrowLeft, AlertCircle, CheckCheck, RefreshCw } from "lucide-react";
import { Link } from "react-router-dom";
import { ApiError, errorMessage } from "../api";
import { validID, validPrice } from "../domain";
import type { Customer, Reservation, Vehicle } from "../domain";

export type Call = <T>(path: string, options?: RequestInit) => Promise<T>;
export const OperatorContext = createContext<{
  call: Call;
  logout: () => void;
} | null>(null);
export function useOperator() {
  const value = useContext(OperatorContext);
  if (!value) throw new Error("Acceso del operador requerido");
  return value;
}
export function useLoad<T>(
  path: string | null,
  valid: (data: T) => boolean,
): { data?: T; error?: unknown; loading: boolean; reload: () => void } {
  const { call } = useOperator();
  const [state, setState] = useState<{
    path: string | null;
    data?: T;
    error?: unknown;
    loading: boolean;
  }>({ path, loading: !!path });
  const [version, setVersion] = useState(0);
  useEffect(() => {
    if (!path) {
      setState({ path, loading: false });
      return;
    }
    let active = true;
    const controller = new AbortController();
    setState({ path, loading: true });
    call<T>(path, { signal: controller.signal })
      .then((data) => {
        if (!valid(data)) throw new ApiError(0, "INVALID_RESPONSE");
        if (active) setState({ path, data, loading: false });
      })
      .catch((error) => {
        if (active) setState({ path, error, loading: false });
      });
    return () => {
      active = false;
      controller.abort();
    };
  }, [path, version, call, valid]);
  return {
    ...(state.path === path ? state : { loading: !!path }),
    reload: () => {
      setState({ path, loading: !!path });
      setVersion((v) => v + 1);
    },
  };
}
export function useCommand() {
  const lock = useRef(false);
  const [busy, setBusy] = useState(false);
  return {
    busy,
    run: async (fn: () => Promise<void>) => {
      if (lock.current) return;
      lock.current = true;
      setBusy(true);
      try {
        await fn();
      } finally {
        lock.current = false;
        setBusy(false);
      }
    },
  };
}
export function Action({
  busy,
  children,
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { busy?: boolean }) {
  return (
    <button
      {...props}
      disabled={busy || props.disabled}
      aria-busy={busy || undefined}
      className={`button ${className}`}
    >
      {busy && <span className="spinner" />}
      {children}
    </button>
  );
}
export function Issue({
  error,
  retry,
}: {
  error: unknown;
  retry?: () => void;
}) {
  return (
    <div className="notice error" role="alert">
      <AlertCircle size={19} />
      <div>
        <p>{errorMessage(error)}</p>
        {error instanceof ApiError && error.requestID && (
          <p className="field-hint">Referencia de soporte: {error.requestID}</p>
        )}
        {retry && (
          <button className="text-button" onClick={retry}>
            Volver a consultar <RefreshCw size={14} />
          </button>
        )}
      </div>
    </div>
  );
}
export function Success({ children }: { children: ReactNode }) {
  return (
    <div className="notice success" role="status">
      <CheckCheck size={20} />
      <div>{children}</div>
    </div>
  );
}
export function Heading({
  title,
  eyebrow = "ESPACIO DEL OPERADOR",
  children,
}: {
  title: string;
  eyebrow?: string;
  children?: ReactNode;
}) {
  return (
    <div className="page-heading">
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h1 tabIndex={-1}>{title}</h1>
      </div>
      {children}
    </div>
  );
}
export function Back({
  to = "/operador",
  children = "Volver al panel",
}: {
  to?: string;
  children?: string;
}) {
  return (
    <Link className="back-link" to={to}>
      <ArrowLeft size={16} />
      {children}
    </Link>
  );
}
export function Busy() {
  return (
    <div className="loading" role="status">
      <span className="spinner" />
      Consultando información…
    </div>
  );
}
export const isCustomer = (v: Customer) =>
  !!v &&
  validID(v.id) &&
  typeof v.name === "string" &&
  typeof v.active === "boolean" &&
  typeof v.licenseValidUntil === "string";
export const isVehicle = (v: Vehicle) =>
  !!v &&
  validID(v.vehicleId) &&
  typeof v.brand === "string" &&
  typeof v.model === "string" &&
  typeof v.category === "string" &&
  typeof v.active === "boolean" &&
  Number.isSafeInteger(v.dailyRateMinor) &&
  v.dailyRateMinor > 0 &&
  v.currency === "ARS" &&
  Number.isInteger(v.version);
export const isReservation = (v: Reservation) =>
  !!v &&
  validID(v.id) &&
  validID(v.vehicleId) &&
  validID(v.customerId) &&
  ["RESERVED", "ACTIVE", "COMPLETED", "CANCELLED"].includes(v.status) &&
  validPrice(v) &&
  typeof v.startsOn === "string" &&
  typeof v.endsOn === "string";
export function checked<T>(v: T, validator: (v: T) => boolean): T {
  if (!validator(v)) throw new ApiError(0, "INVALID_RESPONSE");
  return v;
}
export const uncertain = (err: unknown) =>
  err instanceof ApiError && (err.status === 0 || err.status >= 500);
export function parseRate(text: string) {
  const value = text.trim().replace(",", ".");
  if (!/^\d{1,8}(\.\d{1,2})?$/.test(value))
    throw new Error(
      "Ingresá una tarifa en pesos, con hasta dos decimales y sin separadores de miles.",
    );
  const [whole, fraction = ""] = value.split(".");
  const cents = Number(whole) * 100 + Number(fraction.padEnd(2, "0"));
  if (!Number.isSafeInteger(cents) || cents < 1 || cents > 1000000000)
    throw new Error("La tarifa debe estar entre $0,01 y $10.000.000.");
  return cents;
}

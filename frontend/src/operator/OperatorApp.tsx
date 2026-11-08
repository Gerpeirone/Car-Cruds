import { useCallback, useEffect, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import {
  Link,
  NavLink,
  Navigate,
  Route,
  Routes,
  useLocation,
} from "react-router-dom";
import {
  ArrowRight,
  ArrowUpRight,
  CarFront,
  CalendarDays,
  ClipboardList,
  LayoutDashboard,
  LockKeyhole,
  LogOut,
  Menu,
  ShieldCheck,
  Users,
} from "lucide-react";
import { ApiError, request } from "../api";
import { Action, Heading, Issue, OperatorContext, useOperator } from "./shared";
import type { Call } from "./shared";
import { Fleet, VehicleEditor } from "./Vehicles";
import { Customers, CustomerCreate } from "./Customers";
import { Reservations, Maintenance } from "./Rentals";
import "./operator.css";

export default function OperatorApp() {
  const [token, setToken] = useState<string | null>(null);
  const location = useLocation();
  const call: Call = useCallback(
    async <T,>(path: string, options?: RequestInit) => {
      try {
        return await request<T>(path, token || "", options);
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) setToken(null);
        throw err;
      }
    },
    [token],
  );
  useEffect(() => {
    document.title = "Car Cruds · Panel del operador";
    document.querySelector<HTMLElement>("main h1")?.focus();
    window.scrollTo(0, 0);
  }, [location.pathname, token]);
  const destination =
    typeof location.state?.from === "string" &&
    /^\/operador(\/|$)/.test(location.state.from) &&
    !location.state.from.startsWith("/operador/acceso")
      ? location.state.from
      : "/operador";
  if (!token)
    return (
      <Routes>
        <Route
          path="/operador/acceso"
          element={<OperatorLogin onLogin={setToken} />}
        />
        <Route
          path="*"
          element={
            <Navigate
              to="/operador/acceso"
              replace
              state={{ from: location.pathname + location.search }}
            />
          }
        />
      </Routes>
    );
  return (
    <OperatorContext.Provider value={{ call, logout: () => setToken(null) }}>
      <OperatorShell>
        <Routes>
          <Route path="/operador" element={<Dashboard />} />
          <Route
            path="/operador/acceso"
            element={<Navigate to={destination} replace />}
          />
          <Route path="/operador/flota" element={<Fleet />} />
          <Route
            path="/operador/flota/nuevo"
            element={<VehicleEditor key="new" />}
          />
          <Route path="/operador/flota/:id" element={<VehicleEditor />} />
          <Route path="/operador/clientes" element={<Customers />} />
          <Route path="/operador/clientes/nuevo" element={<CustomerCreate />} />
          <Route path="/operador/reservas" element={<Reservations />} />
          <Route path="/operador/mantenimiento" element={<Maintenance />} />
          <Route path="*" element={<Navigate to="/operador" replace />} />
        </Routes>
      </OperatorShell>
    </OperatorContext.Provider>
  );
}
function OperatorLogin({ onLogin }: { onLogin: (token: string) => void }) {
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>();
  async function submit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(undefined);
    try {
      const session = await request<{ role: string }>(
        "/v1/session",
        token.trim(),
      );
      if (session?.role !== "admin") throw new ApiError(403, "FORBIDDEN");
      onLogin(token.trim());
      setToken("");
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="operator-login">
      <main className="operator-login-card">
        <Link className="brand" to="/acceso">
          <span className="brand-icon">
            <CarFront size={23} />
          </span>
          car<span className="brand-dot">.</span>cruds
        </Link>
        <span className="operator-login-mark">
          <ShieldCheck size={32} />
        </span>
        <span className="eyebrow">OPERACIÓN, BAJO CONTROL</span>
        <h1>
          Todo listo.
          <br />
          <em>Abrimos camino.</em>
        </h1>
        <p className="muted">
          Administrá la flota y acompañá cada viaje, desde el retiro hasta la
          devolución.
        </p>
        <form onSubmit={submit}>
          <label>
            Credencial del operador
            <span className="input-icon">
              <LockKeyhole size={17} />
              <input
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                required
                autoComplete="off"
                aria-describedby="operator-access-hint"
              />
            </span>
          </label>
          <p id="operator-access-hint" className="field-hint">
            Usá la credencial administrativa de laboratorio, sin «Bearer» ni
            comillas. No se guarda en el navegador.
          </p>
          {!!error && <Issue error={error} />}
          <Action type="submit" busy={busy}>
            Entrar al panel <ArrowRight size={18} />
          </Action>
        </form>
        <Link className="back-link" to="/acceso">
          Ir al acceso de clientes <ArrowUpRight size={15} />
        </Link>
      </main>
      <aside className="operator-login-story">
        <span className="eyebrow">CAR CRUDS / OPERACIONES</span>
        <h2>
          Detrás de cada
          <br />
          buen viaje,
          <br />
          <em>todo en orden.</em>
        </h2>
        <div>
          <CarFront size={90} strokeWidth={1} />
          <p>
            Autos preparados.
            <br />
            Clientes acompañados.
            <br />
            Kilómetros por venir.
          </p>
        </div>
      </aside>
    </div>
  );
}
function OperatorShell({ children }: { children: ReactNode }) {
  const { logout } = useOperator();
  const [open, setOpen] = useState(false);
  const location = useLocation();
  useEffect(() => setOpen(false), [location.pathname]);
  const items = [
    ["/operador", "Resumen", LayoutDashboard],
    ["/operador/flota", "Flota", CarFront],
    ["/operador/clientes", "Clientes", Users],
    ["/operador/reservas", "Reservas", ClipboardList],
    ["/operador/mantenimiento", "Mantenimiento", CalendarDays],
  ] as const;
  return (
    <div className="operator-shell">
      <a href="#operator-content" className="skip-link">
        Saltar al contenido
      </a>
      <aside className={`operator-sidebar ${open ? "is-open" : ""}`}>
        <Link to="/operador" className="brand brand-light">
          <span className="brand-icon">
            <CarFront size={23} />
          </span>
          car<span className="brand-dot">.</span>cruds
        </Link>
        <span className="sidebar-caption">PANEL DEL OPERADOR</span>
        <nav aria-label="Navegación del operador">
          {items.map(([path, label, Icon]) => (
            <NavLink key={path} to={path} end={path === "/operador"}>
              <Icon size={18} />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <span>
            <ShieldCheck size={16} />
            Acceso administrativo
          </span>
          <button onClick={logout}>
            <LogOut size={16} />
            Cerrar acceso
          </button>
        </div>
      </aside>
      <div className="operator-main">
        <header className="operator-topbar">
          <button
            className="icon-button operator-menu"
            aria-label="Abrir menú del operador"
            aria-expanded={open}
            onClick={() => setOpen(!open)}
          >
            <Menu />
          </button>
          <span>Un buen día para poner todo en marcha.</span>
          <span className="operator-badge">
            <span /> OPERADOR
          </span>
        </header>
        <main id="operator-content" className="operator-content">
          {children}
        </main>
        <footer className="operator-footer">
          CAR CRUDS · CADA VIAJE EMPIEZA CON UNA BUENA OPERACIÓN.
        </footer>
      </div>
    </div>
  );
}
function Dashboard() {
  const cards = [
    {
      to: "/operador/flota",
      title: "Una flota lista para salir.",
      text: "Registrá autos, actualizá tarifas y administrá su disponibilidad.",
      icon: CarFront,
      label: "Gestionar flota",
    },
    {
      to: "/operador/clientes",
      title: "Personas, antes que kilómetros.",
      text: "Registrá conductores y consultá sus datos y licencia.",
      icon: Users,
      label: "Consultar clientes",
    },
    {
      to: "/operador/reservas",
      title: "Del retiro a la vuelta.",
      text: "Consultá una reserva por su código y acompañá su ciclo.",
      icon: ClipboardList,
      label: "Operar una reserva",
    },
    {
      to: "/operador/mantenimiento",
      title: "Cuidamos el próximo viaje.",
      text: "Bloqueá fechas para preparar y mantener los vehículos.",
      icon: CalendarDays,
      label: "Programar mantenimiento",
    },
  ];
  return (
    <>
      <Heading
        eyebrow="BIENVENIDO AL CENTRO DE OPERACIONES"
        title="Todo en su lugar."
      />
      <section className="operator-welcome">
        <div>
          <span className="eyebrow">TU EQUIPO, EN MOVIMIENTO</span>
          <h2>
            Menos vueltas.
            <br />
            <em>Más control.</em>
          </h2>
          <p>
            Elegí una tarea y seguí cada operación con información actualizada.
          </p>
        </div>
        <CarFront size={125} strokeWidth={0.8} />
      </section>
      <div className="operator-dashboard">
        {cards.map((card) => (
          <Link className="operator-task" key={card.to} to={card.to}>
            <card.icon size={27} />
            <h2>{card.title}</h2>
            <p>{card.text}</p>
            <span>
              {card.label}
              <ArrowUpRight size={18} />
            </span>
          </Link>
        ))}
      </div>
    </>
  );
}

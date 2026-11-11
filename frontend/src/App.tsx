import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import type { FormEvent, ReactNode } from "react";
import {
  Link,
  NavLink,
  Navigate,
  Route,
  Routes,
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import {
  ArrowUpRight,
  ArrowRight,
  ArrowLeft,
  Search,
  SlidersHorizontal,
  Users,
  Cog,
  Fuel,
  Check,
  CheckCheck,
  ShieldCheck,
  LogOut,
  MapPin,
  CalendarDays,
  X,
  AlertCircle,
  RefreshCw,
  Copy,
  LockKeyhole,
  ChevronRight,
  ChevronLeft,
  Leaf,
  Ticket,
  Menu,
  CarFront,
} from "lucide-react";
import { ApiError, errorMessage, request } from "./api";
import {
  clearPending,
  dateError,
  dateLabel,
  money,
  readPending,
  savePending,
  statusLabels,
  today,
  validID,
  validPrice,
} from "./domain";
import type {
  Availability,
  Customer,
  Pending,
  Quote,
  Reservation,
  Vehicle,
  VehiclePage,
} from "./domain";
import CarArt from "./CarArt";
import OperatorApp from "./operator/OperatorApp";
import { AccountPortal } from "./AccountPortal";
import type { Account } from "./AccountPortal";

type Call = <T>(path: string, init?: RequestInit) => Promise<T>;
interface Access {
  customer: Customer;
  call: Call;
  logout: () => void;
  quote: Quote | null;
  setQuote: (q: Quote | null) => void;
  personal?: boolean;
}
const AccessContext = createContext<Access | null>(null);
function useAccess() {
  const value = useContext(AccessContext);
  if (!value) throw new Error("Acceso requerido");
  return value;
}
const isCustomer = (v: Customer) =>
  v &&
  validID(v.id) &&
  typeof v.name === "string" &&
  typeof v.active === "boolean" &&
  typeof v.licenseValidUntil === "string";
const isVehicle = (v: Vehicle) =>
  v &&
  validID(v.vehicleId) &&
  typeof v.brand === "string" &&
  typeof v.model === "string" &&
  typeof v.active === "boolean" &&
  Number.isSafeInteger(v.dailyRateMinor) &&
  v.dailyRateMinor > 0 &&
  v.currency === "ARS";
const isPage = (v: VehiclePage) =>
  v &&
  Array.isArray(v.items) &&
  v.items.every(isVehicle) &&
  Number.isSafeInteger(v.total) &&
  v.total >= 0;
const isReservation = (v: Reservation) =>
  v &&
  validID(v.id) &&
  validID(v.vehicleId) &&
  validID(v.customerId) &&
  Object.hasOwn(statusLabels, v.status) &&
  validPrice(v) &&
  typeof v.startsOn === "string" &&
  typeof v.endsOn === "string";
const isAvailability = (v: Availability) =>
  v &&
  validID(v.vehicleId) &&
  typeof v.available === "boolean" &&
  validPrice(v) &&
  Number.isInteger(v.days) &&
  v.days >= 1 &&
  v.days <= 30;
function checked<T>(v: T, validate: (v: T) => boolean): T {
  if (!validate(v)) throw new ApiError(0, "INVALID_RESPONSE");
  return v;
}

function useResource<T>(path: string, validate: (v: T) => boolean) {
  const { call } = useAccess();
  const [state, setState] = useState<{
    data?: T;
    error?: unknown;
    loading: boolean;
  }>({ loading: true });
  const [iteration, setIteration] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    setState({ loading: true });
    call<T>(path, { signal: controller.signal })
      .then((data) => {
        if (active) setState({ loading: false, data: checked(data, validate) });
      })
      .catch((error) => {
        if (active) setState({ loading: false, error });
      });
    return () => {
      active = false;
      controller.abort();
    };
  }, [path, call, iteration, validate]);
  return { ...state, retry: () => setIteration((n) => n + 1) };
}

function Brand({ light = false }: { light?: boolean }) {
  return (
    <Link
      to="/autos"
      className={`brand ${light ? "brand-light" : ""}`}
      aria-label="Car Cruds, inicio"
    >
      <span className="brand-icon">
        <CarFront size={23} strokeWidth={2.3} />
      </span>
      <span>
        car<span className="brand-dot">.</span>cruds
      </span>
    </Link>
  );
}
function Button({
  children,
  busy = false,
  className = "",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { busy?: boolean }) {
  return (
    <button
      {...props}
      className={`button ${className}`}
      disabled={busy || props.disabled}
      aria-busy={busy || undefined}
    >
      {busy ? <span className="spinner" /> : null}
      {children}
    </button>
  );
}
function ErrorBox({ error, retry }: { error: unknown; retry?: () => void }) {
  return (
    <div className="notice error" role="alert">
      <AlertCircle size={20} />
      <div>
        <p>{errorMessage(error)}</p>
        {error instanceof ApiError && error.requestID && (
          <>
            <button
              className="text-button"
              onClick={() =>
                navigator.clipboard?.writeText(error.requestID).catch(() => {})
              }
            >
              Copiar referencia de soporte <Copy size={13} />
            </button>
            <p className="small">
              Esta referencia identifica el error; no es un código de reserva.
            </p>
          </>
        )}
        {retry && (
          <button className="text-button" onClick={retry}>
            Volver a intentar <RefreshCw size={14} />
          </button>
        )}
      </div>
    </div>
  );
}
function Loading() {
  return (
    <div className="loading" role="status">
      <span className="spinner" />
      Buscando el mejor camino…
    </div>
  );
}
function PageHeading({
  eyebrow,
  title,
  children,
}: {
  eyebrow: string;
  title: string;
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
function useBusy() {
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

export default function App() {
  const [session, setSession] = useState<{
    customer: Customer;
    token: string;
  } | null>(null);
  const [quote, setQuote] = useState<Quote | null>(null);
  const location = useLocation();
  useEffect(() => {
    if (location.pathname.startsWith("/operador")) {
      setSession(null);
      setQuote(null);
    }
  }, [location.pathname]);
  useEffect(() => {
    document.title = `Car Cruds · ${location.pathname.startsWith("/operador") ? "Panel del operador" : location.pathname.startsWith("/reservas") ? "Tu reserva" : location.pathname === "/acceso" ? "Bienvenido" : "Tu próximo viaje"}`;
    document.querySelector<HTMLElement>("main h1")?.focus();
    window.scrollTo(0, 0);
  }, [location.pathname]);
  const call: Call = useCallback(
    async <T,>(path: string, init?: RequestInit) => {
      try {
        return await request<T>(path, session?.token || "", init);
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          setSession((current) =>
            current?.token === session?.token ? null : current,
          );
          setQuote(null);
        }
        throw err;
      }
    },
    [session?.token],
  );
  if (location.pathname.startsWith("/operador")) return <OperatorApp />;
  if (location.pathname === "/" || /^\/(registro|iniciar-sesion|perfil|catalogo)(\/|$)/.test(location.pathname))
    return <AccountPortal catalog={(account, expire) => <PersonalCatalog account={account} expire={expire} />} />;
  if (!session)
    return (
      <Routes>
        <Route
          path="/acceso"
          element={
            <Login
              onLogin={(customer, token) => {
                setQuote(null);
                setSession({ customer, token });
              }}
            />
          }
        />
        <Route
          path="*"
          element={
            <Navigate
              to="/acceso"
              replace
              state={{ from: location.pathname + location.search }}
            />
          }
        />
      </Routes>
    );
  return (
    <AccessContext.Provider
      value={{
        customer: session.customer,
        call,
        logout: () => {
          setSession(null);
          setQuote(null);
        },
        quote,
        setQuote,
      }}
    >
      <Shell>
        <Routes>
          <Route path="/autos" element={<Catalog />} />
          <Route path="/autos/:vehicleId" element={<VehicleDetail />} />
          <Route path="/reservar" element={<Checkout />} />
          <Route path="/reservas/consultar" element={<Lookup />} />
          <Route
            path="/reservas/:reservationId"
            element={<ReservationDetail />}
          />
          <Route
            path="/acceso"
            element={
              <Navigate
                to={
                  typeof location.state?.from === "string" &&
                  /^\/(autos|reservar|reservas)(\/|\?|$)/.test(
                    location.state.from,
                  )
                    ? location.state.from
                    : "/autos"
                }
                replace
              />
            }
          />
          <Route path="/" element={<Navigate to="/autos" replace />} />
          <Route path="*" element={<NotFound />} />
        </Routes>
      </Shell>
    </AccessContext.Provider>
  );
}

function PersonalCatalog({ account, expire }: { account: Account; expire: () => void }) {
  const call: Call = useCallback(async <T,>(path: string, init?: RequestInit) => {
    try {
      return await request<T>(path, "", init);
    } catch (err) {
      if (!(err instanceof ApiError && err.status === 401) || init?.signal?.aborted) throw err;
      // Only the session endpoint decides that the session ended. A 401 from a
      // catalog read can come from a misrouted or degraded backend; then the
      // page reports a service problem and keeps the (still valid) session.
      try {
        await request("/v1/auth/me", "", { signal: init?.signal });
      } catch (check) {
        if (check instanceof ApiError && check.status === 401) {
          expire();
          throw err;
        }
        throw check;
      }
      throw new ApiError(503, "SERVICE_UNAVAILABLE", err.requestID);
    }
  }, [expire]);
  return <AccessContext.Provider value={{
    personal: true,
    customer: { id: account.id, name: account.firstName, active: false, licenseValidUntil: "" },
    call, quote: null, setQuote: () => {}, logout: expire,
  }}>
    <Routes>
      <Route path="/catalogo" element={<Catalog />} />
      <Route path="/catalogo/:vehicleId" element={<VehicleDetail />} />
      <Route path="*" element={<Navigate to="/catalogo" replace />} />
    </Routes>
  </AccessContext.Provider>;
}

function Login({ onLogin }: { onLogin: (c: Customer, t: string) => void }) {
  const [id, setId] = useState("");
  const [token, setToken] = useState("");
  const [error, setError] = useState<unknown>();
  const { busy, run } = useBusy();
  const navigate = useNavigate();
  const location = useLocation();
  function submit(e: FormEvent) {
    e.preventDefault();
    setError(undefined);
    void run(async () => {
      try {
        if (!validID(id.trim()))
          throw new Error(
            "Ingresá el identificador de cliente que te asignó el equipo.",
          );
        const customer = checked(
          await request<Customer>(
            `/v1/customers/${encodeURIComponent(id.trim())}`,
            token.trim(),
          ),
          isCustomer,
        );
        if (customer.id !== id.trim())
          throw new ApiError(0, "INVALID_RESPONSE");
        onLogin(customer, token.trim());
        setToken("");
        const from = location.state?.from;
        navigate(
          typeof from === "string" &&
            /^\/(autos|reservar|reservas)(\/|\?|$)/.test(from)
            ? from
            : "/autos",
          { replace: true },
        );
      } catch (err) {
        setError(err);
      }
    });
  }
  return (
    <div className="access-page">
      <section className="access-story">
        <Brand light />
        <div className="access-copy">
          <span className="eyebrow">
            <span className="live-dot" /> TU VIAJE, A TU MANERA
          </span>
          <h1>
            Menos vueltas.
            <br />
            Más <em>camino.</em>
          </h1>
          <p>
            Encontrá el auto que va con vos.
            <br />
            Nosotros ponemos las ruedas, vos el destino.
          </p>
        </div>
        <div className="access-art">
          <span className="art-orbit" />
          <CarArt hero />
          <span className="art-caption">
            LIBERTAD EN CADA KILÓMETRO <ArrowUpRight size={18} />
          </span>
        </div>
        <div className="access-bottom">
          <span>Hecho para moverte.</span>
          <span>
            Argentina <MapPin size={14} />
          </span>
        </div>
      </section>
      <main className="access-form">
        <div className="access-form-inner">
          <div className="section-number">01 / EMPECEMOS</div>
          <span className="access-icon">
            <ArrowUpRight size={28} />
          </span>
          <h2>
            Tu próximo viaje
            <br />
            te está esperando.
          </h2>
          <p className="muted">
            Ingresá con tu acceso de cliente para explorar la flota y gestionar
            tus reservas.
          </p>
          <form onSubmit={submit}>
            <label>
              Identificador de cliente
              <input
                name="customerId"
                autoComplete="username"
                placeholder="Ej. cli-001"
                value={id}
                onChange={(e) => setId(e.target.value)}
                required
                maxLength={64}
              />
            </label>
            <label>
              Credencial de acceso
              <span className="input-icon">
                <LockKeyhole size={17} />
                <input
                  name="credential"
                  type="password"
                  aria-describedby="credential-hint"
                  autoComplete="off"
                  placeholder="Tu credencial de cliente"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  required
                />
              </span>
            </label>
            <p id="credential-hint" className="field-hint">
              Pegá solo la credencial de cliente, sin agregar «Bearer» ni
              comillas.
            </p>
            {!!error && <ErrorBox error={error} />}
            <Button type="submit" busy={busy}>
              Empezar mi viaje <ArrowRight size={18} />
            </Button>
          </form>
          <div className="access-help">
            <ShieldCheck size={19} />
            <p>
              Acceso de laboratorio. Usá los datos que te entregó el equipo. Tu
              credencial no se guarda en este navegador.
            </p>
          </div>
          <Link to="/operador/acceso" className="back-link">
            Soy operador <ArrowUpRight size={15} />
          </Link>
        </div>
        <span className="access-footnote">
          CAR CRUDS · ALQUILER DE VEHÍCULOS
        </span>
      </main>
    </div>
  );
}

function Shell({ children }: { children: ReactNode }) {
  const { customer, logout } = useAccess();
  const [menu, setMenu] = useState(false);
  const location = useLocation();
  useEffect(() => setMenu(false), [location.pathname]);
  let pending: Pending | null = null;
  let pendingError: unknown;
  try {
    pending = readPending(customer.id);
  } catch (err) {
    pendingError = err;
  }
  return (
    <div className="app-shell">
      <a href="#contenido" className="skip-link">
        Saltar al contenido
      </a>
      <header className="topbar">
        <Brand />
        <nav
          aria-label="Navegación principal"
          className={menu ? "nav-open" : ""}
        >
          <NavLink
            to="/autos"
            className={({ isActive }) => (isActive ? "active" : "")}
          >
            <CarFront size={17} />
            Explorar autos
          </NavLink>
          <NavLink
            to="/reservas/consultar"
            className={() =>
              location.pathname.startsWith("/reservas") ? "active" : ""
            }
          >
            <Ticket size={17} />
            Consultar reserva
          </NavLink>
        </nav>
        <div className="profile">
          <span className="avatar">
            {customer.name.slice(0, 1).toUpperCase()}
          </span>
          <span className="profile-name">
            {customer.name}
            <small>Tu espacio de viaje</small>
          </span>
          <button
            className="icon-button"
            onClick={logout}
            aria-label="Cerrar acceso"
          >
            <LogOut size={18} />
          </button>
        </div>
        <button
          className="icon-button mobile-menu"
          aria-label="Abrir navegación"
          aria-expanded={menu}
          onClick={() => setMenu(!menu)}
        >
          <Menu />
        </button>
      </header>
      <main id="contenido" className="main-content">
        {(!!pending || !!pendingError) && (
          <div className="notice warning">
            <AlertCircle size={20} />
            <div>
              {pendingError
                ? errorMessage(pendingError)
                : "Tenés una reserva cuyo resultado todavía debe confirmarse."}{" "}
              {pending && (
                <Link to="/reservar">
                  Recuperar resultado <ArrowRight size={14} />
                </Link>
              )}
            </div>
          </div>
        )}
        {!customer.active && (
          <div className="notice warning">
            <AlertCircle size={18} />
            Tu perfil está inactivo. Podés consultar tus reservas; contactá al
            equipo para habilitar un nuevo alquiler.
          </div>
        )}
        {children}
      </main>
      <footer className="footer">
        <Brand />
        <span>Tu destino. Tus tiempos. Tu camino.</span>
        <span>
          Hecho en Argentina <span className="footer-dot" />
        </span>
      </footer>
    </div>
  );
}

function Catalog() {
  const [params, setParams] = useSearchParams();
  const [filters, setFilters] = useState(false);
  const [fallback, setFallback] = useState(false);
  const query = new URLSearchParams(params);
  query.set("pageSize", "6");
  if (fallback) query.delete("q");
  const { data, error, loading, retry } = useResource<VehiclePage>(
    `/v1/vehicles${fallback ? "" : "/search"}?${query}`,
    isPage,
  );
  const page = Number(params.get("page") || 1);
  const [search, setSearch] = useState(params.get("q") || "");
  useEffect(() => {
    setSearch(params.get("q") || "");
    setFallback(false);
  }, [params]);
  function update(name: string, value: string) {
    const next = new URLSearchParams(params);
    if (value) next.set(name, value);
    else next.delete(name);
    if (name !== "page") next.delete("page");
    setParams(next);
  }
  return (
    <>
      <section className="catalog-hero">
        <div className="hero-copy">
          <span className="eyebrow">
            <span className="live-dot" /> MOVETE A TU MANERA
          </span>
          <h1 tabIndex={-1}>
            Tu próximo viaje
            <br />
            empieza <em>acá.</em>
          </h1>
          <p>
            Un auto para cada plan. Encontrá el tuyo
            <br className="desktop-break" /> y elegí cuándo empieza la aventura.
          </p>
          <span className="hero-note">
            <ShieldCheck size={17} />
            Precio claro. Reserva a tu ritmo.
          </span>
        </div>
        <div className="hero-visual">
          <div className="hero-ring" />
          <span className="hero-type">GO.</span>
          <CarArt hero />
          <span className="visual-note">
            ILUSTRACIÓN REFERENCIAL <ArrowUpRight size={13} />
          </span>
        </div>
        <span className="hero-index">01 — EXPLORÁ</span>
      </section>
      <section className="fleet-section" aria-labelledby="fleet-title">
        <div className="fleet-heading">
          <div>
            <span className="eyebrow">ENCONTRÁ TU PRÓXIMO AUTO</span>
            <h2 id="fleet-title">
              Elegí cómo querés moverte<span className="brand-dot">.</span>
            </h2>
          </div>
          <span className="results-count" aria-live="polite">
            {loading
              ? "Consultando flota…"
              : data
                ? `${data.total} opciones en la flota`
                : "Explorá la flota"}
          </span>
        </div>
        <form
          className="search-bar"
          onSubmit={(e) => {
            e.preventDefault();
            update("q", search.trim());
          }}
        >
          <div className="search-field">
            <Search size={20} />
            <input
              aria-label="Buscar por marca o modelo"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="¿Qué auto tenés en mente?"
              maxLength={256}
            />
          </div>
          <Button type="submit">
            Buscar <ArrowRight size={17} />
          </Button>
          <button
            type="button"
            className={`filter-toggle ${filters ? "selected" : ""}`}
            onClick={() => setFilters(!filters)}
            aria-expanded={filters}
          >
            <SlidersHorizontal size={18} />
            Filtros
            {["brand", "category", "seats", "transmission"].some((k) =>
              params.has(k),
            ) && <span className="filter-dot" />}
          </button>
          <label className="sort-label">
            <span className="sr-only">Ordenar por precio</span>
            <select
              value={params.get("sort") || "price_asc"}
              onChange={(e) => update("sort", e.target.value)}
            >
              <option value="price_asc">Menor precio primero</option>
              <option value="price_desc">Mayor precio primero</option>
            </select>
          </label>
        </form>
        {filters && (
          <form
            className="filter-panel"
            onSubmit={(e) => {
              e.preventDefault();
              const form = new FormData(e.currentTarget);
              const next = new URLSearchParams(params);
              for (const key of [
                "brand",
                "category",
                "transmission",
                "seats",
              ]) {
                const value = String(form.get(key) || "").trim();
                if (value) next.set(key, value);
                else next.delete(key);
              }
              next.delete("page");
              setParams(next);
            }}
          >
            <label>
              Marca exacta
              <input
                name="brand"
                maxLength={128}
                defaultValue={params.get("brand") || ""}
                placeholder="Ej. Toyota"
              />
            </label>
            <label>
              Categoría exacta
              <input
                name="category"
                maxLength={64}
                defaultValue={params.get("category") || ""}
                placeholder="Ej. sedan"
              />
            </label>
            <label>
              Transmisión
              <select
                name="transmission"
                defaultValue={params.get("transmission") || ""}
              >
                <option value="">Todas</option>
                <option value="automatic">Automática</option>
                <option value="manual">Manual</option>
              </select>
            </label>
            <label>
              Plazas
              <select name="seats" defaultValue={params.get("seats") || ""}>
                <option value="">Todas</option>
                {Array.from({ length: 9 }, (_, i) => (
                  <option key={i + 1} value={i + 1}>
                    {i + 1} plazas
                  </option>
                ))}
              </select>
            </label>
            <Button type="submit">Aplicar</Button>
            <button
              type="button"
              className="text-button"
              onClick={() => setParams({})}
            >
              Limpiar
            </button>
          </form>
        )}
        {fallback && (
          <div className="notice info">
            <Leaf size={19} />
            Estás viendo el catálogo sin búsqueda por texto. Los demás filtros
            se mantienen.
            <button className="text-button" onClick={() => setFallback(false)}>
              Volver a búsqueda
            </button>
          </div>
        )}
        {loading ? (
          <div
            className="skeleton-grid"
            aria-label="Cargando flota"
            role="status"
          >
            {[1, 2, 3].map((n) => (
              <div className="skeleton-card" key={n} />
            ))}
          </div>
        ) : error ? (
          <>
            <ErrorBox error={error} retry={retry} />
            {error instanceof ApiError &&
              error.code === "SEARCH_UNAVAILABLE" &&
              !fallback && (
                <Button className="secondary" onClick={() => setFallback(true)}>
                  Ver catálogo sin búsqueda <ArrowRight size={16} />
                </Button>
              )}
          </>
        ) : data && data.items.length ? (
          <>
            <div className="vehicle-grid">
              {data.items.map((vehicle, i) => (
                <VehicleCard
                  vehicle={vehicle}
                  index={i}
                  key={vehicle.vehicleId}
                />
              ))}
            </div>
            <div className="pagination">
              <span>
                Página {page} de {Math.max(1, Math.ceil(data.total / 6))}
              </span>
              <div>
                <button
                  className="icon-button"
                  disabled={page <= 1}
                  aria-label="Página anterior"
                  onClick={() => update("page", String(page - 1))}
                >
                  <ChevronLeft size={18} />
                </button>
                <span className="page-number">{page}</span>
                <button
                  className="icon-button"
                  disabled={page * 6 >= data.total}
                  aria-label="Página siguiente"
                  onClick={() => update("page", String(page + 1))}
                >
                  <ChevronRight size={18} />
                </button>
              </div>
            </div>
          </>
        ) : (
          <div className="empty-state">
            <Search size={32} />
            <h3>No encontramos autos con esos filtros.</h3>
            <p>Probá otra marca, categoría o una búsqueda más amplia.</p>
            <Button className="secondary" onClick={() => setParams({})}>
              Limpiar búsqueda
            </Button>
          </div>
        )}
      </section>
      <section className="journey-strip">
        <div className="strip-icon">
          <ArrowUpRight size={26} />
        </div>
        <div>
          <h3>El plan lo ponés vos.</h3>
          <p>Elegí tu auto, consultá las fechas y reservá en pocos pasos.</p>
        </div>
        <span>EL RESTO ES CAMINO.</span>
      </section>
    </>
  );
}

function VehicleCard({
  vehicle: v,
  index,
}: {
  vehicle: Vehicle;
  index: number;
}) {
  const { personal } = useAccess();
  const detailPath = `${personal ? "/catalogo" : "/autos"}/${encodeURIComponent(v.vehicleId)}`;
  return (
    <article className="vehicle-card">
      <Link
        className={`vehicle-picture tone-${index % 3}`}
        to={detailPath}
        tabIndex={-1}
        aria-hidden="true"
      >
        <span className="category-tag">{v.category}</span>
        <CarArt tone={["silver", "green", "sand"][index % 3]} />
        <span className="illustration-label">Ilustración referencial</span>
        <span className="picture-arrow">
          <ArrowUpRight size={20} />
        </span>
      </Link>
      <div className="vehicle-card-body">
        <span className="vehicle-brand">{v.brand}</span>
        <h3>
          <Link to={detailPath}>
            {v.model}
          </Link>
        </h3>
        <div className="specs">
          <span>
            <Users size={15} />
            {v.seats} plazas
          </span>
          <span>
            <Cog size={15} />
            {v.transmission === "automatic" ? "Automático" : "Manual"}
          </span>
        </div>
        <div className="vehicle-card-bottom">
          <div>
            <small>Tarifa orientativa</small>
            <strong>
              {money(v.dailyRateMinor)}
              <span> / día</span>
            </strong>
          </div>
          <Link
            className="round-link"
            aria-label={`Ver ${v.brand} ${v.model}`}
            to={detailPath}
          >
            <ArrowRight size={20} />
          </Link>
        </div>
      </div>
    </article>
  );
}

function VehicleDetail() {
  const { vehicleId = "" } = useParams();
  const {
    data: v,
    error,
    loading,
    retry,
  } = useResource<Vehicle>(
    `/v1/vehicles/${encodeURIComponent(vehicleId)}`,
    isVehicle,
  );
  const { call, setQuote, customer, personal } = useAccess();
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [availability, setAvailability] = useState<Availability | null>(null);
  const [quoteError, setQuoteError] = useState<unknown>();
  const { busy, run } = useBusy();
  const generation = useRef(0);
  const navigate = useNavigate();
  useEffect(
    () => () => {
      generation.current++;
    },
    [],
  );
  function changeDates(nextStart: string, nextEnd: string) {
    generation.current++;
    setStart(nextStart);
    setEnd(nextEnd);
    setAvailability(null);
    setQuote(null);
    setQuoteError(undefined);
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    setQuoteError(undefined);
    const error = dateError(start, end);
    if (error) {
      setQuoteError(new Error(error));
      return;
    }
    const version = ++generation.current;
    void run(async () => {
      try {
        const q = checked(
          await call<Availability>(
            `/v1/availability?${new URLSearchParams({ vehicleId, startsOn: start, endsOn: end })}`,
          ),
          isAvailability,
        );
        if (version !== generation.current) return;
        if (
          q.vehicleId !== vehicleId ||
          q.startsOn !== start ||
          q.endsOn !== end
        )
          throw new ApiError(0, "INVALID_RESPONSE");
        setAvailability(q);
      } catch (err) {
        if (version === generation.current) setQuoteError(err);
      }
    });
  }
  if (loading) return <Loading />;
  if (error) return <ErrorBox error={error} retry={retry} />;
  if (!v) return null;
  return (
    <>
      <Link to={personal ? "/catalogo" : "/autos"} className="back-link">
        <ArrowLeft size={16} />
        Volver a la flota
      </Link>
      <div className="detail-grid">
        <section>
          <span className="eyebrow">TU COMPAÑERO DE RUTA</span>
          <h1 tabIndex={-1} className="detail-title">
            {v.brand}
            <br />
            <span>{v.model}</span>
          </h1>
          <div className="detail-car">
            <span className="category-tag">{v.category}</span>
            <CarArt hero />
            <span className="illustration-label">
              Ilustración referencial · Consultá las características del
              vehículo
            </span>
          </div>
          <div className="detail-specs">
            <div>
              <Users />
              <strong>{v.seats} plazas</strong>
              <span>Espacio para tu plan</span>
            </div>
            <div>
              <Cog />
              <strong>
                {v.transmission === "automatic" ? "Automático" : "Manual"}
              </strong>
              <span>Transmisión</span>
            </div>
            <div>
              <CarFront />
              <strong>{v.category}</strong>
              <span>Categoría</span>
            </div>
          </div>
          {!personal && <div className="detail-note">
            <ShieldCheck size={22} />
            <p>
              Tu reserva, con el precio acordado.
              <span>
                El total se confirma según las fechas elegidas y queda congelado
                al reservar.
              </span>
            </p>
          </div>}
        </section>
        <aside className="booking-panel">
          <span className="eyebrow">ARMÁ TU VIAJE</span>
          <div className="panel-price">
            <strong>{money(v.dailyRateMinor)}</strong>
            <span>/ día orientativo</span>
          </div>
          {personal ? <p className="muted">Para reservar necesitás una credencial de alquiler.</p> : <>
          <p className="muted">Elegí tus fechas y consultá disponibilidad.</p>
          {!v.active ? (
            <div className="notice warning">
              Este auto no está disponible para nuevas reservas.
            </div>
          ) : (
            <form onSubmit={submit}>
              <label>
                Retiro
                <span className="input-icon">
                  <CalendarDays size={17} />
                  <input
                    aria-label="Fecha de retiro"
                    type="date"
                    value={start}
                    min={today()}
                    onChange={(e) => changeDates(e.target.value, end)}
                    required
                  />
                </span>
              </label>
              <label>
                Devolución
                <span className="input-icon">
                  <CalendarDays size={17} />
                  <input
                    aria-label="Fecha de devolución"
                    type="date"
                    value={end}
                    min={start || today()}
                    onChange={(e) => changeDates(start, e.target.value)}
                    required
                  />
                </span>
              </label>
              <p className="field-hint">
                De 1 a 30 días. El día de devolución no se cobra.
              </p>
              {!!quoteError && <ErrorBox error={quoteError} />}
              <Button className="secondary full" type="submit" busy={busy}>
                Consultar disponibilidad <ArrowRight size={17} />
              </Button>
            </form>
          )}
          {availability && (
            <div className="quote-result" aria-live="polite">
              {availability.available ? (
                <>
                  <span className="available">
                    <Check size={17} />
                    Disponible para tu viaje
                  </span>
                  <div className="price-row">
                    <span>
                      {availability.days}{" "}
                      {availability.days === 1 ? "día" : "días"} de alquiler
                    </span>
                    <span>{money(availability.dailyRateMinor)} / día</span>
                  </div>
                  <div className="price-row total">
                    <strong>Total</strong>
                    <strong>{money(availability.totalMinor)}</strong>
                  </div>
                  <Button
                    className="full"
                    disabled={!customer.active}
                    onClick={() => {
                      setQuote({ vehicle: v, availability });
                      navigate("/reservar");
                    }}
                  >
                    Revisar mi reserva <ArrowUpRight size={18} />
                  </Button>
                  <p className="field-hint">
                    Consultar no bloquea el auto. Se vuelve a validar al
                    confirmar.
                  </p>
                </>
              ) : (
                <div className="notice warning">
                  <AlertCircle size={18} />
                  El auto no está disponible en ese período. Probá otras fechas.
                </div>
              )}
            </div>
          )}
          <div className="panel-trust">
            <LockKeyhole size={15} />
            Confirmás después de revisar el total.
          </div>
          </>}
        </aside>
      </div>
    </>
  );
}

function Checkout() {
  const { customer, quote, setQuote, call } = useAccess();
  const navigate = useNavigate();
  const { busy, run } = useBusy();
  const [error, setError] = useState<unknown>();
  const [pending, setPending] = useState<Pending | null>(() => {
    try {
      return readPending(customer.id);
    } catch {
      return null;
    }
  });
  const [uncertain, setUncertain] = useState(false);
  const [storageError, setStorageError] = useState<unknown>(() => {
    try {
      readPending(customer.id);
      return undefined;
    } catch (err) {
      return err;
    }
  });
  const [confirmed, setConfirmed] = useState<string>();
  async function send(operation: Pending) {
    try {
      const reservation = checked(
        await call<Reservation>("/v1/reservations", {
          method: "POST",
          body: JSON.stringify(operation.body),
          headers: { "Idempotency-Key": operation.key },
        }),
        isReservation,
      );
      if (
        reservation.customerId !== customer.id ||
        reservation.vehicleId !== operation.body.vehicleId ||
        reservation.startsOn !== operation.body.startsOn ||
        reservation.endsOn !== operation.body.endsOn
      )
        throw new ApiError(0, "INVALID_RESPONSE");
      setConfirmed(reservation.id);
      try {
        clearPending(customer.id);
      } catch {
        setStorageError(
          new Error(
            "La reserva ya está confirmada, pero no se pudo limpiar la referencia local. Conservá su código.",
          ),
        );
        return;
      }
      setPending(null);
      setQuote(null);
      navigate(`/reservas/${encodeURIComponent(reservation.id)}`, {
        replace: true,
        state: { created: true },
      });
    } catch (err) {
      setError(err);
      const definitive =
        err instanceof ApiError &&
        [400, 404, 409, 415, 422].includes(err.status) &&
        err.code !== "IDEMPOTENCY_KEY_REUSED";
      if (definitive) {
        try {
          clearPending(customer.id);
          setPending(null);
          setQuote(null);
        } catch (storage) {
          setStorageError(storage);
        }
      } else {
        setUncertain(true);
      }
    }
  }
  function confirm() {
    setError(undefined);
    void run(async () => {
      if (confirmed) return;
      let existing: Pending | null;
      try {
        existing = readPending(customer.id);
      } catch (err) {
        setStorageError(err);
        return;
      }
      if (existing) {
        setPending(existing);
        setUncertain(true);
        return;
      }
      if (!quote) return;
      if (!customer.active) {
        setError(
          new Error(
            "Tu perfil está inactivo. Contactá al equipo para habilitar un nuevo alquiler.",
          ),
        );
        return;
      }
      const validation = dateError(
        quote.availability.startsOn,
        quote.availability.endsOn,
      );
      if (validation) {
        setError(new Error(validation));
        setQuote(null);
        return;
      }
      const operation: Pending = {
        version: 1,
        key: crypto.randomUUID(),
        body: {
          vehicleId: quote.vehicle.vehicleId,
          customerId: customer.id,
          startsOn: quote.availability.startsOn,
          endsOn: quote.availability.endsOn,
          expectedTotalMinor: quote.availability.totalMinor,
        },
      };
      try {
        savePending(operation);
        setPending(operation);
      } catch {
        setStorageError(
          new Error(
            "No se pudo guardar la referencia de recuperación en esta pestaña. No enviamos la reserva. Habilitá el almacenamiento del navegador antes de continuar.",
          ),
        );
        return;
      }
      await send(operation);
    });
  }
  const recovery = pending;
  return (
    <>
      <Link
        to={quote ? `/autos/${quote.vehicle.vehicleId}` : "/autos"}
        className="back-link"
      >
        <ArrowLeft size={16} />
        Volver a elegir
      </Link>
      <PageHeading
        eyebrow="UN PASO MÁS Y ESTÁS EN CAMINO"
        title={recovery ? "Recuperá tu reserva." : "Todo listo para salir."}
      />
      {confirmed ? (
        <div className="success-card">
          <CheckCheck size={36} />
          <h2>Tu reserva fue confirmada.</h2>
          <p>
            Conservá este código: <strong>{confirmed}</strong>
          </p>
          {!!storageError && <ErrorBox error={storageError} />}
          <Link className="button" to={`/reservas/${confirmed}`}>
            Consultar reserva <ArrowRight size={17} />
          </Link>
        </div>
      ) : storageError ? (
        <ErrorBox error={storageError} />
      ) : recovery ? (
        <section className="recovery-card">
          <div className="recovery-icon">
            <RefreshCw size={28} />
          </div>
          <h2>
            {uncertain
              ? "Todavía no sabemos si se confirmó."
              : "Hay una operación pendiente de confirmar."}
          </h2>
          <p>
            Vamos a consultar su resultado repitiendo la misma operación, con la
            misma referencia. No vuelvas a crear otra reserva para resolver este
            intento.
          </p>
          <div className="summary-facts">
            <span>
              Vehículo<strong>{recovery.body.vehicleId}</strong>
            </span>
            <span>
              Retiro<strong>{dateLabel(recovery.body.startsOn)}</strong>
            </span>
            <span>
              Devolución<strong>{dateLabel(recovery.body.endsOn)}</strong>
            </span>
            <span>
              Total esperado
              <strong>{money(recovery.body.expectedTotalMinor)}</strong>
            </span>
          </div>
          {!!error && <ErrorBox error={error} />}
          <Button
            busy={busy}
            disabled={
              error instanceof ApiError &&
              error.code === "IDEMPOTENCY_KEY_REUSED"
            }
            onClick={() => {
              setError(undefined);
              void run(() => send(recovery));
            }}
          >
            Recuperar resultado <RefreshCw size={17} />
          </Button>
          <p className="field-hint">
            Conservá esta pestaña y sus datos hasta resolver el resultado. No
            guardamos tu credencial.
          </p>
        </section>
      ) : quote ? (
        <div className="checkout-grid">
          <section className="checkout-car">
            <div className="checkout-art">
              <CarArt hero />
            </div>
            <span className="eyebrow">TU ELECCIÓN</span>
            <h2>
              {quote.vehicle.brand} {quote.vehicle.model}
            </h2>
            <div className="specs">
              <span>
                <Users size={17} />
                {quote.vehicle.seats} plazas
              </span>
              <span>
                <Cog size={17} />
                {quote.vehicle.transmission === "automatic"
                  ? "Automático"
                  : "Manual"}
              </span>
            </div>
            <div className="summary-facts">
              <span>
                Retiro<strong>{dateLabel(quote.availability.startsOn)}</strong>
              </span>
              <span>
                Devolución
                <strong>{dateLabel(quote.availability.endsOn)}</strong>
              </span>
            </div>
          </section>
          <section className="booking-panel">
            <span className="eyebrow">RESUMEN DE TU RESERVA</span>
            <h2>Todo claro, desde el inicio.</h2>
            <div className="customer-line">
              <span className="avatar">{customer.name[0]}</span>
              <div>
                <strong>{customer.name}</strong>
                <span>Conductor registrado</span>
              </div>
            </div>
            <div className="price-row">
              <span>{quote.availability.days} días de alquiler</span>
              <strong>{money(quote.availability.dailyRateMinor)} / día</strong>
            </div>
            <div className="price-row total">
              <strong>Total de la reserva</strong>
              <strong>{money(quote.availability.totalMinor)}</strong>
            </div>
            <p className="muted small">
              El precio se vuelve a validar al confirmar. Esta acción crea una
              reserva; no realiza un cobro.
            </p>
            {!!error && <ErrorBox error={error} />}
            <Button
              busy={busy}
              className="full"
              disabled={!customer.active}
              onClick={confirm}
            >
              Confirmar reserva <ArrowUpRight size={19} />
            </Button>
            <span className="panel-trust">
              <ShieldCheck size={16} />
              Una confirmación. Una sola reserva.
            </span>
          </section>
        </div>
      ) : (
        <div className="empty-state">
          <CalendarDays size={34} />
          <h2>Primero, elegí tu auto y tus fechas.</h2>
          {!!error && <ErrorBox error={error} />}
          <p>
            Necesitamos una consulta de disponibilidad vigente para confirmar.
          </p>
          <Link className="button" to="/autos">
            Explorar autos <ArrowRight size={18} />
          </Link>
        </div>
      )}
    </>
  );
}

function Lookup() {
  const [id, setId] = useState("");
  const [error, setError] = useState("");
  const navigate = useNavigate();
  return (
    <>
      <PageHeading
        eyebrow="TUS PLANES, SIEMPRE A MANO"
        title="Volvé a tu reserva."
      />
      <section className="lookup-layout">
        <div className="lookup-card">
          <span className="lookup-icon">
            <Ticket size={30} />
          </span>
          <h2>
            Un código.
            <br />
            Todo tu viaje.
          </h2>
          <p className="muted">
            Ingresá el código que recibiste al confirmar para consultar el
            estado y los detalles de tu reserva.
          </p>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (!validID(id.trim())) {
                setError(
                  "Ingresá un código válido de hasta 64 letras, números, guiones o guiones bajos.",
                );
                return;
              }
              navigate(`/reservas/${encodeURIComponent(id.trim())}`);
            }}
          >
            <label>
              Código de reserva
              <input
                value={id}
                onChange={(e) => setId(e.target.value)}
                maxLength={64}
                placeholder="Ej. código recibido al confirmar"
                required
                aria-describedby={error ? "lookup-error" : undefined}
              />
            </label>
            {!!error && (
              <p role="alert" id="lookup-error" className="field-error">
                {error}
              </p>
            )}
            <Button type="submit">
              Consultar mi reserva <ArrowRight size={18} />
            </Button>
          </form>
          <span className="panel-trust">
            <LockKeyhole size={16} />
            Solo vas a ver reservas asociadas a tu acceso.
          </span>
        </div>
        <div className="lookup-decoration">
          <span className="eyebrow">EL CAMINO SIGUE</span>
          <h2>
            Guardá el código.
            <br />
            <em>Nos vemos en ruta.</em>
          </h2>
          <CarArt tone="green" />
          <span>
            Podés consultar tu reserva cuando lo necesites.
            <br />
            El estado siempre se verifica con el servicio.
          </span>
        </div>
      </section>
    </>
  );
}

function ReservationDetail() {
  const { reservationId = "" } = useParams();
  const {
    data: r,
    error,
    loading,
    retry,
  } = useResource<Reservation>(
    `/v1/reservations/${encodeURIComponent(reservationId)}`,
    isReservation,
  );
  const { call } = useAccess();
  const { busy, run } = useBusy();
  const [cancelError, setCancelError] = useState<unknown>();
  const [confirm, setConfirm] = useState(false);
  const [cancelUnknown, setCancelUnknown] = useState(false);
  const [copied, setCopied] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const location = useLocation();
  useEffect(() => {
    if (confirm) {
      dialog.current?.showModal();
    } else if (dialog.current?.open) {
      dialog.current.close();
      trigger.current?.focus();
    }
  }, [confirm]);
  async function cancel() {
    if (!r) return;
    setCancelError(undefined);
    await run(async () => {
      try {
        const result = checked(
          await call<Reservation>(
            `/v1/reservations/${encodeURIComponent(r.id)}`,
            { method: "DELETE" },
          ),
          isReservation,
        );
        if (result.id !== r.id || result.status !== "CANCELLED")
          throw new ApiError(0, "INVALID_RESPONSE");
        setCancelUnknown(false);
        setConfirm(false);
        retry();
      } catch (err) {
        setCancelError(err);
        setConfirm(false);
        setCancelUnknown(true);
        retry();
      }
    });
  }
  if (loading) return <Loading />;
  if (error) return <ErrorBox error={error} retry={retry} />;
  if (!r) return null;
  return (
    <>
      <Link to="/reservas/consultar" className="back-link">
        <ArrowLeft size={16} />
        Consultar otro código
      </Link>
      <PageHeading
        eyebrow="CADA VIAJE TIENE SU HISTORIA"
        title={
          r.status === "CANCELLED"
            ? "Tu reserva está cancelada."
            : "Tu viaje, en detalle."
        }
      >
        <span className={`status status-${r.status.toLowerCase()}`}>
          <span />
          {statusLabels[r.status]}
        </span>
      </PageHeading>
      {location.state?.created && (
        <div className="notice success">
          <CheckCheck size={20} />
          La reserva se confirmó. El estado que ves a continuación está
          actualizado.
        </div>
      )}
      {!!cancelError && <ErrorBox error={cancelError} />}
      <div className="reservation-grid">
        <section className="reservation-card">
          <div className="reservation-card-head">
            <div>
              <span className="eyebrow">CÓDIGO DE RESERVA</span>
              <h2 className="reservation-id">{r.id}</h2>
            </div>
            <button
              className="icon-button"
              aria-label="Copiar código de reserva"
              onClick={() => {
                void navigator.clipboard
                  ?.writeText(r.id)
                  .then(() => setCopied(true))
                  .catch(() => setCopied(false));
              }}
            >
              {copied ? <Check size={19} /> : <Copy size={19} />}
            </button>
          </div>
          {copied && (
            <span role="status" className="field-hint">
              Código copiado.
            </span>
          )}
          <div className="reservation-car">
            <CarArt />
            <div>
              <span className="eyebrow">VEHÍCULO</span>
              <strong>{r.vehicleId}</strong>
              <Link to={`/autos/${encodeURIComponent(r.vehicleId)}`}>
                Ver ficha <ArrowUpRight size={14} />
              </Link>
            </div>
          </div>
          <div className="trip-dates">
            <div>
              <span className="date-icon">
                <CalendarDays size={20} />
              </span>
              <div>
                <small>RETIRO</small>
                <strong>{dateLabel(r.startsOn)}</strong>
              </div>
            </div>
            <ArrowRight size={18} />
            <div>
              <span className="date-icon">
                <CalendarDays size={20} />
              </span>
              <div>
                <small>DEVOLUCIÓN</small>
                <strong>{dateLabel(r.endsOn)}</strong>
              </div>
            </div>
          </div>
          <p className="field-hint">
            {r.days} {r.days === 1 ? "día" : "días"} · Fechas de la sucursal
            Argentina · El día de devolución no se cobra
          </p>
        </section>
        <aside className="booking-panel">
          <span className="eyebrow">EL PRECIO QUE ACORDASTE</span>
          <h2>Tu reserva, sin vueltas.</h2>
          <div className="price-row">
            <span>Tarifa diaria</span>
            <strong>{money(r.dailyRateMinor)}</strong>
          </div>
          <div className="price-row">
            <span>Duración</span>
            <strong>
              {r.days} {r.days === 1 ? "día" : "días"}
            </strong>
          </div>
          <div className="price-row total">
            <strong>Total</strong>
            <strong>{money(r.totalMinor)}</strong>
          </div>
          <span className="panel-trust">
            <ShieldCheck size={16} />
            Precio congelado al reservar.
          </span>
          <Button
            className="secondary full"
            onClick={() => {
              setCancelUnknown(false);
              retry();
            }}
          >
            Actualizar estado <RefreshCw size={16} />
          </Button>
          {r.status === "RESERVED" &&
            (cancelUnknown ? (
              <div className="notice warning">
                Consultamos el estado después del intento de cancelación.
                Actualizá el estado antes de volver a intentar.
              </div>
            ) : (
              <button
                ref={trigger}
                className="cancel-button"
                onClick={() => setConfirm(true)}
              >
                Cancelar reserva
              </button>
            ))}
          {r.status === "ACTIVE" && (
            <p className="muted small">
              Tu alquiler está en curso. La devolución la registra el operador.
            </p>
          )}
        </aside>
      </div>
      <dialog
        ref={dialog}
        className="cancel-dialog"
        onCancel={(e) => {
          if (busy) e.preventDefault();
          else setConfirm(false);
        }}
        onClose={() => {
          setConfirm(false);
          trigger.current?.focus();
        }}
        aria-labelledby="cancel-title"
      >
        <button
          className="icon-button dialog-close"
          aria-label="Cerrar diálogo"
          disabled={busy}
          onClick={() => setConfirm(false)}
        >
          <X />
        </button>
        <span className="lookup-icon">
          <Ticket size={28} />
        </span>
        <h2 id="cancel-title">¿Cambiaron los planes?</h2>
        <p>
          Vas a cancelar la reserva{" "}
          <strong className="break-word">{r.id}</strong>, del{" "}
          {dateLabel(r.startsOn)} al {dateLabel(r.endsOn)}. Esta acción libera
          tu reserva.
        </p>
        <div className="dialog-actions">
          <Button
            className="secondary"
            disabled={busy}
            onClick={() => setConfirm(false)}
          >
            Mantener reserva
          </Button>
          <Button className="danger" busy={busy} onClick={() => void cancel()}>
            Sí, cancelar
          </Button>
        </div>
      </dialog>
    </>
  );
}
function NotFound() {
  return (
    <div className="empty-state">
      <MapPin size={34} />
      <h1>Este camino no existe.</h1>
      <p>Volvamos a encontrar tu próximo viaje.</p>
      <Link className="button" to="/autos">
        Explorar autos <ArrowRight size={17} />
      </Link>
    </div>
  );
}

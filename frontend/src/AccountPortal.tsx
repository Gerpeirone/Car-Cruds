import { startTransition, useCallback, useEffect, useRef, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import { ArrowRight, CarFront, Eye, EyeOff } from "lucide-react";
import { ApiError, errorMessage, request } from "./api";
import CarArt from "./CarArt";
import "./account.css";

export type Account = {
  id: string;
  firstName: string;
  lastName: string;
  email: string;
  role: "client";
};

export function AccountPortal({ catalog }: { catalog: (account: Account, expire: () => void) => ReactNode }) {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const [account, setAccount] = useState<Account | null>();
  const [error, setError] = useState("");
  const [loggingOut, setLoggingOut] = useState(false);
  const [reload, setReload] = useState(0);
  const expire = useCallback(() => setAccount(null), []);
  const registration = pathname === "/registro";
  const form = registration || pathname === "/iniciar-sesion";
  useEffect(() => {
    const controller = new AbortController();
    request<Account>("/v1/auth/me", "", { signal: controller.signal }).then((value) => {
      if (!controller.signal.aborted) setAccount((current) => current === undefined ? value : current);
    }).catch((err) => {
      if (controller.signal.aborted) return;
      setAccount((current) => current === undefined ? null : current);
      if (!(err instanceof ApiError && err.status === 401)) setError(errorMessage(err));
    });
    return () => controller.abort();
  }, [reload]);
  async function logout() {
    setLoggingOut(true);
    setError("");
    try {
      await request("/v1/auth/logout", "", { method: "POST", body: "{}" });
      // React Router applies navigation as a transition. Clearing the account in
      // the same transition renders the home page once, instead of the
      // protected route redirecting to the login form first.
      startTransition(() => {
        navigate("/", { replace: true });
        setAccount(null);
      });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoggingOut(false);
    }
  }
  if (form && account) return <Navigate to="/catalogo" replace />;
  if (!form && pathname !== "/" && account === undefined)
    return <main className="account-layout"><p role="status">Verificando tu sesión…</p></main>;
  if (!form && pathname !== "/" && account === null && !error)
    return <Navigate to="/iniciar-sesion" replace />;
  return (
    <div className="account-page">
      <a href="#contenido" className="skip-link">Saltar al contenido</a>
      <header className="account-header">
        <Link className="brand" to="/" aria-label="Car Cruds, inicio">
          <span className="brand-icon"><CarFront size={22} /></span>
          <span>car<span className="brand-dot">.</span>cruds</span>
        </Link>
        <nav aria-label="Cuenta">
          {account ? <>
            <Link to="/catalogo">Catálogo</Link>
            <span className="account-name">{account.firstName}</span>
            <Link to="/perfil">Mi perfil</Link>
            <button className="text-button" onClick={logout} disabled={loggingOut}>
              {loggingOut ? "Cerrando sesión…" : "Cerrar sesión"}
            </button>
          </> : <>
            <Link to="/iniciar-sesion">Iniciar sesión</Link>
            <Link className="button" to="/registro">Crear cuenta</Link>
          </>}
        </nav>
      </header>
      {error && <div className="notice error account-status" role="alert">
        {error}
        {!account && <button className="text-button" onClick={() => {
          setError("");
          setAccount(undefined);
          setReload((value) => value + 1);
        }}>Volver a intentar</button>}
      </div>}
      {form ? <main className="account-layout" id="contenido">
        <AccountForm key={pathname} registration={registration} onLogin={(value) => {
          setError("");
          setAccount(value);
          navigate("/catalogo", { replace: true });
        }} />
        <div className="account-visual"><CarArt hero /></div>
      </main> : pathname === "/" ? <main className="account-layout" id="contenido">
        <section className="account-card">
          <h1>Encontrá tu próximo auto</h1>
          <p className="muted">Explorá el catálogo de Car Cruds y compará los vehículos disponibles.</p>
          <div className="account-home-actions">
            <Link className="button" to="/catalogo">
              Explorar {account ? "el " : ""}catálogo <ArrowRight size={18} />
            </Link>
            {!account && <Link className="button secondary" to="/registro">Crear cuenta</Link>}
          </div>
          {!account && <p className="field-hint">Para explorar el catálogo necesitás una cuenta. Reservar requiere además una credencial de alquiler.</p>}
          <Link className="account-alternate" to="/acceso">Ya tengo una credencial de alquiler</Link>
        </section>
        <div className="account-visual"><CarArt hero /></div>
      </main> : account && pathname === "/perfil" ? <main className="account-layout" id="contenido">
        <section className="account-card">
          <h1>Mi perfil</h1>
          <dl><dt>Nombre</dt><dd>{account.firstName} {account.lastName}</dd>
            <dt>Correo electrónico</dt><dd>{account.email}</dd></dl>
          <Link className="button" to="/catalogo">Volver al catálogo</Link>
        </section>
      </main> : account && <main className="main-content" id="contenido">{catalog(account, expire)}</main>}
    </div>
  );
}

function AccountForm({ registration, onLogin }: { registration: boolean; onLogin: (account: Account) => void }) {
  const { state } = useLocation();
  const [visible, setVisible] = useState(false);
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const lock = useRef(false);
  const navigate = useNavigate();
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (lock.current) return;
    setMessage("");
    const data = new FormData(event.currentTarget);
    const email = String(data.get("email") || "").trim();
    const confirmation = String(data.get("passwordConfirmation") || "");
    if (registration && (Array.from(password).length < 12 || new TextEncoder().encode(password).length > 1024)) {
      setMessage("Usá al menos 12 caracteres y hasta 1.024 bytes para la contraseña.");
      return;
    }
    if (registration && password !== confirmation) {
      setMessage("Las contraseñas no coinciden.");
      return;
    }
    lock.current = true;
    setBusy(true);
    try {
      if (registration) {
        await request("/v1/auth/register", "", {
          method: "POST", body: JSON.stringify({ email, password, passwordConfirmation: confirmation,
            firstName: String(data.get("firstName") || "").trim(), lastName: String(data.get("lastName") || "").trim() }),
        });
        // The address travels only in router memory to prefill the login form.
        navigate("/iniciar-sesion", { replace: true, state: { registrationReceived: true, email } });
      } else {
        onLogin(await request<Account>("/v1/auth/login", "", { method: "POST", body: JSON.stringify({ email, password }) }));
      }
    } catch (err) {
      setMessage(!registration && err instanceof ApiError && err.status === 401
        ? "El correo o la contraseña no son correctos." : errorMessage(err));
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  return <section className="account-card">
    <h1>{registration ? "Creá tu cuenta" : "Bienvenido de nuevo"}</h1>
    <p className="muted">{registration ? "Registrate para explorar el catálogo de Car Cruds." : "Ingresá a tu cuenta para continuar."}</p>
    {!registration && state?.registrationReceived && <p className="notice" role="status">
      Recibimos tu solicitud. Si el correo no tenía una cuenta, ya podés iniciar sesión con la contraseña que elegiste.
      Si ya tenía una, no creamos otra ni cambiamos su contraseña: ingresá con la contraseña original.
    </p>}
    <form className="account-form" onSubmit={submit} aria-busy={busy}>
      {registration && <div className="account-fields">
        <label>Nombre<input name="firstName" autoComplete="given-name" required maxLength={80} /></label>
        <label>Apellido<input name="lastName" autoComplete="family-name" required maxLength={80} /></label>
      </div>}
      <label>Correo electrónico<input name="email" type="email" autoComplete={registration ? "email" : "username"} required maxLength={254}
        defaultValue={!registration && typeof state?.email === "string" ? state.email : undefined} /></label>
      <div className="account-password">
        <label htmlFor="account-password">Contraseña</label>
        <input id="account-password" name="password" type={visible ? "text" : "password"} value={password}
          autoComplete={registration ? "new-password" : "current-password"} aria-describedby={registration ? "password-rule" : undefined}
          onChange={(event) => setPassword(event.target.value)} required />
        <button className="icon-button" type="button" onClick={() => setVisible(!visible)} aria-label={visible ? "Ocultar contraseña" : "Mostrar contraseña"}>
          {visible ? <EyeOff size={18} /> : <Eye size={18} />}
        </button>
      </div>
      {registration && <>
        <p id="password-rule" className="field-hint">{Array.from(password).length >= 12 ? "✓ " : ""}Al menos 12 caracteres. Podés usar una frase y caracteres especiales.</p>
        <label>Confirmar contraseña<input name="passwordConfirmation" type={visible ? "text" : "password"} autoComplete="new-password" required /></label>
      </>}
      {message && <p className="notice error" role="alert">{message}</p>}
      <button className="button full" disabled={busy} type="submit">
        {busy ? "Procesando…" : registration ? "Crear cuenta" : "Iniciar sesión"} <ArrowRight size={17} />
      </button>
    </form>
    <p className="account-alternate">{registration ? "¿Ya tenés cuenta? " : "¿No tenés cuenta? "}
      <Link to={registration ? "/iniciar-sesion" : "/registro"}>{registration ? "Iniciá sesión." : "Registrate."}</Link>
    </p>
  </section>;
}

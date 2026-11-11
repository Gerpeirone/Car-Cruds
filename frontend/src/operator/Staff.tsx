import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import {
  Link,
  NavLink,
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import {
  ArrowRight,
  Check,
  HeartPulse,
  Plus,
  RefreshCw,
  Search,
  Send,
  UserMinus,
  X,
} from "lucide-react";
import { ApiError, errorMessage } from "../api";
import {
  Action,
  Back,
  Busy,
  Heading,
  Issue,
  Success,
  checked,
  uncertain,
  useCommand,
  useLoad,
  useOperator,
} from "./shared";
import type { Call } from "./shared";

export interface Employee {
  employeeId: string;
  displayName: string;
  active: boolean;
}
interface EmployeePage {
  items: Employee[];
  page: number;
  pageSize: number;
  total: number;
}
interface Evidence {
  employeeId: string;
  period: string;
  sourceReference: string;
  version: number;
  observedAt: string;
  member?: boolean | null;
  used?: boolean | null;
  costMinor?: number | null;
  spendingMinor?: number | null;
  currency?: string;
}
interface MembershipReport {
  period: string;
  page: number;
  pageSize: number;
  total: number;
  missingEvidence: number;
  incompleteEvidence: number;
  items: (Employee & { sourceApp?: string; evidence: Evidence | null })[];
  totals: {
    currency: string;
    costMinor: number | null;
    spendingMinor: number | null;
    costRecords: number;
    spendingRecords: number;
  }[];
}
type ClinicState = "ready" | "unknown" | "rejected" | "booked" | "cancelled";
type Classification =
  "confirmed" | "without_confirmed_appointment" | "pending_verification";
export interface ClinicRequest {
  requestId: string;
  localReference: string;
  personKind: "employee";
  personId: string;
  patientId: string;
  doctorId: string;
  requestedStartAt: string;
  createdAt: string;
  attemptedAt: string | null;
  observedAt: string | null;
  state: ClinicState;
  classification: Classification;
  lastKnownAppointment: {
    id: string;
    startAt: string;
    endAt: string;
    status: "booked" | "cancelled";
  } | null;
  failureCode?: string;
  nextAttemptAt?: string;
}
interface ClinicReport {
  items: {
    personId: string;
    displayName: string;
    active: boolean | null;
    classification: Classification;
    request: ClinicRequest | null;
  }[];
  page: number;
  pageSize: number;
  total: number;
}

const pageSize = 20;
// Employees accepts opaque keys of up to 100 characters without controls.
export const validKey = (v: string) =>
  v.length > 0 &&
  v.length <= 100 &&
  v.trim() === v &&
  !/[\u0000-\u001f\u007f-\u009f]/.test(v);
const isEmployee = (v: Employee) =>
  !!v &&
  validKey(v.employeeId) &&
  typeof v.displayName === "string" &&
  typeof v.active === "boolean";
const isPage = (v: EmployeePage) =>
  !!v &&
  Array.isArray(v.items) &&
  v.items.every(isEmployee) &&
  Number.isInteger(v.total);
const isMembershipReport = (v: MembershipReport) =>
  !!v &&
  Array.isArray(v.items) &&
  Array.isArray(v.totals) &&
  Number.isInteger(v.total) &&
  Number.isInteger(v.missingEvidence) &&
  Number.isInteger(v.incompleteEvidence);
const states: ClinicState[] = [
  "ready",
  "unknown",
  "rejected",
  "booked",
  "cancelled",
];
export const isClinicRequest = (v: ClinicRequest) =>
  !!v &&
  validKey(v.requestId) &&
  validKey(v.personId) &&
  states.includes(v.state) &&
  typeof v.classification === "string";
interface ClinicDoctors {
  doctors: { id: string; durationMinutes: number }[];
}
const isClinicDoctors = (v: ClinicDoctors) =>
  !!v &&
  Array.isArray(v.doctors) &&
  v.doctors.every((d) => validKey(d.id) && Number.isInteger(d.durationMinutes));
interface ClinicUsage {
  period: string;
  currentPrice: { amountMinor: number; currency: string };
  totals: { currency: string; bookings: number; amountMinor: number }[];
  items: {
    appointmentId: string;
    bookedAt: string;
    priceMinor: number;
    currency: string;
    status: "booked" | "cancelled";
  }[];
}
const isClinicUsage = (v: ClinicUsage) =>
  !!v &&
  typeof v.period === "string" &&
  Array.isArray(v.totals) &&
  Array.isArray(v.items) &&
  Number.isSafeInteger(v.currentPrice?.amountMinor);
// The hospital bills by UTC calendar month.
const utcMonth = () => new Date().toISOString().slice(0, 7);
const isClinicReport = (v: ClinicReport) =>
  !!v && Array.isArray(v.items) && Number.isInteger(v.total);
// A 503 NOT_CONFIGURED is answered before any call to the hospital.
const unsure = (err: unknown) =>
  uncertain(err) && !(err instanceof ApiError && err.code === "NOT_CONFIGURED");

export const stateLabels: Record<ClinicState, string> = {
  ready: "Lista para enviar",
  unknown: "Resultado desconocido",
  rejected: "Rechazada por el hospital",
  booked: "Turno confirmado",
  cancelled: "Turno cancelado",
};
export const classificationLabels: Record<Classification, string> = {
  confirmed: "Confirmado",
  without_confirmed_appointment: "Sin turno confirmado",
  pending_verification: "Pendiente de verificación",
};
const yesNo = (v: boolean | null | undefined) =>
  v === true ? "Sí" : v === false ? "No" : "Sin dato";
export function amount(minor: number | null | undefined, currency = "") {
  if (minor === null || minor === undefined || !currency) return "Sin dato";
  try {
    return new Intl.NumberFormat("es-AR", {
      style: "currency",
      currency,
    }).format(minor / 100);
  } catch {
    return `${minor} ${currency}`;
  }
}
export function moment(iso: string | null | undefined) {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return new Intl.DateTimeFormat("es-AR", {
    timeZone: "America/Buenos_Aires",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).format(date);
}
export function currentPeriod() {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Buenos_Aires",
    year: "numeric",
    month: "2-digit",
  }).format(new Date());
}
// Buenos Aires has no daylight saving time: local wall time is always UTC-3.
export function toInstant(local: string) {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(local))
    throw new Error("Elegí la fecha y hora del turno.");
  const date = new Date(`${local}:00-03:00`);
  if (Number.isNaN(date.getTime()))
    throw new Error("Elegí la fecha y hora del turno.");
  if (date.getTime() <= Date.now())
    throw new Error("El turno debe ser posterior al momento actual.");
  return date.toISOString().replace(".000Z", "Z");
}

function Pager({
  total,
  page,
  noun,
  onPage,
}: {
  total: number;
  page: number;
  noun: string;
  onPage: (page: number) => void;
}) {
  return (
    <div className="operator-pagination">
      <span>
        {total} {noun} · Página {page}
      </span>
      <Action
        className="secondary"
        disabled={page <= 1}
        onClick={() => onPage(page - 1)}
      >
        Anterior
      </Action>
      <Action
        className="secondary"
        disabled={page * pageSize >= total}
        onClick={() => onPage(page + 1)}
      >
        Siguiente
      </Action>
    </div>
  );
}
function usePage() {
  const [params, setParams] = useSearchParams();
  const raw = Number(params.get("page") || "1");
  const page = Number.isSafeInteger(raw) && raw >= 1 ? raw : 1;
  return {
    page,
    params,
    setPage: (next: number) => {
      const copy = new URLSearchParams(params);
      copy.set("page", String(next));
      setParams(copy);
    },
    setParams,
  };
}

export function Staff() {
  const { page, setPage } = usePage();
  const { data, error, loading, reload } = useLoad<EmployeePage>(
    `/v1/admin/employees?page=${page}&pageSize=${pageSize}`,
    isPage,
  );
  const [id, setId] = useState("");
  const [lookupError, setLookupError] = useState<unknown>();
  const navigate = useNavigate();
  function lookup(e: FormEvent) {
    e.preventDefault();
    if (!validKey(id.trim())) {
      setLookupError(new Error("Ingresá un identificador de empleado válido."));
      return;
    }
    navigate(`/operador/personal/${encodeURIComponent(id.trim())}`);
  }
  return (
    <>
      <Heading title="El equipo detrás de cada viaje.">
        <Link to="/operador/personal/nuevo" className="button">
          <Plus size={17} />
          Registrar empleado
        </Link>
      </Heading>
      <form className="operator-toolbar" onSubmit={lookup}>
        <label className="operator-search">
          Consultar empleado por ID
          <input
            value={id}
            onChange={(e) => {
              setId(e.target.value);
              setLookupError(undefined);
            }}
            maxLength={100}
            required
          />
        </label>
        <Action type="submit">
          <Search size={17} />
          Consultar
        </Action>
      </form>
      {!!lookupError && <Issue error={lookupError} />}
      <p className="operator-hint field-hint">
        La lista incluye altas y bajas. El gimnasio solo recibe los empleados
        activos, con su identificador y nombre.
      </p>
      {loading ? (
        <Busy />
      ) : error ? (
        <Issue error={error} retry={reload} />
      ) : (
        data && (
          <div
            className="operator-table-wrap"
            tabIndex={0}
            role="region"
            aria-label="Tabla con desplazamiento horizontal"
          >
            <table className="operator-table">
              <caption className="sr-only">Empleados registrados</caption>
              <thead>
                <tr>
                  <th>Empleado</th>
                  <th>Estado</th>
                  <th>
                    <span className="sr-only">Acciones</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((e) => (
                  <tr key={e.employeeId}>
                    <td>
                      <strong>{e.displayName}</strong>
                      <small>{e.employeeId}</small>
                    </td>
                    <td>
                      <span
                        className={`operator-state ${e.active ? "" : "inactive"}`}
                      >
                        {e.active ? "Activo" : "De baja"}
                      </span>
                    </td>
                    <td>
                      <Link
                        className="text-button"
                        to={`/operador/personal/${encodeURIComponent(e.employeeId)}`}
                        aria-label={`Ver ficha de ${e.displayName}`}
                      >
                        Ver ficha <ArrowRight size={14} />
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!data.items.length && (
              <div className="empty-state">
                <h2>Todavía no hay empleados registrados.</h2>
              </div>
            )}
            <Pager
              total={data.total}
              page={page}
              noun="empleados"
              onPage={setPage}
            />
          </div>
        )
      )}
    </>
  );
}

export function StaffCreate() {
  const [name, setName] = useState("");
  const [active, setActive] = useState(true);
  const [saved, setSaved] = useState<Employee | null>(null);
  const [error, setError] = useState<unknown>();
  const [unknown, setUnknown] = useState(false);
  const { call } = useOperator();
  const command = useCommand();
  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(undefined);
    await command.run(async () => {
      if (unknown || saved) return;
      try {
        const displayName = name.trim();
        if (!displayName || displayName.length > 150)
          throw new Error("Ingresá un nombre de entre 1 y 150 caracteres.");
        setSaved(
          checked(
            await call<Employee>("/v1/employees", {
              method: "POST",
              body: JSON.stringify({ displayName, active }),
            }),
            isEmployee,
          ),
        );
      } catch (err) {
        setError(err);
        if (unsure(err)) setUnknown(true);
      }
    });
  }
  return (
    <>
      <Back to="/operador/personal" children="Volver a personal" />
      <Heading title="Alguien más se suma." />
      <form className="operator-form-card" onSubmit={submit}>
        <div className="operator-section-label">DATOS DEL EMPLEADO</div>
        <div className="operator-fields">
          <label>
            Nombre administrativo
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={unknown || !!saved || command.busy}
              maxLength={150}
              required
            />
          </label>
          <label className="operator-checkbox">
            <input
              type="checkbox"
              checked={active}
              disabled={unknown || !!saved || command.busy}
              onChange={(e) => setActive(e.target.checked)}
            />
            Empleado activo
          </label>
        </div>
        <p className="field-hint">
          El identificador lo asigna el servicio de Empleados. Los activos se
          comparten con el gimnasio.
        </p>
        {!!error && <Issue error={error} />}
        {saved && (
          <Success>
            <strong>Empleado registrado: {saved.displayName}</strong>
            <p>
              Identificador: {saved.employeeId} ·{" "}
              {saved.active ? "Activo" : "De baja"}.{" "}
              <Link
                to={`/operador/personal/${encodeURIComponent(saved.employeeId)}`}
              >
                Abrir ficha
              </Link>
            </p>
          </Success>
        )}
        {unknown && (
          <div className="notice warning">
            El alta pudo haberse guardado y esta API no admite repetirla de
            forma segura. Revisá la lista de personal antes de registrar a la
            misma persona otra vez.
            <Link className="text-button" to="/operador/personal">
              Revisar la lista de personal
            </Link>
          </div>
        )}
        <div className="operator-form-actions">
          <Link to="/operador/personal" className="button secondary">
            Volver a personal
          </Link>
          <Action
            type="submit"
            busy={command.busy}
            disabled={unknown || !!saved}
          >
            Registrar empleado <ArrowRight size={17} />
          </Action>
        </div>
      </form>
    </>
  );
}

export function StaffEditor() {
  const { id = "" } = useParams();
  const { call } = useOperator();
  const {
    data,
    error: loadError,
    loading,
    reload,
  } = useLoad<Employee>(`/v1/employees/${encodeURIComponent(id)}`, isEmployee);
  const [current, setCurrent] = useState<Employee>();
  const [name, setName] = useState("");
  const [active, setActive] = useState(true);
  const [error, setError] = useState<unknown>();
  const [success, setSuccess] = useState("");
  const [unknown, setUnknown] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const command = useCommand();
  const dialog = useRef<HTMLDialogElement>(null);
  const trigger = useRef<HTMLButtonElement | null>(null);
  // The last server answer, from a read or a write, is the record on screen.
  function show(employee: Employee) {
    setCurrent(employee);
    setName(employee.displayName);
    setActive(employee.active);
    setUnknown(false);
  }
  useEffect(() => {
    if (data) show(data);
  }, [data]);
  useEffect(() => {
    if (confirm) dialog.current?.showModal();
    else if (dialog.current?.open) {
      dialog.current.close();
      trigger.current?.focus();
    }
  }, [confirm]);
  async function save(e: FormEvent) {
    e.preventDefault();
    setError(undefined);
    setSuccess("");
    await command.run(async () => {
      try {
        if (unknown)
          throw new Error(
            "Consultá primero la ficha antes de volver a guardar.",
          );
        const displayName = name.trim();
        if (!displayName || displayName.length > 150)
          throw new Error("Ingresá un nombre de entre 1 y 150 caracteres.");
        const result = checked(
          await call<Employee>(`/v1/employees/${encodeURIComponent(id)}`, {
            method: "PUT",
            body: JSON.stringify({ displayName, active }),
          }),
          isEmployee,
        );
        show(result);
        setSuccess(
          `Ficha actualizada: ${result.displayName} · ${result.active ? "Activo" : "De baja"}.`,
        );
      } catch (err) {
        setError(err);
        if (unsure(err)) setUnknown(true);
      }
    });
  }
  async function deactivate() {
    setError(undefined);
    setSuccess("");
    await command.run(async () => {
      try {
        const result = checked(
          await call<Employee>(`/v1/employees/${encodeURIComponent(id)}`, {
            method: "DELETE",
          }),
          isEmployee,
        );
        if (result.active) throw new ApiError(0, "INVALID_RESPONSE");
        show(result);
        setSuccess("Baja registrada. La historia del empleado se conserva.");
      } catch (err) {
        setError(err);
        if (unsure(err)) setUnknown(true);
        else reload();
      } finally {
        setConfirm(false);
      }
    });
  }
  return (
    <>
      <Back to="/operador/personal" children="Volver a personal" />
      <Heading title="Ficha del empleado." />
      {loading && !current ? (
        <Busy />
      ) : loadError && !current ? (
        <Issue error={loadError} retry={reload} />
      ) : (
        current && (
          <>
            <form className="operator-form-card" onSubmit={save}>
              <div className="operator-section-label">{current.employeeId}</div>
              <div className="operator-fields">
                <label>
                  Nombre administrativo
                  <input
                    value={name}
                    onChange={(e) => {
                      setName(e.target.value);
                      setSuccess("");
                    }}
                    maxLength={150}
                    required
                  />
                </label>
                <label className="operator-checkbox">
                  <input
                    type="checkbox"
                    checked={active}
                    onChange={(e) => {
                      setActive(e.target.checked);
                      setSuccess("");
                    }}
                  />
                  Empleado activo
                </label>
              </div>
              <p className="field-hint">
                Guardar reemplaza nombre y estado con los valores del
                formulario. La baja no cancela turnos ni membresías: su
                seguimiento sigue disponible.
              </p>
              {!!error && <Issue error={error} />}
              {!!success && <Success>{success}</Success>}
              {unknown && (
                <div className="notice warning">
                  El cambio pudo haberse guardado aunque la respuesta no llegó.
                  <button
                    type="button"
                    className="text-button"
                    onClick={() => {
                      setUnknown(false);
                      reload();
                    }}
                  >
                    Volver a consultar la ficha
                  </button>
                </div>
              )}
              <div className="operator-form-actions">
                {current.active && (
                  <Action
                    type="button"
                    className="secondary"
                    disabled={unknown || command.busy}
                    onClick={(e) => {
                      trigger.current = e.currentTarget;
                      setConfirm(true);
                    }}
                  >
                    <UserMinus size={17} />
                    Dar de baja
                  </Action>
                )}
                <Action type="submit" busy={command.busy} disabled={unknown}>
                  Guardar cambios <Check size={17} />
                </Action>
              </div>
            </form>
            <IntegrationStatus employee={current} />
            <dialog
              ref={dialog}
              className="cancel-dialog"
              aria-labelledby="staff-confirm-title"
              onCancel={(e) => {
                if (command.busy) e.preventDefault();
                else setConfirm(false);
              }}
              onClose={() => setConfirm(false)}
            >
              <button
                className="icon-button dialog-close"
                disabled={command.busy}
                aria-label="Cerrar confirmación"
                onClick={() => setConfirm(false)}
              >
                <X size={20} />
              </button>
              <span className="lookup-icon">
                <UserMinus size={26} />
              </span>
              <h2 id="staff-confirm-title">Dar de baja</h2>
              <p>
                <strong className="break-word">{current.displayName}</strong>
                <br />
                {current.employeeId}
              </p>
              <p>
                Dejará de compartirse con el gimnasio y no podrá iniciar nuevas
                solicitudes de turno. No se cancelan turnos ni membresías.
              </p>
              <div className="dialog-actions">
                <Action
                  className="secondary"
                  disabled={command.busy}
                  onClick={() => setConfirm(false)}
                >
                  Volver
                </Action>
                <Action busy={command.busy} onClick={() => void deactivate()}>
                  Confirmar baja <Check size={17} />
                </Action>
              </div>
            </dialog>
          </>
        )
      )}
    </>
  );
}

export function Memberships() {
  const { page, params, setParams } = usePage();
  const period = params.get("period") || currentPeriod();
  const selected = params.get("empleado");
  const [draft, setDraft] = useState(period);
  const { data, error, loading, reload } = useLoad<MembershipReport>(
    `/v1/employees/reports/memberships?period=${encodeURIComponent(period)}&page=${page}&pageSize=${pageSize}`,
    isMembershipReport,
  );
  const [formError, setFormError] = useState<unknown>();
  function apply(e: FormEvent) {
    e.preventDefault();
    if (!validKey(draft.trim())) {
      setFormError(new Error("Ingresá un período, por ejemplo 2026-11."));
      return;
    }
    setFormError(undefined);
    setParams({ period: draft.trim(), page: "1" });
  }
  return (
    <>
      <Heading
        eyebrow="INTEGRACIÓN CON EL GIMNASIO"
        title="Gimnasio – Membresías de empleados"
      />
      <form className="operator-toolbar" onSubmit={apply}>
        <label className="operator-search">
          Período informado por el gimnasio
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            maxLength={100}
            placeholder="AAAA-MM"
            required
          />
        </label>
        <Action type="submit">
          <Search size={17} />
          Ver informe
        </Action>
      </form>
      {!!formError && <Issue error={formError} />}
      <p className="operator-hint field-hint">
        El gimnasio consume nuestra lista de empleados y nos envía la evidencia
        de membresía, uso y gastos de cada mes. Sin evidencia no significa que
        no haya uso ni costo: el dato simplemente no llegó. El gimnasio
        administra el vínculo entre cada empleado y su socio; Car Cruds no lo
        guarda.
      </p>
      {loading ? (
        <Busy />
      ) : error ? (
        <Issue error={error} retry={reload} />
      ) : (
        data && (
          <>
            <div className="operator-summary">
              <div>
                <small>EMPLEADOS</small>
                <strong>{data.total}</strong>
              </div>
              <div>
                <small>SIN EVIDENCIA</small>
                <strong>{data.missingEvidence}</strong>
              </div>
              <div>
                <small>EVIDENCIA INCOMPLETA</small>
                <strong>{data.incompleteEvidence}</strong>
              </div>
              {data.totals.map((t) => (
                <div key={t.currency}>
                  <small>TOTAL {t.currency}</small>
                  <strong>{amount(t.costMinor, t.currency)}</strong>
                  <span className="field-hint">
                    Gastos: {amount(t.spendingMinor, t.currency)}
                  </span>
                </div>
              ))}
            </div>
            <div
              className="operator-table-wrap"
              tabIndex={0}
              role="region"
              aria-label="Tabla con desplazamiento horizontal"
            >
              <table className="operator-table">
                <caption className="sr-only">
                  Membresías del período {data.period}
                </caption>
                <thead>
                  <tr>
                    <th>Empleado</th>
                    <th>Socio</th>
                    <th>Uso</th>
                    <th>Costo</th>
                    <th>Gastos</th>
                    <th>Evidencia</th>
                    <th>
                      <span className="sr-only">Acciones</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {data.items.map((item) => {
                    const ev = item.evidence;
                    return (
                      <tr key={item.employeeId}>
                        <td>
                          <strong>{item.displayName}</strong>
                          <small>
                            {item.employeeId}
                            {item.active ? "" : " · De baja"}
                          </small>
                        </td>
                        <td>{ev ? yesNo(ev.member) : "Sin evidencia"}</td>
                        <td>{ev ? yesNo(ev.used) : "—"}</td>
                        <td>{ev ? amount(ev.costMinor, ev.currency) : "—"}</td>
                        <td>
                          {ev ? amount(ev.spendingMinor, ev.currency) : "—"}
                        </td>
                        <td>
                          {ev ? (
                            <>
                              v{ev.version} · {moment(ev.observedAt)}
                              <small>
                                {item.sourceApp || "Origen no informado"}
                              </small>
                            </>
                          ) : (
                            "—"
                          )}
                        </td>
                        <td>
                          <button
                            type="button"
                            className="text-button"
                            aria-label={`Ver evidencia de ${item.displayName}`}
                            onClick={() =>
                              setParams({
                                period,
                                page: String(page),
                                empleado: item.employeeId,
                              })
                            }
                          >
                            Ver evidencia <ArrowRight size={14} />
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              {!data.items.length && (
                <div className="empty-state">
                  <h2>No hay empleados para este período.</h2>
                </div>
              )}
              <Pager
                total={data.total}
                page={page}
                noun="empleados"
                onPage={(next) => setParams({ period, page: String(next) })}
              />
            </div>
            {selected && (
              <EvidenceDetail
                employeeId={selected}
                period={period}
                row={data.items.find((i) => i.employeeId === selected)}
                onClose={() => setParams({ period, page: String(page) })}
              />
            )}
          </>
        )
      )}
    </>
  );
}

const clinicDraftKey = "carcruds.operator.clinic.v1";
type ClinicDraft = {
  personId: string;
  patientId: string;
  doctorId: string;
  startLocal: string;
  localReference: string;
};
function readClinicDraft(): ClinicDraft | null {
  try {
    const raw = sessionStorage.getItem(clinicDraftKey);
    const v = raw ? (JSON.parse(raw) as ClinicDraft) : null;
    return v && validKey(v.localReference) ? v : null;
  } catch {
    return null;
  }
}
function forgetClinicDraft() {
  try {
    sessionStorage.removeItem(clinicDraftKey);
  } catch {
    // Storage may be unavailable; the reference is still shown on screen.
  }
}

export function Clinic() {
  const { page, setPage, params } = usePage();
  const employees = useLoad<EmployeePage>(
    "/v1/admin/employees?page=1&pageSize=100",
    isPage,
  );
  const overview = useLoad<ClinicReport>(
    "/v1/employees/reports/clinic?page=1&pageSize=100",
    isClinicReport,
  );
  const [review, setReview] = useState<{
    draft: ClinicDraft;
    startAt: string;
  } | null>(null);
  const reviewDialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (review) reviewDialog.current?.showModal();
    else if (reviewDialog.current?.open) reviewDialog.current.close();
  }, [review]);
  const active = employees.data?.items.filter((e) => e.active) ?? [];
  const doctors = useLoad<ClinicDoctors>(
    "/v1/employees/clinic/doctors",
    isClinicDoctors,
  );
  const { data, error, loading, reload } = useLoad<ClinicReport>(
    `/v1/employees/reports/clinic?page=${page}&pageSize=${pageSize}`,
    isClinicReport,
  );
  const [pending] = useState(readClinicDraft);
  const [form, setForm] = useState<ClinicDraft>(
    () =>
      pending ?? {
        personId: params.get("empleado") ?? "",
        patientId: "",
        doctorId: "",
        startLocal: "",
        localReference: `turno-${crypto.randomUUID().slice(0, 13)}`,
      },
  );
  const [formError, setFormError] = useState<unknown>();
  const [unknown, setUnknown] = useState(!!pending);
  const { call } = useOperator();
  const command = useCommand();
  const navigate = useNavigate();
  const locked = unknown || command.busy;
  function update<K extends keyof ClinicDraft>(key: K, value: string) {
    setForm((current) => ({ ...current, [key]: value }));
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    setFormError(undefined);
    try {
      const draft = {
        ...form,
        personId: form.personId.trim(),
        patientId: form.patientId.trim(),
        doctorId: form.doctorId.trim(),
      };
      if (![draft.personId, draft.patientId, draft.doctorId].every(validKey))
        throw new Error(
          "Completá empleado, paciente y profesional con identificadores de hasta 100 caracteres.",
        );
      // Nothing reaches the hospital until the operator confirms the summary.
      setReview({ draft, startAt: toInstant(draft.startLocal) });
    } catch (err) {
      setFormError(err);
    }
  }
  async function save() {
    if (!review) return;
    const { draft, startAt } = review;
    await command.run(async () => {
      try {
        // Saving the reference first lets a lost response be recovered with
        // the same body: Employees returns the original intention for it.
        try {
          sessionStorage.setItem(clinicDraftKey, JSON.stringify(draft));
        } catch {
          // Without storage the reference stays visible in the form.
        }
        await call<void>("/v1/employees/clinic/patient-links", {
          method: "POST",
          body: JSON.stringify({
            personKind: "employee",
            personId: draft.personId,
            patientId: draft.patientId,
          }),
        });
        const created = checked(
          await call<ClinicRequest>("/v1/employees/clinic/requests", {
            method: "POST",
            body: JSON.stringify({
              personKind: "employee",
              personId: draft.personId,
              localReference: draft.localReference,
              doctorId: draft.doctorId,
              startAt,
            }),
          }),
          isClinicRequest,
        );
        forgetClinicDraft();
        setReview(null);
        navigate(
          `/operador/hospital/${encodeURIComponent(created.requestId)}`,
          { state: { clinicCreated: true } },
        );
      } catch (err) {
        setReview(null);
        setFormError(err);
        if (unsure(err)) setUnknown(true);
        else if (err instanceof ApiError) forgetClinicDraft();
      }
    });
  }
  function discard() {
    forgetClinicDraft();
    setUnknown(false);
    setFormError(undefined);
    setForm({
      personId: "",
      patientId: "",
      doctorId: "",
      startLocal: "",
      localReference: `turno-${crypto.randomUUID().slice(0, 13)}`,
    });
  }
  return (
    <>
      <Heading
        eyebrow="INTEGRACIÓN CON EL HOSPITAL"
        title="Hospital – Gestión de turnos médicos"
      />
      <ClinicOverview report={overview} />
      <div className="operator-split">
        <form className="operator-form-card" onSubmit={submit}>
          <div className="operator-section-label">NUEVA SOLICITUD DE TURNO</div>
          <div className="operator-fields">
            {active.length ? (
              <label>
                Empleado
                <select
                  value={form.personId}
                  onChange={(e) => update("personId", e.target.value)}
                  disabled={locked}
                  required
                >
                  <option value="">Elegí un empleado activo</option>
                  {form.personId &&
                    !active.some((e) => e.employeeId === form.personId) && (
                      <option value={form.personId}>{form.personId}</option>
                    )}
                  {active.map((e) => (
                    <option key={e.employeeId} value={e.employeeId}>
                      {e.displayName} · {e.employeeId}
                    </option>
                  ))}
                </select>
              </label>
            ) : (
              <label>
                ID del empleado
                <input
                  value={form.personId}
                  onChange={(e) => update("personId", e.target.value)}
                  disabled={locked}
                  maxLength={100}
                  required
                />
              </label>
            )}
            <label>
              ID de paciente en el hospital
              <input
                value={form.patientId}
                onChange={(e) => update("patientId", e.target.value)}
                disabled={locked}
                maxLength={100}
                required
              />
            </label>
            {doctors.data?.doctors.length ? (
              <label>
                Médico del hospital
                <select
                  value={form.doctorId}
                  onChange={(e) => update("doctorId", e.target.value)}
                  disabled={locked}
                  required
                >
                  <option value="">Elegí un médico</option>
                  {doctors.data.doctors.map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.id} · {d.durationMinutes} min
                    </option>
                  ))}
                </select>
              </label>
            ) : (
              <label>
                ID del profesional
                <input
                  value={form.doctorId}
                  onChange={(e) => update("doctorId", e.target.value)}
                  disabled={locked}
                  maxLength={100}
                  required
                />
              </label>
            )}
            <label>
              Fecha y hora (Buenos Aires)
              <input
                type="datetime-local"
                value={form.startLocal}
                onChange={(e) => update("startLocal", e.target.value)}
                disabled={locked}
                required
              />
            </label>
          </div>
          <p className="field-hint">
            Referencia de la solicitud: <strong>{form.localReference}</strong>.
            Guardar no envía nada al hospital: el envío se confirma en la
            siguiente pantalla. El paciente lo habilita el hospital; sus
            pacientes de prueba informados son pat-001 y pat-003.
          </p>
          {!!formError && <Issue error={formError} />}
          {unknown && (
            <div className="notice warning">
              La solicitud pudo haberse guardado. Repetir con la misma
              referencia recupera la original sin duplicarla.
              <button
                type="button"
                className="text-button"
                disabled={command.busy}
                onClick={() => {
                  setUnknown(false);
                  setFormError(undefined);
                }}
              >
                Revisar y reintentar con la misma referencia
              </button>
              <button
                type="button"
                className="text-button"
                disabled={command.busy}
                onClick={discard}
              >
                Descartar y empezar otra
              </button>
            </div>
          )}
          <div className="operator-form-actions">
            <Action type="submit" busy={command.busy} disabled={unknown}>
              Revisar solicitud <ArrowRight size={17} />
            </Action>
          </div>
        </form>
        <aside className="operator-explainer">
          <HeartPulse size={34} />
          <h2>Cómo funciona</h2>
          <ul>
            <li>Se vincula al empleado activo con su paciente del hospital.</li>
            <li>Se guarda la intención con una referencia estable.</li>
            <li>
              El envío al hospital es una acción explícita, sin reintentos
              automáticos.
            </li>
            <li>
              Un resultado desconocido se concilia consultando al hospital.
            </li>
          </ul>
        </aside>
      </div>
      {!!doctors.error && (
        <p className="operator-hint field-hint">
          No pudimos leer los médicos del hospital (
          {errorMessage(doctors.error)}). Podés ingresar el ID del profesional a
          mano.
        </p>
      )}
      <dialog
        ref={reviewDialog}
        className="cancel-dialog"
        aria-labelledby="clinic-review-title"
        onCancel={(e) => {
          if (command.busy) e.preventDefault();
          else setReview(null);
        }}
        onClose={() => setReview(null)}
      >
        <button
          className="icon-button dialog-close"
          disabled={command.busy}
          aria-label="Cerrar resumen"
          onClick={() => setReview(null)}
        >
          <X size={20} />
        </button>
        <span className="lookup-icon">
          <HeartPulse size={26} />
        </span>
        <h2 id="clinic-review-title">Confirmar solicitud</h2>
        {review && (
          <dl className="operator-review">
            <dt>Empleado</dt>
            <dd>{review.draft.personId}</dd>
            <dt>Paciente</dt>
            <dd>{review.draft.patientId}</dd>
            <dt>Médico</dt>
            <dd>{review.draft.doctorId}</dd>
            <dt>Turno</dt>
            <dd>{moment(review.startAt)}</dd>
            <dt>Referencia</dt>
            <dd className="break-word">{review.draft.localReference}</dd>
          </dl>
        )}
        <p>
          Se vincula el empleado con el paciente y se guarda la solicitud.
          Todavía no se envía al hospital ni se genera ningún cargo.
        </p>
        <div className="dialog-actions">
          <Action
            className="secondary"
            disabled={command.busy}
            onClick={() => setReview(null)}
          >
            Volver
          </Action>
          <Action busy={command.busy} onClick={() => void save()}>
            Confirmar y guardar <Check size={17} />
          </Action>
        </div>
      </dialog>
      <ClinicBilling />
      <h2 className="operator-subtitle">Última solicitud por empleado</h2>
      {loading ? (
        <Busy />
      ) : error ? (
        <Issue error={error} retry={reload} />
      ) : (
        data && (
          <div
            className="operator-table-wrap"
            tabIndex={0}
            role="region"
            aria-label="Tabla con desplazamiento horizontal"
          >
            <table className="operator-table">
              <caption className="sr-only">Turnos médicos del personal</caption>
              <thead>
                <tr>
                  <th>Empleado</th>
                  <th>Clasificación</th>
                  <th>Estado</th>
                  <th>Turno pedido</th>
                  <th>
                    <span className="sr-only">Acciones</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((item) => (
                  <tr key={item.personId}>
                    <td>
                      <strong>{item.displayName}</strong>
                      <small>
                        {item.personId}
                        {item.active === false ? " · De baja" : ""}
                      </small>
                    </td>
                    <td>
                      <span
                        className={`operator-state ${item.classification === "confirmed" ? "" : "inactive"}`}
                      >
                        {classificationLabels[item.classification] ??
                          item.classification}
                      </span>
                    </td>
                    <td>
                      {item.request ? (
                        <>
                          {stateLabels[item.request.state]}
                          {item.request.failureCode && (
                            <small>{item.request.failureCode}</small>
                          )}
                        </>
                      ) : (
                        "—"
                      )}
                    </td>
                    <td>
                      {item.request
                        ? moment(item.request.requestedStartAt)
                        : "Sin solicitudes"}
                    </td>
                    <td>
                      {item.request && (
                        <Link
                          className="text-button"
                          to={`/operador/hospital/${encodeURIComponent(item.request.requestId)}`}
                          aria-label={`Ver solicitud de ${item.displayName}`}
                        >
                          Ver solicitud <ArrowRight size={14} />
                        </Link>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!data.items.length && (
              <div className="empty-state">
                <h2>Todavía no hay empleados registrados.</h2>
              </div>
            )}
            <Pager
              total={data.total}
              page={page}
              noun="empleados"
              onPage={setPage}
            />
          </div>
        )
      )}
    </>
  );
}

export function ClinicRequestView() {
  const { requestId = "" } = useParams();
  const location = useLocation();
  const { call } = useOperator();
  const {
    data: r,
    error: loadError,
    loading,
    reload,
  } = useLoad<ClinicRequest>(
    `/v1/employees/clinic/requests/${encodeURIComponent(requestId)}`,
    isClinicRequest,
  );
  const [error, setError] = useState<unknown>();
  const [success, setSuccess] = useState(() =>
    location.state?.clinicCreated
      ? "Solicitud guardada. Todavía no se envió al hospital."
      : "",
  );
  const [unknown, setUnknown] = useState(false);
  const [appointmentId, setAppointmentId] = useState("");
  const [confirm, setConfirm] = useState(false);
  const command = useCommand();
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (confirm) dialog.current?.showModal();
    else if (dialog.current?.open) dialog.current.close();
  }, [confirm]);
  const waiting =
    !!r?.nextAttemptAt && new Date(r.nextAttemptAt).getTime() > Date.now();
  const canSubmit =
    !!r &&
    (r.state === "ready" || r.state === "unknown") &&
    !r.lastKnownAppointment;
  const canRefresh = !!r?.attemptedAt;
  function outcome(result: ClinicRequest) {
    return result.classification === "confirmed"
      ? "El hospital confirmó el turno."
      : result.classification === "without_confirmed_appointment"
        ? "El hospital informó que la solicitud no tiene un turno confirmado."
        : "El resultado sigue pendiente de verificación. Consultá al hospital más tarde.";
  }
  async function act(kind: "submit" | "refresh") {
    setError(undefined);
    setSuccess("");
    await command.run(async () => {
      try {
        const recovery = appointmentId.trim();
        if (kind === "refresh" && recovery && !validKey(recovery))
          throw new Error("Ingresá un ID de turno válido.");
        const result = checked(
          await call<ClinicRequest>(
            `/v1/employees/clinic/requests/${encodeURIComponent(requestId)}/${kind}`,
            {
              method: "POST",
              ...(kind === "refresh"
                ? {
                    body: JSON.stringify(
                      recovery ? { appointmentId: recovery } : {},
                    ),
                  }
                : {}),
            },
          ),
          isClinicRequest,
        );
        setUnknown(false);
        setSuccess(outcome(result));
      } catch (err) {
        setError(err);
        setUnknown(unsure(err));
      } finally {
        setConfirm(false);
        reload();
      }
    });
  }
  return (
    <>
      <Back to="/operador/hospital" children="Volver a Hospital" />
      <Heading title="Seguimiento del turno." />
      {!!error && <Issue error={error} />}
      {!!success && <Success>{success}</Success>}
      {unknown && (
        <div className="notice warning">
          No sabemos si el hospital recibió la acción. La solicitud conserva su
          clave: volver a enviarla no duplica el turno.
        </div>
      )}
      {loading && !r ? (
        <Busy />
      ) : loadError ? (
        <Issue error={loadError} retry={reload} />
      ) : (
        r && (
          <section className="operator-reservation">
            <div className="operator-reservation-head">
              <div>
                <span className="eyebrow">SOLICITUD {r.localReference}</span>
                <h2 className="break-word">{r.requestId}</h2>
              </div>
              <span className={`status status-clinic-${r.state}`}>
                <span />
                {stateLabels[r.state]}
              </span>
            </div>
            <div className="operator-reservation-facts">
              <div>
                <small>EMPLEADO</small>
                <strong>{r.personId}</strong>
              </div>
              <div>
                <small>PACIENTE</small>
                <strong>{r.patientId}</strong>
              </div>
              <div>
                <small>PROFESIONAL</small>
                <strong>{r.doctorId}</strong>
              </div>
              <div>
                <small>TURNO PEDIDO</small>
                <strong>{moment(r.requestedStartAt)}</strong>
              </div>
            </div>
            <div className="operator-reservation-facts">
              <div>
                <small>CLASIFICACIÓN</small>
                <strong>
                  {classificationLabels[r.classification] ?? r.classification}
                </strong>
              </div>
              <div>
                <small>ÚLTIMO ENVÍO</small>
                <strong>{moment(r.attemptedAt)}</strong>
              </div>
              <div>
                <small>ÚLTIMA EVIDENCIA</small>
                <strong>{moment(r.observedAt)}</strong>
              </div>
              <div>
                <small>TURNO DEL HOSPITAL</small>
                <strong className="break-word">
                  {r.lastKnownAppointment
                    ? `${r.lastKnownAppointment.id} · ${r.lastKnownAppointment.status === "booked" ? "Reservado" : "Cancelado"}`
                    : "—"}
                </strong>
              </div>
            </div>
            {(r.failureCode || waiting) && (
              <p className="field-hint">
                {r.failureCode && (
                  <>Último código informado: {r.failureCode}. </>
                )}
                {waiting && (
                  <>
                    El hospital pidió esperar hasta {moment(r.nextAttemptAt)}.
                  </>
                )}
              </p>
            )}
            {canRefresh && !r.lastKnownAppointment && (
              <label className="operator-inline-field">
                ID de turno del hospital (para conciliar)
                <input
                  value={appointmentId}
                  onChange={(e) => setAppointmentId(e.target.value)}
                  maxLength={100}
                />
              </label>
            )}
            <div className="operator-form-actions">
              <Action
                className="secondary"
                onClick={() => {
                  setUnknown(false);
                  reload();
                }}
              >
                Actualizar <RefreshCw size={16} />
              </Action>
              {canRefresh && (
                <Action
                  className="secondary"
                  busy={command.busy && !confirm}
                  disabled={
                    command.busy ||
                    waiting ||
                    (!r.lastKnownAppointment && !appointmentId.trim())
                  }
                  onClick={() => void act("refresh")}
                >
                  Consultar al hospital <HeartPulse size={16} />
                </Action>
              )}
              {canSubmit && (
                <Action
                  disabled={command.busy || waiting}
                  onClick={() => setConfirm(true)}
                >
                  {r.state === "unknown"
                    ? "Reenviar al hospital"
                    : "Enviar al hospital"}{" "}
                  <Send size={16} />
                </Action>
              )}
            </div>
            <p className="field-hint">
              Cada acción hace como máximo una llamada al hospital. Una
              respuesta correcta no siempre confirma el turno: se informa la
              clasificación.
            </p>
            <dialog
              ref={dialog}
              className="cancel-dialog"
              aria-labelledby="clinic-confirm-title"
              onCancel={(e) => {
                if (command.busy) e.preventDefault();
                else setConfirm(false);
              }}
              onClose={() => setConfirm(false)}
            >
              <button
                className="icon-button dialog-close"
                disabled={command.busy}
                aria-label="Cerrar confirmación"
                onClick={() => setConfirm(false)}
              >
                <X size={20} />
              </button>
              <span className="lookup-icon">
                <HeartPulse size={26} />
              </span>
              <h2 id="clinic-confirm-title">Enviar solicitud al hospital</h2>
              <p>
                Empleado {r.personId} · Profesional {r.doctorId}
                <br />
                {moment(r.requestedStartAt)}
              </p>
              <p>
                Se usa la misma clave de la solicitud: si ya había llegado, el
                hospital devuelve el turno original.
              </p>
              <div className="dialog-actions">
                <Action
                  className="secondary"
                  disabled={command.busy}
                  onClick={() => setConfirm(false)}
                >
                  Volver
                </Action>
                <Action busy={command.busy} onClick={() => void act("submit")}>
                  Confirmar envío <Check size={17} />
                </Action>
              </div>
            </dialog>
          </section>
        )
      )}
    </>
  );
}

// The backend expects YYYY-MM (e.g. 2026-11). The same source feeds the HTML
// pattern (implicitly anchored) and the submit check, so they cannot diverge.
const billingPeriodPattern = "\\d{4}-(0[1-9]|1[0-2])";
const billingPeriod = new RegExp(`^${billingPeriodPattern}$`);
function ClinicBilling() {
  const [period, setPeriod] = useState(utcMonth);
  const [draft, setDraft] = useState(period);
  const { data, error, loading, reload } = useLoad<ClinicUsage>(
    `/v1/employees/clinic/usage?period=${encodeURIComponent(period)}`,
    isClinicUsage,
  );
  return (
    <>
      <h2 className="operator-subtitle">Facturación del hospital</h2>
      <form
        className="operator-toolbar"
        onSubmit={(e) => {
          e.preventDefault();
          if (billingPeriod.test(draft.trim())) setPeriod(draft.trim());
        }}
      >
        <label className="operator-search">
          Mes facturado (UTC)
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="AAAA-MM"
            pattern={billingPeriodPattern}
            title="Año y mes, por ejemplo 2026-11"
            required
          />
        </label>
        <Action type="submit">
          <Search size={17} />
          Ver facturación
        </Action>
      </form>
      <p className="operator-hint field-hint">
        El hospital cobra cada reserva creada al precio vigente. Cancelar no
        reintegra el cargo y repetir una reserva incierta no la cobra de nuevo.
      </p>
      {loading ? (
        <Busy />
      ) : error ? (
        <Issue error={error} retry={reload} />
      ) : (
        data && (
          <>
            <div className="operator-summary">
              <div>
                <small>PRECIO VIGENTE</small>
                <strong>
                  {amount(
                    data.currentPrice.amountMinor,
                    data.currentPrice.currency,
                  )}
                </strong>
              </div>
              {data.totals.map((t) => (
                <div key={t.currency}>
                  <small>TOTAL {t.currency}</small>
                  <strong>{amount(t.amountMinor, t.currency)}</strong>
                  <span className="field-hint">
                    {t.bookings === 1
                      ? "1 reserva cobrada"
                      : `${t.bookings} reservas cobradas`}
                  </span>
                </div>
              ))}
              {!data.totals.length && (
                <div>
                  <small>TOTAL DEL MES</small>
                  <strong>Sin cargos</strong>
                </div>
              )}
            </div>
            {!!data.items.length && (
              <div
                className="operator-table-wrap"
                tabIndex={0}
                role="region"
                aria-label="Tabla con desplazamiento horizontal"
              >
                <table className="operator-table">
                  <caption className="sr-only">
                    Cargos del hospital en {data.period}
                  </caption>
                  <thead>
                    <tr>
                      <th>Turno</th>
                      <th>Reservado</th>
                      <th>Estado</th>
                      <th>Cargo</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.items.map((i) => (
                      <tr key={i.appointmentId}>
                        <td>
                          <small>{i.appointmentId}</small>
                        </td>
                        <td>{moment(i.bookedAt)}</td>
                        <td>
                          <span
                            className={`operator-state ${i.status === "booked" ? "" : "inactive"}`}
                          >
                            {i.status === "booked" ? "Reservado" : "Cancelado"}
                          </span>
                        </td>
                        <td>{amount(i.priceMinor, i.currency)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )
      )}
    </>
  );
}

// Indicators come only from the employees clinic report: the last request
// of each employee. No aggregate endpoint exists, so nothing else is shown.
function ClinicOverview({
  report,
}: {
  report: { data?: ClinicReport; error?: unknown; loading: boolean };
}) {
  if (report.loading) return <Busy />;
  if (report.error) return <Issue error={report.error} />;
  if (!report.data) return null;
  const requests = report.data.items.flatMap((i) =>
    i.request ? [i.request] : [],
  );
  const count = (...states: ClinicState[]) =>
    requests.filter((r) => states.includes(r.state)).length;
  const cards: [string, number][] = [
    ["CON SOLICITUD", requests.length],
    ["CONFIRMADAS", count("booked")],
    ["REGISTRADAS SIN ENVIAR", count("ready")],
    ["A CONCILIAR", count("unknown")],
    ["RECHAZADAS O CANCELADAS", count("rejected", "cancelled")],
  ];
  const partial = report.data.total > report.data.items.length;
  return (
    <>
      <div className="operator-summary">
        {cards.map(([label, value]) => (
          <div key={label}>
            <small>{label}</small>
            <strong>{value}</strong>
          </div>
        ))}
      </div>
      <p className="operator-hint field-hint">
        Cuenta la última solicitud de cada empleado según Empleados
        {partial
          ? ` (primeros ${report.data.items.length} de ${report.data.total} empleados)`
          : ""}
        . Car Cruds llama al hospital desde su servicio de Empleados; este panel
        nunca se comunica con el hospital directamente.
      </p>
    </>
  );
}

interface EvidenceHistory {
  employeeId: string;
  period: string;
  items: (Omit<Evidence, "employeeId" | "period"> & {
    sourceApp: string;
    receivedAt: string;
  })[];
}
const isEvidenceHistory = (v: EvidenceHistory) =>
  !!v &&
  Array.isArray(v.items) &&
  v.items.every(
    (i) => Number.isInteger(i.version) && typeof i.receivedAt === "string",
  );

function EvidenceDetail({
  employeeId,
  period,
  row,
  onClose,
}: {
  employeeId: string;
  period: string;
  row?: MembershipReport["items"][number];
  onClose: () => void;
}) {
  const history = useLoad<EvidenceHistory>(
    `/v1/employees/${encodeURIComponent(employeeId)}/membership-evidence?period=${encodeURIComponent(period)}`,
    isEvidenceHistory,
  );
  const ev = row?.evidence;
  const facts: [string, string][] = ev
    ? [
        ["EMPLEADO", ev.employeeId],
        ["ORIGEN", row?.sourceApp || "No informado"],
        ["REFERENCIA", ev.sourceReference],
        ["VERSIÓN VIGENTE", String(ev.version)],
        ["OBSERVADA", moment(ev.observedAt)],
        ["SOCIO", yesNo(ev.member)],
        ["USO", yesNo(ev.used)],
        ["COSTO", amount(ev.costMinor, ev.currency)],
        ["GASTOS", amount(ev.spendingMinor, ev.currency)],
        ["MONEDA", ev.currency || "No informada"],
      ]
    : [];
  return (
    <section className="operator-reservation" aria-labelledby="evidence-title">
      <div className="operator-reservation-head">
        <div>
          <span className="eyebrow">EVIDENCIA DEL PERÍODO {period}</span>
          <h2 id="evidence-title" className="break-word">
            {row?.displayName ?? employeeId}
          </h2>
        </div>
        <button type="button" className="text-button" onClick={onClose}>
          Cerrar detalle <X size={14} />
        </button>
      </div>
      {!row && (
        <p className="field-hint">
          El empleado no figura en esta página del informe; se muestra solo su
          historial.
        </p>
      )}
      {row && !ev && (
        <div className="notice warning">
          No llegó evidencia del gimnasio para este empleado en {period}. No
          significa que no sea socio ni que no haya gastos.
        </div>
      )}
      {!!facts.length && (
        <div className="operator-reservation-facts">
          {facts.map(([label, value]) => (
            <div key={label}>
              <small>{label}</small>
              <strong className="break-word">{value}</strong>
            </div>
          ))}
        </div>
      )}
      <h3 className="operator-subtitle">Versiones recibidas</h3>
      {history.loading ? (
        <Busy />
      ) : history.error ? (
        <Issue error={history.error} retry={history.reload} />
      ) : history.data?.items.length ? (
        <div
          className="operator-table-wrap"
          tabIndex={0}
          role="region"
          aria-label="Tabla con desplazamiento horizontal"
        >
          <table className="operator-table">
            <caption className="sr-only">
              Historial de evidencia de {employeeId} en {period}
            </caption>
            <thead>
              <tr>
                <th>Versión</th>
                <th>Recibida</th>
                <th>Observada</th>
                <th>Socio / uso</th>
                <th>Origen</th>
              </tr>
            </thead>
            <tbody>
              {history.data.items.map((h) => (
                <tr key={h.version}>
                  <td>
                    v{h.version}
                    {ev?.version === h.version && <small>Vigente</small>}
                  </td>
                  <td>{moment(h.receivedAt)}</td>
                  <td>{moment(h.observedAt)}</td>
                  <td>
                    {yesNo(h.member)} / {yesNo(h.used)}
                  </td>
                  <td>
                    {h.sourceApp}
                    <small>{h.sourceReference}</small>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <p className="field-hint">Sin versiones recibidas para este período.</p>
      )}
      <p className="field-hint">
        Una versión atrasada se guarda en el historial pero no reemplaza la
        vigente. El gimnasio administra el vínculo con su socio.
      </p>
    </section>
  );
}

// Finds one employee in a paginated report without inventing a filter the
// API does not offer. It stops after a bounded number of pages.
async function findInReport<T extends { total: number }>(
  call: Call,
  path: (page: number) => string,
  ids: (data: T) => string[],
  id: string,
  maxPages = 10,
) {
  for (let page = 1; page <= maxPages; page++) {
    const data = await call<T>(path(page));
    const index = ids(data).indexOf(id);
    if (index >= 0) return { data, index };
    if (page * 100 >= data.total) return null;
  }
  return null;
}

type Lookup<T> = { loading: boolean; error?: unknown; item?: T | null };

function IntegrationStatus({ employee }: { employee: Employee }) {
  const { call } = useOperator();
  const period = currentPeriod();
  const [clinic, setClinic] = useState<Lookup<ClinicReport["items"][number]>>({
    loading: true,
  });
  const [gym, setGym] = useState<Lookup<MembershipReport["items"][number]>>({
    loading: true,
  });
  useEffect(() => {
    let alive = true;
    findInReport<ClinicReport>(
      call,
      (p) => `/v1/employees/reports/clinic?page=${p}&pageSize=100`,
      (d) => d.items.map((i) => i.personId),
      employee.employeeId,
    )
      .then(
        (r) =>
          alive &&
          setClinic({ loading: false, item: r ? r.data.items[r.index] : null }),
      )
      .catch((error) => alive && setClinic({ loading: false, error }));
    findInReport<MembershipReport>(
      call,
      (p) =>
        `/v1/employees/reports/memberships?period=${encodeURIComponent(period)}&page=${p}&pageSize=100`,
      (d) => d.items.map((i) => i.employeeId),
      employee.employeeId,
    )
      .then(
        (r) =>
          alive &&
          setGym({ loading: false, item: r ? r.data.items[r.index] : null }),
      )
      .catch((error) => alive && setGym({ loading: false, error }));
    return () => {
      alive = false;
    };
  }, [call, employee.employeeId, period]);
  const id = encodeURIComponent(employee.employeeId);
  const request = clinic.item?.request;
  const ev = gym.item?.evidence;
  return (
    <div className="operator-integrations">
      <section className="operator-form-card" aria-labelledby="int-hospital">
        <div className="operator-section-label" id="int-hospital">
          HOSPITAL · TURNOS MÉDICOS
        </div>
        {clinic.loading ? (
          <Busy />
        ) : clinic.error ? (
          <Issue error={clinic.error} />
        ) : request ? (
          <p>
            <strong>{stateLabels[request.state]}</strong> ·{" "}
            {classificationLabels[request.classification]}
            <br />
            Turno pedido: {moment(request.requestedStartAt)}
            <br />
            Última evidencia: {moment(request.observedAt)}
          </p>
        ) : (
          <p className="field-hint">
            {clinic.item === null
              ? "No se encontró en el informe de Hospital."
              : "Sin solicitudes de turno registradas."}
          </p>
        )}
        <div className="operator-form-actions">
          {request && (
            <Link
              className="button secondary"
              to={`/operador/hospital/${encodeURIComponent(request.requestId)}`}
            >
              Ver solicitud
            </Link>
          )}
          {employee.active && (
            <Link className="button" to={`/operador/hospital?empleado=${id}`}>
              Solicitar turno <ArrowRight size={16} />
            </Link>
          )}
        </div>
      </section>
      <section className="operator-form-card" aria-labelledby="int-gym">
        <div className="operator-section-label" id="int-gym">
          GIMNASIO · MEMBRESÍA {period}
        </div>
        {gym.loading ? (
          <Busy />
        ) : gym.error ? (
          <Issue error={gym.error} />
        ) : ev ? (
          <p>
            <strong>Socio: {yesNo(ev.member)}</strong> · Uso: {yesNo(ev.used)}
            <br />
            Versión {ev.version} de{" "}
            {gym.item?.sourceApp || "origen no informado"}
            <br />
            Observada: {moment(ev.observedAt)}
          </p>
        ) : (
          <p className="field-hint">
            {gym.item === null
              ? "No se encontró en el informe del gimnasio."
              : "Sin evidencia del gimnasio para este mes."}
          </p>
        )}
        <div className="operator-form-actions">
          <Link
            className="button secondary"
            to={`/operador/gimnasio?period=${encodeURIComponent(period)}&empleado=${id}`}
          >
            Ver evidencia
          </Link>
        </div>
      </section>
    </div>
  );
}

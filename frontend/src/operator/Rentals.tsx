import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { useSearchParams } from "react-router-dom";
import {
  ArrowRight,
  CalendarDays,
  Check,
  Copy,
  LogIn,
  LogOut,
  RefreshCw,
  Search,
  X,
} from "lucide-react";
import {
  dateError,
  dateLabel,
  money,
  statusLabels,
  today,
  validID,
} from "../domain";
import type { Reservation } from "../domain";
import { ApiError } from "../api";
import {
  Action,
  Busy,
  Heading,
  Issue,
  Success,
  checked,
  isReservation,
  uncertain,
  useCommand,
  useLoad,
  useOperator,
} from "./shared";

type Transition = "pickup" | "return" | "cancel";
const transitionLabel = {
  pickup: "Registrar retiro",
  return: "Registrar devolución",
  cancel: "Cancelar reserva",
};
export function Reservations() {
  const [params, setParams] = useSearchParams();
  const selected = params.get("id") || "";
  const [id, setId] = useState(selected);
  const { call } = useOperator();
  const {
    data: r,
    error: loadError,
    loading,
    reload,
  } = useLoad<Reservation>(
    selected ? `/v1/reservations/${encodeURIComponent(selected)}` : null,
    isReservation,
  );
  const [error, setError] = useState<unknown>();
  const [success, setSuccess] = useState("");
  const [confirm, setConfirm] = useState<Transition | null>(null);
  const [unknown, setUnknown] = useState(false);
  const command = useCommand();
  const dialog = useRef<HTMLDialogElement>(null);
  const trigger = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    if (confirm) dialog.current?.showModal();
    else if (dialog.current?.open) {
      dialog.current.close();
      trigger.current?.focus();
    }
  }, [confirm]);
  useEffect(() => {
    setId(selected);
    setError(undefined);
    setSuccess("");
    setUnknown(false);
  }, [selected]);
  function lookup(e: FormEvent) {
    e.preventDefault();
    setError(undefined);
    setSuccess("");
    if (!validID(id.trim())) {
      setError(
        new Error(
          "Ingresá un código de reserva válido de hasta 64 caracteres.",
        ),
      );
      return;
    }
    if (id.trim() === selected) reload();
    else setParams({ id: id.trim() });
  }
  async function transition() {
    if (!r || !confirm) return;
    const action = confirm;
    setError(undefined);
    setSuccess("");
    await command.run(async () => {
      try {
        const result = checked(
          await call<Reservation>(
            `/v1/reservations/${encodeURIComponent(r.id)}${action === "cancel" ? "" : `/${action}`}`,
            { method: action === "cancel" ? "DELETE" : "POST" },
          ),
          isReservation,
        );
        const expected =
          action === "pickup"
            ? "ACTIVE"
            : action === "return"
              ? "COMPLETED"
              : "CANCELLED";
        if (result.id !== r.id || result.status !== expected)
          throw new ApiError(0, "INVALID_RESPONSE");
        setSuccess(
          action === "pickup"
            ? "Retiro registrado. El alquiler está en curso."
            : action === "return"
              ? "Devolución registrada. El alquiler finalizó."
              : "Reserva cancelada.",
        );
        setUnknown(false);
        setConfirm(null);
        reload();
      } catch (err) {
        setError(err);
        setUnknown(uncertain(err));
        setConfirm(null);
        reload();
      }
    });
  }
  function ask(action: Transition, button: HTMLButtonElement) {
    trigger.current = button;
    setConfirm(action);
  }
  return (
    <>
      <Heading title="Cada viaje, acompañado." />
      <form className="operator-toolbar" onSubmit={lookup}>
        <label className="operator-search">
          Código de reserva
          <input
            value={id}
            onChange={(e) => setId(e.target.value)}
            maxLength={64}
            required
            placeholder="Pegá el código de la reserva"
          />
        </label>
        <Action type="submit">
          <Search size={17} />
          Consultar reserva
        </Action>
      </form>
      <p className="field-hint operator-hint">
        Las reservas se consultan por código. La API no ofrece un listado
        general. El retiro y la devolución siempre se registran con
        confirmación.
      </p>
      {!!error && <Issue error={error} />}{" "}
      {!!success && <Success>{success}</Success>}
      {unknown && (
        <div className="notice warning">
          La operación pudo completarse aunque su respuesta no llegó. Revisá el
          estado actualizado antes de volver a operar.
          <button
            type="button"
            className="text-button"
            onClick={() => {
              setUnknown(false);
              reload();
            }}
          >
            Volver a consultar el estado
          </button>
        </div>
      )}
      {loading ? (
        <Busy />
      ) : loadError ? (
        <Issue error={loadError} retry={reload} />
      ) : r ? (
        <>
          <section className="operator-reservation">
            <div className="operator-reservation-head">
              <div>
                <span className="eyebrow">RESERVA CONSULTADA</span>
                <h2>{r.id}</h2>
              </div>
              <span className={`status status-${r.status.toLowerCase()}`}>
                <span />
                {statusLabels[r.status]}
              </span>
            </div>
            <div className="operator-reservation-facts">
              <div>
                <small>CLIENTE</small>
                <strong>{r.customerId}</strong>
              </div>
              <div>
                <small>VEHÍCULO</small>
                <strong>{r.vehicleId}</strong>
              </div>
              <div>
                <small>RETIRO</small>
                <strong>{dateLabel(r.startsOn)}</strong>
              </div>
              <div>
                <small>DEVOLUCIÓN</small>
                <strong>{dateLabel(r.endsOn)}</strong>
              </div>
            </div>
            <div className="operator-rental-total">
              <span>
                {r.days} días · {money(r.dailyRateMinor)} / día
              </span>
              <strong>{money(r.totalMinor)}</strong>
            </div>
            <div className="operator-form-actions">
              <Action
                className="secondary"
                onClick={() => {
                  setUnknown(false);
                  reload();
                }}
              >
                Actualizar estado <RefreshCw size={16} />
              </Action>
              {r.status === "RESERVED" && (
                <>
                  <Action
                    className="secondary"
                    disabled={unknown || command.busy}
                    onClick={(e) => ask("cancel", e.currentTarget)}
                  >
                    Cancelar reserva
                  </Action>
                  <Action
                    disabled={unknown || command.busy}
                    onClick={(e) => ask("pickup", e.currentTarget)}
                  >
                    Registrar retiro <LogOut size={17} />
                  </Action>
                </>
              )}
              {r.status === "ACTIVE" && (
                <Action
                  disabled={unknown || command.busy}
                  onClick={(e) => ask("return", e.currentTarget)}
                >
                  Registrar devolución <LogIn size={17} />
                </Action>
              )}
            </div>
            <p className="field-hint">
              Al retirar se verifican la fecha vigente, la habilitación del
              conductor y la ocupación del vehículo. Llegar a la fecha de
              devolución no finaliza el alquiler automáticamente.
            </p>
          </section>
          <dialog
            ref={dialog}
            className="cancel-dialog"
            aria-labelledby="operator-confirm-title"
            onCancel={(e) => {
              if (command.busy) e.preventDefault();
              else setConfirm(null);
            }}
            onClose={() => {
              setConfirm(null);
              trigger.current?.focus();
            }}
          >
            <button
              className="icon-button dialog-close"
              disabled={command.busy}
              aria-label="Cerrar confirmación"
              onClick={() => setConfirm(null)}
            >
              <X size={20} />
            </button>
            <span className="lookup-icon">
              <CalendarDays size={26} />
            </span>
            <h2 id="operator-confirm-title">
              {confirm ? transitionLabel[confirm] : ""}
            </h2>
            <p>
              Reserva <strong className="break-word">{r.id}</strong>
              <br />
              Cliente: {r.customerId}
              <br />
              Vehículo: {r.vehicleId}
            </p>
            <p>
              {confirm === "pickup"
                ? "Confirmá que el vehículo se entrega al conductor."
                : confirm === "return"
                  ? "Confirmá que el vehículo ya fue devuelto."
                  : "Confirmá que querés cancelar esta reserva antes de su retiro."}
            </p>
            <div className="dialog-actions">
              <Action
                className="secondary"
                disabled={command.busy}
                onClick={() => setConfirm(null)}
              >
                Volver
              </Action>
              <Action busy={command.busy} onClick={() => void transition()}>
                Confirmar operación <Check size={17} />
              </Action>
            </div>
          </dialog>
        </>
      ) : (
        <div className="operator-empty">
          <CalendarDays size={38} />
          <h2>Un código, todo el recorrido.</h2>
          <p>
            Buscá una reserva para consultar su estado y registrar el retiro o
            la devolución.
          </p>
        </div>
      )}
    </>
  );
}

const maintenanceKey = "carcruds.operator.maintenance.v1";
type MaintenanceDraft = { vehicleId: string; startsOn: string; endsOn: string };
function readMaintenance(): MaintenanceDraft | null {
  const raw = sessionStorage.getItem(maintenanceKey);
  if (!raw) return null;
  const v = JSON.parse(raw) as MaintenanceDraft;
  if (
    !v ||
    !validID(v.vehicleId) ||
    !/^\d{4}-\d{2}-\d{2}$/.test(v.startsOn) ||
    !/^\d{4}-\d{2}-\d{2}$/.test(v.endsOn)
  )
    throw new Error(
      "No se pudo leer el mantenimiento pendiente. Consultá al equipo antes de enviar otro bloqueo.",
    );
  return v;
}
export function Maintenance() {
  const [initial] = useState(() => {
    try {
      return { pending: readMaintenance(), error: undefined };
    } catch (err) {
      return { pending: null, error: err };
    }
  });
  const [vehicle, setVehicle] = useState(initial.pending?.vehicleId || "");
  const [start, setStart] = useState(initial.pending?.startsOn || "");
  const [end, setEnd] = useState(initial.pending?.endsOn || "");
  const [pending, setPending] = useState<MaintenanceDraft | null>(
    initial.pending,
  );
  const [error, setError] = useState<unknown>(initial.error);
  const [saved, setSaved] = useState<{
    id: string;
    vehicleId: string;
    startsOn: string;
    endsOn: string;
  } | null>(null);
  const { call } = useOperator();
  const command = useCommand();
  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(undefined);
    await command.run(async () => {
      if (pending || saved || initial.error) return;
      const body = { vehicleId: vehicle.trim(), startsOn: start, endsOn: end };
      const validation = dateError(start, end);
      if (!validID(body.vehicleId) || validation) {
        setError(
          new Error(
            validation || "Ingresá un identificador de vehículo válido.",
          ),
        );
        return;
      }
      try {
        sessionStorage.setItem(maintenanceKey, JSON.stringify(body));
        setPending(body);
      } catch {
        setError(
          new Error(
            "No se pudo conservar la referencia de esta operación. No enviamos el mantenimiento.",
          ),
        );
        return;
      }
      try {
        const result = await call<{
          id: string;
          vehicleId: string;
          startsOn: string;
          endsOn: string;
        }>("/v1/maintenance", { method: "POST", body: JSON.stringify(body) });
        if (
          !result ||
          !validID(result.id) ||
          result.vehicleId !== body.vehicleId ||
          result.startsOn !== start ||
          result.endsOn !== end
        )
          throw new ApiError(0, "INVALID_RESPONSE");
        setSaved(result);
        sessionStorage.removeItem(maintenanceKey);
        setPending(null);
      } catch (err) {
        setError(err);
        if (
          err instanceof ApiError &&
          [400, 403, 409, 422].includes(err.status)
        ) {
          try {
            sessionStorage.removeItem(maintenanceKey);
            setPending(null);
          } catch {
            /* Se conserva el bloqueo local hasta revisión. */
          }
        }
      }
    });
  }
  return (
    <>
      <Heading title="Preparados para lo que viene." />
      <div className="operator-split">
        <form className="operator-form-card" onSubmit={submit}>
          <div className="operator-section-label">PROGRAMAR MANTENIMIENTO</div>
          <label>
            Identificador del vehículo
            <input
              value={vehicle}
              onChange={(e) => setVehicle(e.target.value)}
              disabled={!!pending || !!saved}
              maxLength={64}
              required
              placeholder="Ej. auto-001"
            />
          </label>
          <div className="operator-fields">
            <label>
              Inicio del mantenimiento
              <input
                type="date"
                min={today()}
                value={start}
                onChange={(e) => setStart(e.target.value)}
                disabled={!!pending || !!saved}
                required
              />
            </label>
            <label>
              Fin del mantenimiento
              <input
                type="date"
                min={start || today()}
                value={end}
                onChange={(e) => setEnd(e.target.value)}
                disabled={!!pending || !!saved}
                required
              />
            </label>
          </div>
          <p className="field-hint">
            De 1 a 30 días. El día de finalización queda disponible para nuevas
            reservas. Un conflicto de agenda no cancela reservas existentes.
          </p>
          {!!error && <Issue error={error} />}{" "}
          {saved && (
            <Success>
              <strong>Mantenimiento registrado.</strong>
              <p>
                Código: {saved.id}
                <br />
                Vehículo: {saved.vehicleId}
                <br />
                {dateLabel(saved.startsOn)} al {dateLabel(saved.endsOn)}
              </p>
            </Success>
          )}
          {pending && !saved && (
            <div className="notice warning">
              Hay un bloqueo enviado cuyo resultado todavía no se confirmó.
              Conservamos vehículo y fechas en esta pestaña y no permitimos
              repetirlo. La API no tiene consulta ni cancelación de
              mantenimiento: el equipo debe verificarlo antes de iniciar otra
              operación.
            </div>
          )}
          <Action
            type="submit"
            busy={command.busy}
            disabled={!!pending || !!saved || !!initial.error}
          >
            Registrar mantenimiento <ArrowRight size={17} />
          </Action>
          {saved && (
            <Action
              type="button"
              className="secondary"
              onClick={() => {
                setSaved(null);
                setStart("");
                setEnd("");
                setError(undefined);
              }}
            >
              Programar otro período
            </Action>
          )}
        </form>
        <aside className="operator-explainer">
          <CalendarDays size={32} />
          <h2>
            Cuidar también
            <br />
            es planificar.
          </h2>
          <p>
            El bloqueo ocupa la agenda del vehículo para evitar nuevas reservas
            incompatibles.
          </p>
          <ul>
            <li>No modifica reservas existentes.</li>
            <li>Se confirma con la agenda real del servicio.</li>
            <li>Si la respuesta no llega, no se repite a ciegas.</li>
          </ul>
        </aside>
      </div>
    </>
  );
}

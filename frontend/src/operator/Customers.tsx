import { useState } from "react";
import type { FormEvent } from "react";
import { Link } from "react-router-dom";
import { Plus, Search, ArrowRight } from "lucide-react";
import { dateLabel, validID } from "../domain";
import type { Customer } from "../domain";
import {
  Action,
  Back,
  Busy,
  Heading,
  Issue,
  Success,
  checked,
  isCustomer,
  uncertain,
  useCommand,
  useLoad,
  useOperator,
} from "./shared";

const isList = (data: { items: Customer[] }) =>
  !!data && Array.isArray(data.items) && data.items.every(isCustomer);
export function Customers() {
  const { data, error, loading, reload } = useLoad<{ items: Customer[] }>(
    "/v1/customers",
    isList,
  );
  const { id, setId } = useID();
  const [result, setResult] = useState<Customer | null>(null);
  const [lookupError, setLookupError] = useState<unknown>();
  const { call } = useOperator();
  const command = useCommand();
  async function lookup(e: FormEvent) {
    e.preventDefault();
    setLookupError(undefined);
    setResult(null);
    await command.run(async () => {
      try {
        if (!validID(id.trim()))
          throw new Error("Ingresá un identificador de cliente válido.");
        setResult(
          checked(
            await call<Customer>(
              `/v1/customers/${encodeURIComponent(id.trim())}`,
            ),
            isCustomer,
          ),
        );
      } catch (err) {
        setLookupError(err);
      }
    });
  }
  return (
    <>
      <Heading title="Personas que hacen camino.">
        <Link to="/operador/clientes/nuevo" className="button">
          <Plus size={17} />
          Registrar cliente
        </Link>
      </Heading>
      <form className="operator-toolbar" onSubmit={lookup}>
        <label className="operator-search">
          Consultar cliente por ID
          <input
            value={id}
            onChange={(e) => setId(e.target.value)}
            placeholder="Ej. cli-001"
            maxLength={64}
            required
          />
        </label>
        <Action type="submit" busy={command.busy}>
          <Search size={17} />
          Consultar
        </Action>
      </form>
      {!!lookupError && <Issue error={lookupError} />}{" "}
      {result && (
        <Success>
          <strong>{result.name}</strong> · {result.id} ·{" "}
          {result.active ? "Activo" : "Inactivo"} · Licencia hasta{" "}
          {dateLabel(result.licenseValidUntil)}
        </Success>
      )}
      <p className="operator-hint field-hint">
        Se muestran hasta 200 perfiles. La consulta por ID permite buscar otros
        registros. La API actual ofrece alta y consulta; no permite editar ni
        eliminar clientes.
      </p>
      {loading ? (
        <Busy />
      ) : error ? (
        <Issue error={error} retry={reload} />
      ) : (
        data && (
          <div className="operator-table-wrap">
            <table className="operator-table">
              <caption className="sr-only">Clientes registrados</caption>
              <thead>
                <tr>
                  <th>Cliente</th>
                  <th>Estado</th>
                  <th>Licencia válida hasta</th>
                </tr>
              </thead>
              <tbody>
                {data.items.map((c) => (
                  <tr key={c.id}>
                    <td>
                      <strong>{c.name}</strong>
                      <small>{c.id}</small>
                    </td>
                    <td>
                      <span
                        className={`operator-state ${c.active ? "" : "inactive"}`}
                      >
                        {c.active ? "Activo" : "Inactivo"}
                      </span>
                    </td>
                    <td>{dateLabel(c.licenseValidUntil)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            {!data.items.length && (
              <div className="empty-state">
                <h2>Todavía no hay clientes registrados.</h2>
              </div>
            )}
          </div>
        )
      )}
    </>
  );
}
function useID() {
  const [id, setId] = useState("");
  return { id, setId };
}
export function CustomerCreate() {
  const [id, setId] = useState(() => `cli-${crypto.randomUUID().slice(0, 8)}`);
  const [name, setName] = useState("");
  const [license, setLicense] = useState("");
  const [active, setActive] = useState(true);
  const [saved, setSaved] = useState<Customer | null>(null);
  const [error, setError] = useState<unknown>();
  const [unknown, setUnknown] = useState(false);
  const { call } = useOperator();
  const command = useCommand();
  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(undefined);
    await command.run(async () => {
      try {
        if (!validID(id))
          throw new Error(
            "Ingresá un identificador válido de hasta 64 caracteres.",
          );
        if (unknown || saved) return;
        setSaved(
          checked(
            await call<Customer>("/v1/customers", {
              method: "POST",
              body: JSON.stringify({
                id,
                name: name.trim(),
                licenseValidUntil: license,
                active,
              }),
            }),
            isCustomer,
          ),
        );
      } catch (err) {
        setError(err);
        if (uncertain(err)) setUnknown(true);
      }
    });
  }
  async function recover() {
    setError(undefined);
    await command.run(async () => {
      try {
        setSaved(
          checked(
            await call<Customer>(`/v1/customers/${encodeURIComponent(id)}`),
            isCustomer,
          ),
        );
        setUnknown(false);
      } catch (err) {
        setError(err);
      }
    });
  }
  return (
    <>
      <Back to="/operador/clientes" children="Volver a clientes" />
      <Heading title="Un nuevo conductor." />
      <form className="operator-form-card" onSubmit={submit}>
        <div className="operator-section-label">DATOS DEL CLIENTE</div>
        <div className="operator-fields">
          <label>
            Identificador del cliente
            <input
              value={id}
              onChange={(e) => setId(e.target.value)}
              disabled={unknown || !!saved || command.busy}
              maxLength={64}
              required
            />
          </label>
          <label>
            Nombre completo
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={200}
              required
            />
          </label>
          <label>
            Licencia válida hasta
            <input
              type="date"
              value={license}
              onChange={(e) => setLicense(e.target.value)}
              required
            />
          </label>
          <label className="operator-checkbox">
            <input
              type="checkbox"
              checked={active}
              onChange={(e) => setActive(e.target.checked)}
            />
            Cliente activo
          </label>
        </div>
        <p className="field-hint">
          Registrar la fecha de licencia no garantiza su vigencia. Alquileres
          vuelve a verificar la habilitación para el período solicitado y al
          retirar.
        </p>
        {!!error && <Issue error={error} />}{" "}
        {saved && (
          <Success>
            <strong>Perfil disponible: {saved.name}</strong>
            <p>
              Identificador: {saved.id} · {saved.active ? "Activo" : "Inactivo"}{" "}
              · Licencia hasta {dateLabel(saved.licenseValidUntil)}. Revisá los
              datos antes de operar.
            </p>
          </Success>
        )}
        {unknown && (
          <div className="notice warning">
            El alta pudo haberse guardado. Conservá este identificador y
            consultalo antes de intentar otra creación.
            <button
              type="button"
              className="text-button"
              disabled={command.busy}
              onClick={() => void recover()}
            >
              Consultar cliente por ID
            </button>
          </div>
        )}
        <div className="operator-form-actions">
          <Link to="/operador/clientes" className="button secondary">
            Volver a clientes
          </Link>
          <Action
            type="submit"
            busy={command.busy}
            disabled={unknown || !!saved}
          >
            Registrar cliente <ArrowRight size={17} />
          </Action>
        </div>
      </form>
    </>
  );
}

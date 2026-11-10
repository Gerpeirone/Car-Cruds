import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import {
  Link,
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import { ArrowRight, ArrowUpRight, Plus, Search } from "lucide-react";
import { money, validID } from "../domain";
import type { Vehicle, VehiclePage } from "../domain";
import { ApiError } from "../api";
import {
  Action,
  Back,
  Busy,
  Heading,
  Issue,
  Success,
  checked,
  isVehicle,
  parseRate,
  uncertain,
  useCommand,
  useLoad,
  useOperator,
} from "./shared";

const isPage = (p: VehiclePage) =>
  !!p &&
  Array.isArray(p.items) &&
  p.items.every(isVehicle) &&
  Number.isSafeInteger(p.total);
export function Fleet() {
  const [params, setParams] = useSearchParams();
  const [brand, setBrand] = useState(params.get("brand") || "");
  const { call } = useOperator();
  const command = useCommand();
  const [reindex, setReindex] = useState(false);
  const [error, setError] = useState<unknown>();
  const query = new URLSearchParams({
    page: params.get("page") || "1",
    pageSize: "20",
    sort: params.get("sort") || "price_asc",
    includeInactive: "true",
  });
  if (params.get("brand")) query.set("brand", params.get("brand")!);
  const {
    data,
    error: loadError,
    loading,
    reload,
  } = useLoad<VehiclePage>(`/v1/vehicles?${query}`, isPage);
  function update(name: string, value: string) {
    const p = new URLSearchParams(params);
    if (value) p.set(name, value);
    else p.delete(name);
    if (name !== "page") p.delete("page");
    setParams(p);
  }
  async function rebuild() {
    setError(undefined);
    await command.run(async () => {
      try {
        const result = await call<{ status: string }>("/v1/vehicles/reindex", {
          method: "POST",
        });
        if (result?.status !== "pending")
          throw new ApiError(0, "INVALID_RESPONSE");
        setReindex(true);
      } catch (err) {
        setError(err);
      }
    });
  }
  return (
    <>
      <Heading title="La flota, a punto.">
        <Link className="button" to="/operador/flota/nuevo">
          <Plus size={17} />
          Registrar vehículo
        </Link>
      </Heading>
      <form
        className="operator-toolbar"
        onSubmit={(e) => {
          e.preventDefault();
          update("brand", brand.trim());
        }}
      >
        <label className="operator-search">
          <span>Filtrar por marca exacta</span>
          <input
            value={brand}
            onChange={(e) => setBrand(e.target.value)}
            placeholder="Ej. Toyota"
            maxLength={128}
          />
        </label>
        <Action type="submit">
          <Search size={17} />
          Buscar
        </Action>
        <label>
          Ordenar
          <select
            value={params.get("sort") || "price_asc"}
            onChange={(e) => update("sort", e.target.value)}
          >
            <option value="price_asc">Menor tarifa</option>
            <option value="price_desc">Mayor tarifa</option>
          </select>
        </label>
        <Action
          type="button"
          className="secondary"
          busy={command.busy}
          onClick={() => void rebuild()}
        >
          Actualizar índice
        </Action>
      </form>
      <p className="field-hint operator-hint">
        Incluye vehículos activos e inactivos. Desactivar un auto conserva sus
        reservas y su historia.
      </p>
      {reindex && (
        <Success>
          Se aceptó la actualización del índice de búsqueda. Puede tardar en
          reflejar los cambios; la aceptación no significa que haya terminado.
        </Success>
      )}
      {!!error && <Issue error={error} />}{" "}
      {loading ? (
        <Busy />
      ) : loadError ? (
        <Issue error={loadError} retry={reload} />
      ) : (
        data && (
          <>
            <div className="operator-table-wrap">
              <table className="operator-table">
                <caption className="sr-only">Vehículos de la flota</caption>
                <thead>
                  <tr>
                    <th>Vehículo</th>
                    <th>Características</th>
                    <th>Tarifa diaria</th>
                    <th>Estado</th>
                    <th>
                      <span className="sr-only">Acciones</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {data.items.map((v) => (
                    <tr key={v.vehicleId}>
                      <td>
                        <strong>
                          {v.brand} {v.model}
                        </strong>
                        <small>{v.vehicleId}</small>
                      </td>
                      <td>
                        {v.category}
                        <small>
                          {v.seats} plazas ·{" "}
                          {v.transmission === "automatic"
                            ? "Automático"
                            : "Manual"}
                        </small>
                      </td>
                      <td>{money(v.dailyRateMinor)}</td>
                      <td>
                        <span
                          className={`operator-state ${v.active ? "" : "inactive"}`}
                        >
                          {v.active ? "Activo" : "Inactivo"}
                        </span>
                      </td>
                      <td>
                        <Link
                          className="text-button"
                          to={`/operador/flota/${v.vehicleId}`}
                        >
                          Editar <ArrowUpRight size={15} />
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {!data.items.length && (
                <div className="empty-state">
                  <h2>No hay vehículos con esa marca.</h2>
                  <p>Probá otro filtro o registrá un vehículo.</p>
                </div>
              )}
            </div>
            <div className="operator-pagination">
              <span>
                {data.total} vehículos · Página {query.get("page")}
              </span>
              <Action
                className="secondary"
                disabled={Number(query.get("page")) <= 1}
                onClick={() =>
                  update("page", String(Number(query.get("page")) - 1))
                }
              >
                Anterior
              </Action>
              <Action
                className="secondary"
                disabled={Number(query.get("page")) * 20 >= data.total}
                onClick={() =>
                  update("page", String(Number(query.get("page")) + 1))
                }
              >
                Siguiente
              </Action>
            </div>
          </>
        )
      )}
    </>
  );
}
type Draft = {
  vehicleId: string;
  brand: string;
  model: string;
  category: string;
  transmission: string;
  seats: string;
  rate: string;
  active: boolean;
};
const initial = (): Draft => ({
  vehicleId: `auto-${crypto.randomUUID().slice(0, 8)}`,
  brand: "",
  model: "",
  category: "",
  transmission: "manual",
  seats: "5",
  rate: "",
  active: true,
});
export function VehicleEditor() {
  const { id } = useParams();
  const creating = !id;
  const navigate = useNavigate();
  const location = useLocation();
  const [createdID, setCreatedID] = useState<string>();
  const { call } = useOperator();
  const {
    data,
    error: loadError,
    loading,
    reload,
  } = useLoad<Vehicle>(
    id ? `/v1/vehicles/${encodeURIComponent(id)}` : null,
    isVehicle,
  );
  const [form, setForm] = useState<Draft>(initial);
  const [loadedVersion, setLoadedVersion] = useState<number | null>(null);
  const [error, setError] = useState<unknown>();
  const [success, setSuccess] = useState(() =>
    location.state?.vehicleCreated ? "Vehículo registrado." : "",
  );
  const [unknown, setUnknown] = useState(false);
  const command = useCommand();
  useEffect(() => {
    if (data) {
      setForm({
        vehicleId: data.vehicleId,
        brand: data.brand,
        model: data.model,
        category: data.category,
        transmission: data.transmission,
        seats: String(data.seats),
        rate: (data.dailyRateMinor / 100).toFixed(2),
        active: data.active,
      });
      setUnknown(false);
      setLoadedVersion(data.version);
    }
  }, [data]);
  function update<K extends keyof Draft>(key: K, value: Draft[K]) {
    setForm((current) => ({ ...current, [key]: value }));
    setSuccess("");
  }
  async function submit(e: FormEvent) {
    e.preventDefault();
    setError(undefined);
    setSuccess("");
    await command.run(async () => {
      try {
        if (unknown)
          throw new Error(
            "Consultá primero el registro antes de volver a guardar.",
          );
        if (!validID(form.vehicleId))
          throw new Error(
            "Usá un identificador de hasta 64 letras, números, guiones o guiones bajos.",
          );
        const payload = {
          vehicleId: form.vehicleId,
          brand: form.brand.trim(),
          model: form.model.trim(),
          category: form.category.trim(),
          transmission: form.transmission,
          seats: Number(form.seats),
          dailyRateMinor: parseRate(form.rate),
          currency: "ARS",
          active: form.active,
          ...(creating ? {} : { version: data?.version }),
        };
        const result = checked(
          await call<Vehicle>(
            creating
              ? "/v1/vehicles"
              : `/v1/vehicles/${encodeURIComponent(id!)}`,
            {
              method: creating ? "POST" : "PUT",
              body: JSON.stringify(payload),
            },
          ),
          isVehicle,
        );
        if (result.vehicleId !== form.vehicleId)
          throw new ApiError(0, "INVALID_RESPONSE");
        if (creating) {
          setCreatedID(result.vehicleId);
          navigate(`/operador/flota/${result.vehicleId}`, {
            replace: true,
            state: { vehicleCreated: true },
          });
        } else reload();
        setSuccess(creating ? "Vehículo registrado." : "Ficha actualizada.");
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
        const result = checked(
          await call<Vehicle>(
            `/v1/vehicles/${encodeURIComponent(form.vehicleId)}`,
          ),
          isVehicle,
        );
        if (creating)
          navigate(`/operador/flota/${result.vehicleId}`, { replace: true });
        else reload();
        setSuccess(
          "Encontramos una ficha con ese ID. Revisá sus datos antes de realizar más cambios.",
        );
        setUnknown(false);
      } catch (err) {
        setError(err);
      }
    });
  }
  if (loading || (creating && createdID)) return <Busy />;
  if (loadError) return <Issue error={loadError} retry={reload} />;
  if (
    !creating &&
    (!data || loadedVersion !== data.version || form.vehicleId !== id)
  )
    return <Busy />;
  return (
    <>
      <Back to="/operador/flota" children="Volver a la flota" />
      <Heading
        title={
          creating ? "Un nuevo compañero de ruta." : "Cada detalle, al día."
        }
      />
      <form className="operator-form-card" onSubmit={submit}>
        <div className="operator-section-label">DATOS DEL VEHÍCULO</div>
        <div className="operator-fields">
          <label>
            Identificador
            <input
              value={form.vehicleId}
              disabled={!creating || unknown || command.busy}
              maxLength={64}
              onChange={(e) => update("vehicleId", e.target.value)}
              required
            />
          </label>
          <label>
            Marca
            <input
              value={form.brand}
              maxLength={128}
              onChange={(e) => update("brand", e.target.value)}
              required
            />
          </label>
          <label>
            Modelo
            <input
              value={form.model}
              maxLength={128}
              onChange={(e) => update("model", e.target.value)}
              required
            />
          </label>
          <label>
            Categoría
            <input
              value={form.category}
              maxLength={64}
              onChange={(e) => update("category", e.target.value)}
              placeholder="Ej. sedan"
              required
            />
          </label>
          <label>
            Transmisión
            <select
              value={form.transmission}
              onChange={(e) => update("transmission", e.target.value)}
            >
              <option value="manual">Manual</option>
              <option value="automatic">Automática</option>
            </select>
          </label>
          <label>
            Plazas
            <input
              type="number"
              min={1}
              max={9}
              value={form.seats}
              onChange={(e) => update("seats", e.target.value)}
              required
            />
          </label>
          <label>
            Tarifa diaria en pesos (ARS)
            <input
              inputMode="decimal"
              value={form.rate}
              onChange={(e) => update("rate", e.target.value)}
              placeholder="Ej. 50000,00"
              required
            />
          </label>
          <label className="operator-checkbox">
            <input
              type="checkbox"
              checked={form.active}
              onChange={(e) => update("active", e.target.checked)}
            />
            Disponible para nuevas reservas
          </label>
        </div>
        <p className="field-hint">
          La tarifa vigente no modifica el precio de reservas ya confirmadas. La
          baja es lógica y conserva el historial.
        </p>
        {!!error && <Issue error={error} />}{" "}
        {!!success && <Success>{success}</Success>}
        {error instanceof ApiError && error.code === "VERSION_CONFLICT" && (
          <div className="notice warning">
            Otra operación pudo cambiar esta ficha. Tus cambios se conservaron
            en el formulario.{" "}
            <button type="button" className="text-button" onClick={reload}>
              Descartar mis cambios y cargar ficha actual
            </button>
          </div>
        )}
        {unknown && (
          <div className="notice warning">
            El resultado no se confirmó. No repetimos la escritura.{" "}
            <button
              type="button"
              className="text-button"
              disabled={command.busy}
              onClick={() => void recover()}
            >
              Consultar registro por ID
            </button>
          </div>
        )}
        <div className="operator-form-actions">
          <Link to="/operador/flota" className="button secondary">
            Volver
          </Link>
          <Action type="submit" busy={command.busy} disabled={unknown}>
            {creating ? "Registrar vehículo" : "Guardar cambios"}
            <ArrowRight size={17} />
          </Action>
        </div>
      </form>
    </>
  );
}

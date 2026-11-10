export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    public requestID = "",
  ) {
    super(code);
    this.name = "ApiError";
  }
}
export async function request<T>(
  path: string,
  token: string,
  init: RequestInit = {},
): Promise<T> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 10000);
  const abort = () => controller.abort();
  init.signal?.addEventListener("abort", abort, { once: true });
  if (init.signal?.aborted) controller.abort();
  try {
    const response = await fetch(path, {
      ...init,
      signal: controller.signal,
      credentials: "omit",
      cache: "no-store",
      redirect: "error",
      headers: {
        Accept: "application/json",
        Authorization: `Bearer ${token}`,
        ...(init.body ? { "Content-Type": "application/json" } : {}),
        ...init.headers,
      },
    });
    if (response.status === 204) return undefined as T;
    let data: unknown;
    try {
      data = await response.json();
    } catch {
      throw new ApiError(
        response.ok ? 0 : response.status,
        "INVALID_RESPONSE",
        response.headers.get("X-Request-ID") || "",
      );
    }
    if (!response.ok)
      throw new ApiError(
        response.status,
        (data as { code?: string })?.code || "UNKNOWN",
        response.headers.get("X-Request-ID") || "",
      );
    return data as T;
  } catch (err) {
    if (err instanceof ApiError) throw err;
    throw new ApiError(0, "NETWORK_ERROR");
  } finally {
    clearTimeout(timeout);
    init.signal?.removeEventListener("abort", abort);
  }
}
const messages: Record<string, string> = {
  UNAUTHORIZED:
    "Tu acceso no es válido o dejó de estar disponible. Volvé a ingresar.",
  FORBIDDEN: "Este acceso no tiene permiso para realizar la acción.",
  INVALID_INPUT: "Revisá los datos ingresados e intentá nuevamente.",
  NOT_FOUND: "No encontramos ese recurso para tu acceso.",
  VEHICLE_NOT_FOUND: "Este auto ya no está disponible.",
  VEHICLE_UNAVAILABLE:
    "Alguien se adelantó: el auto no está disponible para esas fechas. Elegí otro período o vehículo.",
  PRICE_CHANGED:
    "La tarifa cambió. Volvé a consultar disponibilidad y revisá el nuevo total antes de confirmar.",
  IDEMPOTENCY_KEY_REUSED:
    "La referencia de esta operación ya está asociada a otros datos. Conservamos tu operación; pedí ayuda antes de crear otra.",
  INVALID_STATE:
    "El estado de la reserva cambió y ya no permite esta acción. Consultá su estado actual.",
  CUSTOMER_NOT_ELIGIBLE:
    "El conductor no está habilitado para este alquiler. Consultá el estado de tu perfil con el equipo.",
  SEARCH_UNAVAILABLE:
    "La búsqueda está temporalmente fuera de servicio. Podés reintentar o consultar el catálogo sin texto.",
  NETWORK_ERROR:
    "No pudimos completar la conexión. Revisá tu conexión e intentá nuevamente.",
  INVALID_RESPONSE:
    "No pudimos interpretar la respuesta del servicio. Intentá consultar nuevamente.",
  VERSION_CONFLICT:
    "La ficha cambió o ese identificador ya existe. Consultá el registro actual antes de volver a guardar.",
  CONFLICT:
    "Ya existe un cliente con ese identificador. Consultá sus datos antes de registrar otro.",
};
export function errorMessage(error: unknown): string {
  if (error instanceof ApiError)
    return (
      messages[error.code] ||
      (error.status === 404
        ? messages.NOT_FOUND
        : error.status === 503
          ? "El servicio está temporalmente ocupado o no disponible. Intentá nuevamente en un momento."
          : "No pudimos completar la operación. Intentá nuevamente.")
    );
  return error instanceof Error
    ? error.message
    : "Ocurrió un problema inesperado.";
}

"""Verifica OpenAPI, ejemplos y cobertura de las rutas HTTP implementadas."""

import hashlib
import json
from pathlib import Path
import re
import sys

from openapi_schema_validator import OAS30Validator
from openapi_spec_validator import validate as validate_spec

ROOT = Path(__file__).resolve().parents[1]
METHODS = {"get", "post", "put", "delete", "patch", "head", "options"}


def resolve(doc, value):
    if isinstance(value, dict) and "$ref" in value:
        pointer = value["$ref"]
        if not pointer.startswith("#/"):
            raise ValueError("Los contratos propios deben resolver sin red: " + pointer)
        target = doc
        for part in pointer[2:].split("/"):
            target = target[part.replace("~1", "/").replace("~0", "~")]
        return resolve(doc, target)
    if isinstance(value, dict):
        return {key: resolve(doc, item) for key, item in value.items()}
    if isinstance(value, list):
        return [resolve(doc, item) for item in value]
    return value


def check_examples(schema, value, context):
    validator = OAS30Validator(schema, format_checker=OAS30Validator.FORMAT_CHECKER)
    for problem in validator.iter_errors(value):
        raise ValueError(f"{context}: ejemplo incompatible: {problem.message}")


def examples_in(node, context):
    if "example" in node:
        yield node["example"]
    for name, example in node.get("examples", {}).items():
        if "value" not in example:
            raise ValueError(f"{context}/{name}: se requiere un ejemplo local")
        yield example["value"]


def implemented_routes():
    routes = set()
    # Flota, Clientes, Alquileres y la plataforma declaran patrones con método.
    for name in (
        "services/fleet/http.go",
        "services/customers/presentation/http.go",
        "services/rentals/adapters/httpapi/handler.go",
        "internal/platform/runtime.go",
    ):
        text = (ROOT / name).read_text(encoding="utf-8")
        routes.update(re.findall(r'"(GET|POST|PUT|DELETE|PATCH) (/(?:v1|internal)/[^"\s]+)"', text))
    # Empleados registra funciones y discrimina el método en cada función.
    directory = ROOT / "services/employees/adapters/httpapi"
    text = "\n".join(path.read_text(encoding="utf-8") for path in sorted(directory.glob("*.go")) if not path.name.endswith("_test.go"))
    functions = {}
    for match in re.finditer(r"(?ms)^func \(h \*Handler\) (\w+)\([^\n]+\n(.*?)(?=^func |\Z)", text):
        functions[match[1]] = match[2]
    methods = {"Get": "GET", "Post": "POST", "Put": "PUT", "Delete": "DELETE", "Patch": "PATCH"}
    for path, function in re.findall(r'mux.HandleFunc\("(/v1/[^"\s]+)", h\.(\w+)\)', text):
        source = functions[function]
        paths = [path]
        if "{action}" in path:
            paths = [path.replace("{action}", action) for action in re.findall(r'case "(\w+)":', source)]
        for method in set(re.findall(r"http.Method(Get|Post|Put|Delete|Patch)\b", source)):
            routes.update((methods[method], route) for route in paths)
    return routes


def main():
    catalog = json.loads((ROOT / "docs/api/catalog.json").read_text(encoding="utf-8"))
    routes = set()
    ids = set()
    examples = 0
    for entry in catalog:
        name = entry["file"]
        original = json.loads((ROOT / "docs/contracts" / name).read_text(encoding="utf-8"))
        # Resolver primero impide que el validador busque referencias en la red.
        doc = resolve(original, original)
        validate_spec(original)
        for path, item in doc["paths"].items():
            for method, op in item.items():
                if method not in METHODS:
                    continue
                context = f"{name}: {method.upper()} {path}"
                route = (method.upper(), path)
                if route in routes:
                    raise ValueError(context + ": operación duplicada")
                routes.add(route)
                oid = op.get("operationId")
                if not oid or oid in ids:
                    raise ValueError(context + ": operationId ausente o repetido")
                ids.add(oid)
                for field in ("summary", "description", "tags", "x-roles"):
                    if not op.get(field):
                        raise ValueError(context + ": falta " + field)
                for param in item.get("parameters", []) + op.get("parameters", []):
                    if not param.get("description"):
                        raise ValueError(context + ": parámetro sin explicación")
                    for value in examples_in(param, context):
                        check_examples(param["schema"], value, context)
                        examples += 1
                request = op.get("requestBody", {})
                for media in request.get("content", {}).values():
                    values = list(examples_in(media, context))
                    if not values:
                        raise ValueError(context + ": cuerpo sin ejemplo")
                    for value in values:
                        check_examples(media["schema"], value, context)
                        examples += 1
                for status, response in op["responses"].items():
                    if not response.get("description") or response["description"] == "Consultar estado y documentación de la operación.":
                        raise ValueError(context + ": respuesta sin explicación concreta")
                    for media in response.get("content", {}).values():
                        values = list(examples_in(media, context))
                        if status.startswith("2") and not values:
                            raise ValueError(context + ": resultado satisfactorio sin ejemplo")
                        for value in values:
                            check_examples(media["schema"], value, context)
                            examples += 1
        print(f"OK {name}")
    actual = implemented_routes()
    if routes != actual:
        raise ValueError(f"Cobertura distinta del código. Sin documentar: {sorted(actual - routes)}; sin ruta implementada: {sorted(routes - actual)}")
    guide = (ROOT / "docs/API.md").read_text(encoding="utf-8")
    guide_routes = set(re.findall(r"`(GET|POST|PUT|DELETE|PATCH) (/(?:v1|internal)/[^`\s]+)`", guide))
    if guide_routes != routes:
        raise ValueError(f"Inventario de la guía incompleto: {sorted(routes - guide_routes)}; sobrante: {sorted(guide_routes - routes)}")
    vendor = ROOT / "docs/api/vendor"
    provenance = json.loads((vendor / "provenance.json").read_text(encoding="utf-8"))
    for name, digest in provenance["filesSHA256"].items():
        if hashlib.sha256((vendor / name).read_bytes()).hexdigest() != digest:
            raise ValueError("Recurso Swagger alterado: " + name)
    print(f"OK {len(routes)} operaciones, {examples} ejemplos y recursos Swagger UI {provenance['version']} verificados.")


if __name__ == "__main__":
    try:
        main()
    except Exception as problem:
        print(str(problem), file=sys.stderr)
        sys.exit(1)

"""Validate rendered Compose on stdin; never print environment values."""
import json
import sys


def validate(config):
    services = config["services"]
    for name, service in services.items():
        if name == "rabbit-init":
            if service.get("restart", "no") != "no":
                raise ValueError("rabbit-init debe ejecutarse una sola vez")
            continue
        if service.get("restart") != "unless-stopped":
            raise ValueError(f"{name}: falta reinicio automático")
        for port in service.get("ports", []):
            if name != "caddy" and port.get("host_ip") != "127.0.0.1":
                raise ValueError(f"{name}: puerto de soporte expuesto fuera de loopback")
    caddy = services["caddy"]
    if {str(port["published"]) for port in caddy["ports"]} != {"80", "443"}:
        raise ValueError("Caddy debe publicar únicamente 80 y 443")
    if caddy["depends_on"]["gateway"]["condition"] != "service_healthy":
        raise ValueError("Caddy debe esperar al gateway saludable")
    if not caddy["environment"].get("PUBLIC_HOST"):
        raise ValueError("falta PUBLIC_HOST")


if __name__ == "__main__":
    try:
        validate(json.load(sys.stdin))
    except (ValueError, KeyError, TypeError):
        print("FAIL: configuración de despliegue inválida; revisar reinicios, puertos y dependencias", file=sys.stderr)
        sys.exit(1)
    print("PASS: reinicio de servicios permanentes, puertos y dependencia de Caddy")

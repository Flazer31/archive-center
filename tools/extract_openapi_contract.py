#!/usr/bin/env python3
"""Extract OpenAPI contract from Archive Center Beta 0.8(fix) backend.

This tool imports the FastAPI application read-only and produces a summary
report of routes, schemas, and operation counts. It is safe to run against
the current runtime because it does not send traffic or mutate state.

Usage:
    PYTHONDONTWRITEBYTECODE=1 python tools/extract_openapi_contract.py
    PYTHONDONTWRITEBYTECODE=1 python tools/extract_openapi_contract.py --format markdown
    PYTHONDONTWRITEBYTECODE=1 python tools/extract_openapi_contract.py --format json --out openapi.json
    PYTHONDONTWRITEBYTECODE=1 python tools/extract_openapi_contract.py --schema-out schema-freeze.json --inventory-out inventory.md
"""

import argparse
import json
import os
import sys
import traceback
import warnings
from pathlib import Path

# Prevent bytecode creation before any imports.
sys.dont_write_bytecode = True


# API key fields that may block import if missing or encrypted.
_DUMMY_API_KEYS = [
    "PROJECT_MAIN_API_KEY",
    "PROJECT_SUPERVISOR_API_KEY",
    "PROJECT_CRITIC_API_KEY",
    "PROJECT_EMBEDDING_API_KEY",
]

# Environment variable that must NOT be set because it triggers .env mutation.
_RECOVERY_ENV_KEY = "RISUAI_ALLOW_ENCRYPTED_KEY_RECOVERY"

_DEFAULT_SOURCE_ROOT = r"M:\risulongmemory\Archive Center Beta 0.8(fix)"


def _normalize_message(msg: str) -> str:
    """Collapse whitespace and newlines into a single line."""
    return " ".join(msg.split())


def _prepare_env():
    """Set process-only dummy overrides so the 0.8 backend can be imported
    without requiring a real .env or vault. No files are written."""
    for key in _DUMMY_API_KEYS:
        if not os.getenv(key):
            os.environ[key] = "DUMMY_KEY_FOR_EXTRACTOR"
    if not os.getenv("DATABASE_URL"):
        os.environ["DATABASE_URL"] = "sqlite:///:memory:"
    # Ensure recovery mode cannot trigger .env mutation.
    os.environ.pop(_RECOVERY_ENV_KEY, None)


def _import_app(source_root: str):
    """Import the FastAPI app from the 0.8 backend.

    Returns the app object or raises RuntimeError with a clear message.
    """
    source_root = os.path.abspath(source_root)
    backend_pkg = os.path.join(source_root, "backend")
    if not os.path.isdir(backend_pkg):
        raise RuntimeError(f"backend package not found in source root: {source_root}")

    if source_root not in sys.path:
        sys.path.insert(0, source_root)

    try:
        import backend.main as main_module
    except Exception as exc:
        raise RuntimeError(
            f"Failed to import backend.main: {exc}"
        ) from exc

    app = getattr(main_module, "app", None)
    if app is None:
        raise RuntimeError("backend.main does not expose an 'app' object")
    return app


def _validate_spec(spec):
    """Defense-in-depth sanity check for the OpenAPI dict."""
    if not isinstance(spec, dict):
        raise RuntimeError(f"app.openapi() returned {type(spec).__name__}, expected dict")
    paths = spec.get("paths")
    if not isinstance(paths, dict):
        raise RuntimeError(f"'paths' is missing or not a dict (got {type(paths).__name__ if paths is not None else 'None'})")
    schemas = spec.get("components", {}).get("schemas")
    if not isinstance(schemas, dict):
        raise RuntimeError(f"'components.schemas' is missing or not a dict (got {type(schemas).__name__ if schemas is not None else 'None'})")


def _extract_openapi(app):
    """Call app.openapi() safely and capture any warnings."""
    captured = []
    try:
        with warnings.catch_warnings(record=True) as w:
            warnings.simplefilter("always")
            spec = app.openapi()
            for warning in w:
                msg = str(warning.message)
                msg = _normalize_message(msg)
                captured.append({
                    "category": warning.category.__name__ if warning.category else "Warning",
                    "message": msg,
                })
    except Exception as exc:
        raise RuntimeError(f"app.openapi() failed: {exc}") from exc
    return spec, captured


def _collect_summary(spec: dict, openapi_warnings: list = None) -> dict:
    """Build a summary from the raw OpenAPI dict."""
    paths = spec.get("paths", {})
    components = spec.get("components", {})
    schemas = components.get("schemas", {})

    total_paths = len(paths)
    total_ops = 0
    method_counts = {}
    status_counts = {}
    request_body_routes = []
    routes_without_response_schema = []
    op_ids = {}
    duplicate_op_ids = []
    route_details = []

    for path, path_item in paths.items():
        for method, op in path_item.items():
            if method in ("parameters",):
                continue
            total_ops += 1
            method_counts[method] = method_counts.get(method, 0) + 1

            op_id = op.get("operationId", "")
            if op_id:
                op_ids.setdefault(op_id, []).append(f"{method.upper()} {path}")

            tags = op.get("tags", [])

            # Request body schema refs
            req_schema_refs = []
            request_body = op.get("requestBody")
            if request_body:
                content = request_body.get("content", {})
                for media_type, media in content.items():
                    schema = media.get("schema")
                    if schema:
                        ref = schema.get("$ref")
                        if ref:
                            req_schema_refs.append(ref)
                        else:
                            req_schema_refs.append(f"inline:{media_type}")
                request_body_routes.append(f"{method.upper()} {path}")

            # Response schema refs
            resp_schema_refs = []
            responses = op.get("responses", {})
            for status, resp in responses.items():
                status_counts[status] = status_counts.get(status, 0) + 1
                content = resp.get("content", {})
                for media_type, media in content.items():
                    schema = media.get("schema")
                    if schema:
                        ref = schema.get("$ref")
                        if ref:
                            resp_schema_refs.append(ref)
                        else:
                            resp_schema_refs.append(f"inline:{media_type}")
                if not content:
                    routes_without_response_schema.append(f"{method.upper()} {path}")

            route_details.append({
                "method": method.upper(),
                "path": path,
                "operation_id": op_id,
                "tags": tags,
                "request_schema_refs": req_schema_refs,
                "has_request_body": bool(req_schema_refs),
                "response_schema_refs": resp_schema_refs,
                "has_response_schema": bool(resp_schema_refs),
            })

    for op_id, routes in op_ids.items():
        if len(routes) > 1:
            duplicate_op_ids.append({
                "operation_id": op_id,
                "routes": routes,
            })

    if openapi_warnings is None:
        openapi_warnings = []
    else:
        openapi_warnings = [
            {
                "category": w.get("category", "Warning"),
                "message": _normalize_message(w.get("message", "")),
            }
            for w in openapi_warnings
        ]

    return {
        "path_count": total_paths,
        "operation_count": total_ops,
        "schema_component_count": len(schemas),
        "method_counts": method_counts,
        "status_counts": status_counts,
        "request_body_count": len(request_body_routes),
        "routes_with_request_body": request_body_routes,
        "routes_without_response_schema": routes_without_response_schema,
        "duplicate_operation_ids": duplicate_op_ids,
        "routes": route_details,
        "openapi_warnings": openapi_warnings,
    }


def _collect_schema_refs(summary: dict) -> dict:
    """Map each schema component name to the routes that reference it."""
    schema_to_routes = {}
    for route in summary.get("routes", []):
        for ref in route.get("request_schema_refs", []) + route.get("response_schema_refs", []):
            if ref.startswith("#/components/schemas/"):
                name = ref.split("/")[-1]
                entry = schema_to_routes.setdefault(name, {"used_in": []})
                route_key = f"{route['method']} {route['path']}"
                if route_key not in entry["used_in"]:
                    entry["used_in"].append(route_key)
    return schema_to_routes


def _analyze_blockers(spec: dict) -> dict:
    """Analyze schemas for Go struct mapping blockers."""
    schemas = spec.get("components", {}).get("schemas", {})
    blockers = {
        "anyOf": [],
        "oneOf": [],
        "allOf": [],
        "additionalProperties": [],
        "arrays_without_items": [],
        "nullable_without_type": [],
        "object_without_properties": [],
    }

    def _scan(name: str, node: dict, path: str = ""):
        if not isinstance(node, dict):
            return
        current = f"{path}.{name}" if path else name
        if "anyOf" in node:
            blockers["anyOf"].append(current)
        if "oneOf" in node:
            blockers["oneOf"].append(current)
        if "allOf" in node:
            blockers["allOf"].append(current)
        if node.get("additionalProperties") is not False:
            # additionalProperties: true or a schema counts as a blocker
            if "additionalProperties" in node:
                blockers["additionalProperties"].append(current)
        if node.get("type") == "array" and "items" not in node:
            blockers["arrays_without_items"].append(current)
        if node.get("nullable") and "type" not in node:
            blockers["nullable_without_type"].append(current)
        if node.get("type") == "object" and "properties" not in node and "additionalProperties" not in node:
            blockers["object_without_properties"].append(current)
        for key, child in node.items():
            if isinstance(child, dict):
                _scan(key, child, current)
            elif isinstance(child, list):
                for idx, item in enumerate(child):
                    if isinstance(item, dict):
                        _scan(f"{key}[{idx}]", item, current)

    for name, schema in schemas.items():
        _scan(name, schema)
    return blockers


def _build_schema_dump(spec: dict, summary: dict) -> dict:
    """Build a full machine-readable schema freeze artifact."""
    schema_to_routes = _collect_schema_refs(summary)
    blockers = _analyze_blockers(spec)
    return {
        "openapi_version": spec.get("openapi", ""),
        "info": {
            "title": spec.get("info", {}).get("title", ""),
            "version": spec.get("info", {}).get("version", ""),
        },
        "schema_component_count": summary["schema_component_count"],
        "schemas": spec.get("components", {}).get("schemas", {}),
        "schema_usage": schema_to_routes,
        "route_schema_refs": [
            {
                "method": r["method"],
                "path": r["path"],
                "operation_id": r["operation_id"],
                "request_schema_refs": r["request_schema_refs"],
                "response_schema_refs": r["response_schema_refs"],
            }
            for r in summary.get("routes", [])
        ],
        "openapi_warnings": summary.get("openapi_warnings", []),
        "go_struct_mapping_blockers": {
            "counts": {k: len(v) for k, v in blockers.items()},
            "details": blockers,
        },
    }


def _render_markdown(summary: dict) -> str:
    lines = []
    lines.append("# OpenAPI Contract Summary")
    lines.append("")
    lines.append(f"- **Paths**: {summary['path_count']}")
    lines.append(f"- **Operations**: {summary['operation_count']}")
    lines.append(f"- **Schema Components**: {summary['schema_component_count']}")
    lines.append("")

    lines.append("## Method Counts")
    for method, count in sorted(summary["method_counts"].items()):
        lines.append(f"- {method.upper()}: {count}")
    lines.append("")

    lines.append("## Response Status Counts")
    for status, count in sorted(summary["status_counts"].items(), key=lambda x: (not x[0].isdigit(), int(x[0]) if x[0].isdigit() else x[0])):
        lines.append(f"- {status}: {count}")
    lines.append("")

    lines.append(f"## Request Body Routes ({summary['request_body_count']})")
    if summary["routes_with_request_body"]:
        for route in summary["routes_with_request_body"]:
            lines.append(f"- {route}")
    else:
        lines.append("_None_")
    lines.append("")

    dupes = summary["duplicate_operation_ids"]
    lines.append(f"## Duplicate Operation IDs ({len(dupes)})")
    if dupes:
        for d in dupes:
            lines.append(f"- `{d['operation_id']}`: {', '.join(d['routes'])}")
    else:
        lines.append("_None_")
    lines.append("")

    warns = summary.get("openapi_warnings", [])
    lines.append(f"## OpenAPI Warnings ({len(warns)})")
    if warns:
        for w in warns:
            cat = w.get("category", "Warning")
            msg = w.get("message", "")
            lines.append(f"- **{cat}**: {msg}")
    else:
        lines.append("_None_")
    lines.append("")

    lines.append("## Routes Without Response Schema")
    if summary["routes_without_response_schema"]:
        for route in summary["routes_without_response_schema"]:
            lines.append(f"- {route}")
    else:
        lines.append("_None_")
    lines.append("")

    lines.append("## Route Details")
    lines.append("")
    lines.append("| Method | Path | Operation ID | Tags | Req Schema | Resp Schema |")
    lines.append("|--------|------|--------------|------|------------|-------------|")
    for r in summary["routes"]:
        req = ", ".join(r["request_schema_refs"]) if r["request_schema_refs"] else "-"
        resp = ", ".join(r["response_schema_refs"]) if r["response_schema_refs"] else "-"
        tags = ", ".join(r["tags"]) if r["tags"] else "-"
        lines.append(f"| {r['method']} | `{r['path']}` | {r['operation_id'] or '-'} | {tags} | {req} | {resp} |")

    return "\n".join(lines)


def _render_json(summary: dict) -> str:
    return json.dumps(summary, indent=2, ensure_ascii=False)


def _render_inventory(schema_dump: dict) -> str:
    """Render a markdown inventory from a schema dump."""
    lines = []
    lines.append("# OpenAPI Schema Inventory")
    lines.append("")
    lines.append("> Status: R0 evidence. Schema extraction is preparatory, not a cutover completion.")
    lines.append("> This document lists all schema components and their usage, plus Go struct mapping blockers.")
    lines.append("> Go struct mapping itself is NOT done yet.")
    lines.append("")

    info = schema_dump.get("info", {})
    lines.append(f"- **OpenAPI Version**: {schema_dump.get('openapi_version', 'N/A')}")
    lines.append(f"- **API Title**: {info.get('title', 'N/A')}")
    lines.append(f"- **API Version**: {info.get('version', 'N/A')}")
    lines.append(f"- **Schema Component Count**: {schema_dump.get('schema_component_count', 0)}")
    lines.append("")

    # Schema list with usage
    lines.append("## Schema Components")
    lines.append("")
    schema_usage = schema_dump.get("schema_usage", {})
    for name in sorted(schema_usage.keys()):
        usage = schema_usage[name]
        routes = usage.get("used_in", [])
        lines.append(f"### {name}")
        if routes:
            for route in sorted(routes):
                lines.append(f"- {route}")
        else:
            lines.append("- _No route references detected_")
        lines.append("")

    # Blockers
    blockers = schema_dump.get("go_struct_mapping_blockers", {})
    counts = blockers.get("counts", {})
    details = blockers.get("details", {})
    lines.append("## Go Struct Mapping Blockers")
    lines.append("")
    lines.append("| Blocker Type | Count | Details |")
    lines.append("|--------------|-------|---------|")
    for key in ["anyOf", "oneOf", "allOf", "additionalProperties", "arrays_without_items", "nullable_without_type", "object_without_properties"]:
        count = counts.get(key, 0)
        detail_list = details.get(key, [])
        if detail_list:
            detail_str = ", ".join(f"`{d}`" for d in detail_list[:5])
            if len(detail_list) > 5:
                detail_str += f", ... ({len(detail_list) - 5} more)"
        else:
            detail_str = "_None_"
        lines.append(f"| {key} | {count} | {detail_str} |")
    lines.append("")

    # Route schema refs
    lines.append("## Route Schema Refs Summary")
    lines.append("")
    lines.append("| Method | Path | Operation ID | Request Refs | Response Refs |")
    lines.append("|--------|------|--------------|--------------|---------------|")
    for r in schema_dump.get("route_schema_refs", []):
        req = ", ".join(r["request_schema_refs"]) if r["request_schema_refs"] else "-"
        resp = ", ".join(r["response_schema_refs"]) if r["response_schema_refs"] else "-"
        lines.append(f"| {r['method']} | `{r['path']}` | {r['operation_id'] or '-'} | {req} | {resp} |")
    lines.append("")

    warnings = schema_dump.get("openapi_warnings", [])
    lines.append(f"## OpenAPI Warnings ({len(warnings)})")
    if warnings:
        for w in warnings:
            cat = w.get("category", "Warning")
            msg = w.get("message", "")
            lines.append(f"- **{cat}**: {msg}")
    else:
        lines.append("_None_")
    lines.append("")

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(
        description="Extract OpenAPI contract from Archive Center Beta 0.8(fix) backend."
    )
    parser.add_argument(
        "--source-root",
        default=_DEFAULT_SOURCE_ROOT,
        help="Path to the Archive Center Beta 0.8(fix) directory containing the backend package.",
    )
    parser.add_argument(
        "--format",
        choices=["json", "markdown"],
        default="json",
        help="Output format for the summary report.",
    )
    parser.add_argument(
        "--out",
        default=None,
        help="Optional output file path for the summary. If omitted, writes to stdout.",
    )
    parser.add_argument(
        "--schema-out",
        default=None,
        help="Optional output file path for a full schema dump JSON artifact. Writes only when specified.",
    )
    parser.add_argument(
        "--inventory-out",
        default=None,
        help="Optional output file path for a schema inventory markdown document. Writes only when specified.",
    )
    parser.add_argument(
        "--debug",
        action="store_true",
        help="Print full traceback on errors.",
    )
    args = parser.parse_args()

    _prepare_env()

    try:
        app = _import_app(args.source_root)
        spec, openapi_warnings = _extract_openapi(app)
        _validate_spec(spec)
        summary = _collect_summary(spec, openapi_warnings)
    except RuntimeError as exc:
        if args.debug:
            traceback.print_exc()
        else:
            sys.stderr.write(f"error: {exc}\n")
        sys.exit(1)

    # Traditional summary output (backward compatible)
    if args.format == "markdown":
        output = _render_markdown(summary)
    else:
        output = _render_json(summary)

    if args.out:
        Path(args.out).write_text(output, encoding="utf-8")
    else:
        sys.stdout.write(output)
        sys.stdout.write("\n")

    # Optional full schema dump
    if args.schema_out or args.inventory_out:
        schema_dump = _build_schema_dump(spec, summary)
        if args.schema_out:
            Path(args.schema_out).write_text(
                json.dumps(schema_dump, indent=2, ensure_ascii=False),
                encoding="utf-8",
            )
        if args.inventory_out:
            Path(args.inventory_out).write_text(
                _render_inventory(schema_dump),
                encoding="utf-8",
            )


if __name__ == "__main__":
    main()

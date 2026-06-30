import json
import sys
import argparse
import os
from typing import Any, Dict, List, Tuple

ABBREVS = {"id", "url", "http", "api", "json", "db", "sql", "uuid", "html", "xml", "cpu", "gpu", "ram", "ssd", "ip", "tcp", "udp"}

def to_go_field_name(name: str) -> str:
    parts = name.split("_")
    result = ""
    for part in parts:
        if not part:
            continue
        lower = part.lower()
        if lower in ABBREVS:
            result += lower.upper()
        else:
            result += part.capitalize()
    return result

def is_int64_field(field_path: str, field_name: str) -> bool:
    lower_path = (field_path + " " + field_name).lower()
    big_int_keywords = {"token", "count", "timeout", "budget", "limit", "offset", "size", "length", "timestamp", "duration"}
    small_index_exceptions = {"turn_index", "page_index", "array_index", "loop_index", "local_index"}
    if any(exc in lower_path for exc in small_index_exceptions):
        return False
    if any(kw in lower_path for kw in big_int_keywords):
        return True
    if field_name.lower().endswith("_id") or field_name.lower() == "id":
        return True
    return False

def map_property(prop: dict, schema_name: str, field_path: str, field_name: str = "") -> Tuple[str, List[str], str]:
    blockers: List[str] = []

    if "$ref" in prop:
        ref_name = prop["$ref"].split("/")[-1]
        if prop.get("nullable") is True:
            return f"*{ref_name}", blockers, "nullable_ref"
        return ref_name, blockers, "ref"

    if "oneOf" in prop:
        blockers.append(f"{field_path}: oneOf not supported (union type)")
        return "json.RawMessage", blockers, "oneof_json_rawmessage"
    if "allOf" in prop:
        blockers.append(f"{field_path}: allOf not supported (composition type)")
        return "json.RawMessage", blockers, "allof_json_rawmessage"

    if "anyOf" in prop:
        variants = prop["anyOf"]
        non_null = [v for v in variants if v.get("type") != "null"]
        has_null = any(v.get("type") == "null" for v in variants)

        if len(non_null) == 1 and has_null:
            inner_type, inner_blockers, inner_strategy = map_property(non_null[0], schema_name, f"{field_path}.anyOf[0]", field_name)
            blockers.extend(inner_blockers)
            if inner_type.startswith("*"):
                blockers.append(f"{field_path}: double-pointer guard triggered (inner type {inner_type} already pointer)")
                return "json.RawMessage", blockers, "double_pointer_guard"
            return f"*{inner_type}", blockers, "nullable_pointer"
        elif len(non_null) == 0:
            blockers.append(f"{field_path}: anyOf with only null")
            return "any", blockers, "nullable_anyof_only_null"
        else:
            blockers.append(f"{field_path}: non-null union anyOf")
            return "json.RawMessage", blockers, "union_json_rawmessage"

    prop_type = prop.get("type")
    if prop_type == "string":
        return "string", blockers, "direct"
    elif prop_type == "integer":
        go_int_type = "int64" if is_int64_field(field_path, field_name) else "int"
        return go_int_type, blockers, "direct"
    elif prop_type == "number":
        return "float64", blockers, "direct"
    elif prop_type == "boolean":
        return "bool", blockers, "direct"
    elif prop_type == "array":
        if "items" not in prop:
            blockers.append(f"{field_path}: array without items")
            return "[]any", blockers, "array_blocker"
        item_type, item_blockers, item_strategy = map_property(prop["items"], schema_name, f"{field_path}.items", field_name)
        blockers.extend(item_blockers)
        return f"[]{item_type}", blockers, "array_of"
    elif prop_type == "object":
        has_properties = "properties" in prop
        has_additional = "additionalProperties" in prop

        if has_properties and has_additional:
            add = prop["additionalProperties"]
            blockers.append(f"{field_path}: inline object with properties (needs named schema)")
            if add is True:
                blockers.append(f"{field_path}: inline object with properties + additionalProperties:true (fidelity loss: named fields and loose keys overlap)")
                return "map[string]any", blockers, "inline_object_with_additionalProperties"
            elif isinstance(add, dict):
                val_type, val_blockers, val_strategy = map_property(add, schema_name, f"{field_path}.additionalProperties", field_name)
                blockers.extend(val_blockers)
                blockers.append(f"{field_path}: inline object with properties + additionalProperties:{{schema}} (fidelity loss: named fields and typed loose keys overlap)")
                return f"map[string]{val_type}", blockers, "inline_object_with_typed_additionalProperties"

        if has_properties:
            blockers.append(f"{field_path}: inline object with properties (needs named schema)")
            return "map[string]any", blockers, "inline_object"
        elif has_additional:
            add = prop["additionalProperties"]
            if add is True:
                return "map[string]any", blockers, "map_string_any"
            elif isinstance(add, dict):
                val_type, val_blockers, val_strategy = map_property(add, schema_name, f"{field_path}.additionalProperties", field_name)
                blockers.extend(val_blockers)
                return f"map[string]{val_type}", blockers, "typed_map"
        else:
            blockers.append(f"{field_path}: object without properties")
            return "map[string]any", blockers, "object_without_properties"

    if prop_type is None and not any(k in prop for k in ("$ref", "anyOf", "oneOf", "allOf")):
        blockers.append(f"{field_path}: untyped property")
        return "any", blockers, "untyped"

    blockers.append(f"{field_path}: unknown type {prop_type}")
    return "any", blockers, "unknown"

def process_schema(name: str, schema: dict, routes_by_schema: dict) -> dict:
    properties = schema.get("properties", {})
    required = set(schema.get("required", []))

    fields = []
    required_count = 0
    blocker_count = 0

    for prop_name in sorted(properties.keys()):
        prop_def = properties[prop_name]
        is_required = prop_name in required
        if is_required:
            required_count += 1

        go_field_name = to_go_field_name(prop_name)
        go_type, blockers, strategy = map_property(prop_def, name, f"{name}.properties.{prop_name}", prop_name)

        nullable = False
        if "anyOf" in prop_def:
            variants = prop_def["anyOf"]
            has_null = any(v.get("type") == "null" for v in variants)
            non_null_count = len([v for v in variants if v.get("type") != "null"])
            if has_null and non_null_count == 1:
                nullable = True
        if prop_def.get("nullable") is True:
            nullable = True

        if blockers:
            blocker_count += len(blockers)

        if is_required:
            json_tag = prop_name
        else:
            json_tag = f"{prop_name},omitempty"

        has_default = "default" in prop_def
        default_value = prop_def.get("default")

        decode_presence_note = None
        if not is_required and not nullable and strategy == "direct" and go_type in ("string", "int", "int64", "float64", "bool"):
            decode_presence_note = (
                f"Optional non-null scalar {go_type}: absent vs zero-value distinction requires "
                "pointer type or custom decode logic when zero is semantically meaningful."
            )

        default_application_note = None
        if has_default and not is_required:
            default_application_note = (
                f"Optional field with default ({json.dumps(default_value)}): "
                "Go handler must apply default when field is absent in request."
            )

        fields.append({
            "name": prop_name,
            "go_field_name": go_field_name,
            "json_tag": json_tag,
            "required": is_required,
            "nullable": nullable,
            "proposed_go_type": go_type,
            "blockers": blockers,
            "mapping_strategy": strategy,
            "has_default": has_default,
            "default_value": default_value,
            "decode_presence_note": decode_presence_note,
            "default_application_note": default_application_note,
        })

    return {
        "schema_name": name,
        "go_type_name": name,
        "field_count": len(properties),
        "required_count": required_count,
        "blocker_count": blocker_count,
        "fields": fields,
        "routes_using": routes_by_schema.get(name, []),
        "openapi_required": sorted(list(required)),
    }

def build_routes_by_schema(contract_summary_path: str) -> dict:
    with open(contract_summary_path, "r", encoding="utf-8") as f:
        summary = json.load(f)

    routes_by_schema = {}
    for route in summary.get("routes", []):
        method = route.get("method", "")
        path = route.get("path", "")
        route_str = f"{method} {path}"
        for ref in route.get("request_schema_refs", []):
            schema_name = ref.split("/")[-1]
            routes_by_schema.setdefault(schema_name, []).append({"role": "request", "route": route_str})
        for ref in route.get("response_schema_refs", []):
            schema_name = ref.split("/")[-1]
            routes_by_schema.setdefault(schema_name, []).append({"role": "response", "route": route_str})

    return routes_by_schema

def generate_plan(freeze_path: str, contract_summary_path: str) -> dict:
    with open(freeze_path, "r", encoding="utf-8") as f:
        freeze = json.load(f)

    routes_by_schema = build_routes_by_schema(contract_summary_path)
    schemas = freeze.get("schemas", {})

    plan = {
        "openapi_version": freeze.get("openapi_version"),
        "schema_component_count": len(schemas),
        "mapping_policy": {
            "string": "string",
            "integer_small": "int (local indices: turn_index, page_index, etc.)",
            "integer_large": "int64 (token/count/timeout/budget/limit/offset/id/size/length/timestamp/duration)",
            "number": "float64",
            "boolean": "bool",
            "nullable_anyOf_T_null": "pointer *T",
            "nullable_ref": "pointer *RefName",
            "non_null_union_anyOf": "json.RawMessage (custom union policy needed)",
            "oneOf": "json.RawMessage (blocker: oneOf not supported)",
            "allOf": "json.RawMessage (blocker: allOf not supported)",
            "additionalProperties_true": "map[string]any",
            "additionalProperties_typed": "map[string]T where T is mapped from schema",
            "object_without_properties": "map[string]any (blocker: loose object)",
            "array_with_items": "[]T where T is mapped from items",
            "array_without_items": "blocker",
            "ref": "referenced DTO type",
            "inline_object": "map[string]any (blocker: needs named schema)",
            "inline_object_with_additionalProperties": "map[string]any (blocker: fidelity loss)",
            "untyped": "any (blocker: untyped property)",
        },
        "schemas": [],
    }

    for name in sorted(schemas.keys()):
        schema = schemas[name]
        plan["schemas"].append(process_schema(name, schema, routes_by_schema))

    return plan

def generate_markdown(plan: dict) -> str:
    lines = []
    lines.append("# Go DTO Mapping Plan")
    lines.append("")
    lines.append(f"> Generated from frozen OpenAPI schema ({plan['schema_component_count']} schemas)")
    lines.append("")
    lines.append("## Mapping Policy")
    lines.append("")
    lines.append("| OpenAPI Construct | Proposed Go Type | Strategy |")
    lines.append("|-------------------|------------------|----------|")
    for k, v in plan["mapping_policy"].items():
        lines.append(f"| `{k}` | {v} | - |")
    lines.append("")
    lines.append("## Per-Schema Summary")
    lines.append("")
    lines.append("| Schema | Fields | Required | Blockers | Routes |")
    lines.append("|--------|--------|----------|----------|--------|")
    for s in plan["schemas"]:
        route_count = len(s["routes_using"])
        lines.append(f"| {s['go_type_name']} | {s['field_count']} | {s['required_count']} | {s['blocker_count']} | {route_count} |")
    lines.append("")

    for s in plan["schemas"]:
        lines.append(f"## {s['go_type_name']}")
        lines.append("")
        lines.append(f"- **Fields**: {s['field_count']}")
        lines.append(f"- **Required**: {s['required_count']}")
        lines.append(f"- **Blockers**: {s['blocker_count']}")
        if s["routes_using"]:
            lines.append("- **Routes**:")
            for r in s["routes_using"]:
                lines.append(f"  - {r['role'].upper()}: `{r['route']}`")
        else:
            lines.append("- **Routes**: none")
        lines.append("")
        lines.append("| JSON Tag | Go Field | Required | Nullable | Has Default | Default Value | Go Type | Strategy | Blockers | Decode Note | Default Note |")
        lines.append("|----------|----------|----------|----------|-------------|---------------|---------|----------|----------|-------------|--------------|")
        for f in s["fields"]:
            blockers = "; ".join(f["blockers"]) if f["blockers"] else "-"
            decode_note = f.get("decode_presence_note") or "-"
            if decode_note != "-":
                decode_note = decode_note[:70] + "..." if len(decode_note) > 70 else decode_note
            default_note = f.get("default_application_note") or "-"
            if default_note != "-":
                default_note = default_note[:70] + "..." if len(default_note) > 70 else default_note
            has_default = "Yes" if f.get("has_default") else "No"
            default_value = json.dumps(f.get("default_value")) if f.get("has_default") else "-"
            lines.append(f"| `{f['json_tag']}` | {f['go_field_name']} | {'Yes' if f['required'] else 'No'} | {'Yes' if f['nullable'] else 'No'} | {has_default} | {default_value} | `{f['proposed_go_type']}` | {f['mapping_strategy']} | {blockers} | {decode_note} | {default_note} |")
        lines.append("")

    return "\n".join(lines)

def main():
    parser = argparse.ArgumentParser(description="Plan Go DTO mapping from frozen OpenAPI schema")
    parser.add_argument("--freeze", default="contracts/openapi-schema-freeze.json", help="Path to frozen schema JSON")
    parser.add_argument("--contract-summary", default="contracts/openapi-contract-summary.json", help="Path to contract summary JSON")
    parser.add_argument("--json-out", help="Output JSON plan to file")
    parser.add_argument("--markdown-out", help="Output Markdown plan to file")
    args = parser.parse_args()

    plan = generate_plan(args.freeze, args.contract_summary)

    if args.json_out:
        with open(args.json_out, "w", encoding="utf-8") as f:
            json.dump(plan, f, indent=2, ensure_ascii=False)
        print(f"Wrote JSON plan to {args.json_out}")

    if args.markdown_out:
        md = generate_markdown(plan)
        with open(args.markdown_out, "w", encoding="utf-8") as f:
            f.write(md)
        print(f"Wrote Markdown plan to {args.markdown_out}")

    if not args.json_out and not args.markdown_out:
        print(f"Schemas: {plan['schema_component_count']}")
        total_fields = sum(s["field_count"] for s in plan["schemas"])
        total_blockers = sum(s["blocker_count"] for s in plan["schemas"])
        print(f"Total fields: {total_fields}")
        print(f"Total blockers: {total_blockers}")
        for s in plan["schemas"]:
            print(f"  {s['go_type_name']}: {s['field_count']} fields, {s['required_count']} required, {s['blocker_count']} blockers")

if __name__ == "__main__":
    main()

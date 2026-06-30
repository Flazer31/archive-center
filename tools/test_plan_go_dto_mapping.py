import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "tools"))

from plan_go_dto_mapping import (
    to_go_field_name,
    map_property,
    process_schema,
    generate_plan,
    build_routes_by_schema,
)

class TestGoFieldNaming(unittest.TestCase):
    def test_basic(self):
        self.assertEqual(to_go_field_name("chat_session_id"), "ChatSessionID")
        self.assertEqual(to_go_field_name("name"), "Name")
        self.assertEqual(to_go_field_name("url"), "URL")
        self.assertEqual(to_go_field_name("api_key"), "APIKey")
        self.assertEqual(to_go_field_name("db_connection"), "DBConnection")
        self.assertEqual(to_go_field_name("json_data"), "JSONData")
        self.assertEqual(to_go_field_name("uuid"), "UUID")

class TestMapProperty(unittest.TestCase):
    def test_string(self):
        t, b, s = map_property({"type": "string"}, "Test", "Test.props.x")
        self.assertEqual(t, "string")
        self.assertEqual(b, [])
        self.assertEqual(s, "direct")

    def test_integer(self):
        t, b, s = map_property({"type": "integer"}, "Test", "Test.props.x")
        self.assertEqual(t, "int")

    def test_number(self):
        t, b, s = map_property({"type": "number"}, "Test", "Test.props.x")
        self.assertEqual(t, "float64")

    def test_boolean(self):
        t, b, s = map_property({"type": "boolean"}, "Test", "Test.props.x")
        self.assertEqual(t, "bool")

    def test_nullable_anyof_string_null(self):
        prop = {"anyOf": [{"type": "string"}, {"type": "null"}]}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "*string")
        self.assertEqual(b, [])
        self.assertEqual(s, "nullable_pointer")

    def test_nullable_anyof_object_null(self):
        prop = {"anyOf": [{"type": "object", "additionalProperties": True}, {"type": "null"}]}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "*map[string]any")
        self.assertEqual(b, [])
        self.assertEqual(s, "nullable_pointer")

    def test_non_null_union_anyof(self):
        prop = {"anyOf": [{"type": "string"}, {"type": "integer"}]}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "json.RawMessage")
        self.assertEqual(len(b), 1)
        self.assertIn("non-null union anyOf", b[0])
        self.assertEqual(s, "union_json_rawmessage")

    def test_anyof_only_null(self):
        prop = {"anyOf": [{"type": "null"}]}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "any")
        self.assertEqual(len(b), 1)
        self.assertIn("anyOf with only null", b[0])
        self.assertEqual(s, "nullable_anyof_only_null")

    def test_nullable_ref(self):
        prop = {"$ref": "#/components/schemas/SomeSchema", "nullable": True}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "*SomeSchema")
        self.assertEqual(b, [])
        self.assertEqual(s, "nullable_ref")

    def test_inline_object_with_properties(self):
        prop = {"type": "object", "properties": {"name": {"type": "string"}}}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "map[string]any")
        self.assertEqual(len(b), 1)
        self.assertIn("inline object with properties", b[0])
        self.assertEqual(s, "inline_object")

    def test_inline_object_with_properties_and_additionalProperties(self):
        prop = {
            "type": "object",
            "properties": {"name": {"type": "string"}},
            "additionalProperties": {"type": "integer"}
        }
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "map[string]int")
        self.assertEqual(len(b), 2)
        self.assertTrue(any("fidelity loss" in bi for bi in b))
        self.assertTrue(any("inline object with properties" in bi for bi in b))
        self.assertEqual(s, "inline_object_with_typed_additionalProperties")

    def test_double_pointer_guard(self):
        prop = {
            "anyOf": [
                {"anyOf": [{"type": "string"}, {"type": "null"}]},
                {"type": "null"}
            ]
        }
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "json.RawMessage")
        self.assertTrue(any("double-pointer" in bi for bi in b))
        self.assertEqual(s, "double_pointer_guard")

    def test_oneOf(self):
        prop = {"oneOf": [{"type": "string"}, {"type": "integer"}]}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "json.RawMessage")
        self.assertEqual(len(b), 1)
        self.assertIn("oneOf not supported", b[0])
        self.assertEqual(s, "oneof_json_rawmessage")

    def test_allOf(self):
        prop = {"allOf": [{"type": "object", "properties": {"a": {"type": "string"}}}]}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "json.RawMessage")
        self.assertEqual(len(b), 1)
        self.assertIn("allOf not supported", b[0])
        self.assertEqual(s, "allof_json_rawmessage")

    def test_additional_properties_true(self):
        prop = {"type": "object", "additionalProperties": True}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "map[string]any")
        self.assertEqual(b, [])
        self.assertEqual(s, "map_string_any")

    def test_additional_properties_typed(self):
        prop = {"type": "object", "additionalProperties": {"type": "string"}}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "map[string]string")
        self.assertEqual(b, [])
        self.assertEqual(s, "typed_map")

    def test_object_without_properties(self):
        prop = {"type": "object"}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "map[string]any")
        self.assertEqual(len(b), 1)
        self.assertIn("object without properties", b[0])
        self.assertEqual(s, "object_without_properties")

    def test_array_with_items(self):
        prop = {"type": "array", "items": {"type": "string"}}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "[]string")
        self.assertEqual(b, [])
        self.assertEqual(s, "array_of")

    def test_array_without_items(self):
        prop = {"type": "array"}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "[]any")
        self.assertEqual(len(b), 1)
        self.assertIn("array without items", b[0])
        self.assertEqual(s, "array_blocker")

    def test_ref(self):
        prop = {"$ref": "#/components/schemas/SomeSchema"}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "SomeSchema")
        self.assertEqual(b, [])
        self.assertEqual(s, "ref")

    def test_untyped(self):
        prop = {"title": "Input"}
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "any")
        self.assertEqual(len(b), 1)
        self.assertIn("untyped property", b[0])
        self.assertEqual(s, "untyped")

    def test_array_items_with_non_null_union(self):
        prop = {
            "type": "array",
            "items": {"anyOf": [{"type": "string"}, {"type": "integer"}]}
        }
        t, b, s = map_property(prop, "Test", "Test.props.x")
        self.assertEqual(t, "[]json.RawMessage")
        self.assertEqual(s, "array_of")
        self.assertEqual(len(b), 1)

    def test_integer_width_policy_int64(self):
        for name in ["token_count", "max_tokens", "timeout_ms", "budget_limit", "page_offset", "user_id", "file_size", "content_length", "created_timestamp", "request_duration"]:
            t, b, s = map_property({"type": "integer"}, "Test", f"Test.props.{name}", name)
            self.assertEqual(t, "int64", f"expected int64 for {name}")

    def test_integer_width_policy_int_small_index(self):
        for name in ["turn_index", "page_index", "array_index", "loop_index", "local_index"]:
            t, b, s = map_property({"type": "integer"}, "Test", f"Test.props.{name}", name)
            self.assertEqual(t, "int", f"expected int for {name}")

class TestProcessSchema(unittest.TestCase):
    def test_required_and_optional(self):
        schema = {
            "type": "object",
            "properties": {
                "id": {"type": "integer"},
                "count": {"type": "integer"},
            },
            "required": ["id"],
        }
        result = process_schema("Test", schema, {})
        fields_by_name = {f["name"]: f for f in result["fields"]}
        self.assertTrue(fields_by_name["id"]["required"])
        self.assertFalse(fields_by_name["count"]["required"])
        self.assertFalse(fields_by_name["id"]["has_default"])
        self.assertEqual(fields_by_name["id"]["json_tag"], "id")
        self.assertEqual(fields_by_name["count"]["json_tag"], "count,omitempty")
        self.assertEqual(fields_by_name["id"]["go_field_name"], "ID")
        self.assertEqual(fields_by_name["count"]["go_field_name"], "Count")

class TestInputErrors(unittest.TestCase):
    def test_missing_freeze_file(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            freeze_path = os.path.join(tmpdir, "nonexistent.json")
            contract_path = os.path.join(tmpdir, "contract.json")
            with open(contract_path, "w", encoding="utf-8") as f:
                json.dump({"routes": []}, f)
            with self.assertRaises(FileNotFoundError):
                generate_plan(freeze_path, contract_path)

    def test_missing_contract_file(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            freeze_path = os.path.join(tmpdir, "freeze.json")
            contract_path = os.path.join(tmpdir, "nonexistent.json")
            with open(freeze_path, "w", encoding="utf-8") as f:
                json.dump({"schemas": {}}, f)
            with self.assertRaises(FileNotFoundError):
                generate_plan(freeze_path, contract_path)

    def test_malformed_freeze_json(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            freeze_path = os.path.join(tmpdir, "freeze.json")
            contract_path = os.path.join(tmpdir, "contract.json")
            with open(freeze_path, "w", encoding="utf-8") as f:
                f.write("not json {")
            with open(contract_path, "w", encoding="utf-8") as f:
                json.dump({"routes": []}, f)
            with self.assertRaises(json.JSONDecodeError):
                generate_plan(freeze_path, contract_path)

class TestRoutesBySchema(unittest.TestCase):
    def test_routes_by_schema(self):
        summary = {
            "routes": [
                {
                    "method": "POST",
                    "path": "/test",
                    "request_schema_refs": ["#/components/schemas/TestSchema"],
                    "response_schema_refs": ["#/components/schemas/OtherSchema"],
                }
            ]
        }
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False, encoding="utf-8") as f:
            json.dump(summary, f)
            tmp_path = f.name
        try:
            routes = build_routes_by_schema(tmp_path)
            self.assertIn("TestSchema", routes)
            self.assertIn("OtherSchema", routes)
            self.assertEqual(routes["TestSchema"][0]["role"], "request")
            self.assertEqual(routes["OtherSchema"][0]["role"], "response")
        finally:
            os.unlink(tmp_path)

class TestGeneratePlanIntegration(unittest.TestCase):
    def test_synthetic_schemas(self):
        freeze = {
            "openapi_version": "3.1.0",
            "schemas": {
                "NullableString": {
                    "type": "object",
                    "properties": {
                        "value": {"anyOf": [{"type": "string"}, {"type": "null"}]}
                    },
                    "title": "NullableString",
                },
                "UnionValue": {
                    "type": "object",
                    "properties": {
                        "value": {"anyOf": [{"type": "string"}, {"type": "integer"}]}
                    },
                    "title": "UnionValue",
                },
                "LooseObject": {
                    "type": "object",
                    "properties": {
                        "meta": {"type": "object"}
                    },
                    "title": "LooseObject",
                },
                "TypedMap": {
                    "type": "object",
                    "properties": {
                        "counts": {"type": "object", "additionalProperties": {"type": "integer"}}
                    },
                    "title": "TypedMap",
                },
                "StringList": {
                    "type": "object",
                    "properties": {
                        "items": {"type": "array", "items": {"type": "string"}}
                    },
                    "title": "StringList",
                },
                "RefHolder": {
                    "type": "object",
                    "properties": {
                        "other": {"$ref": "#/components/schemas/StringList"}
                    },
                    "title": "RefHolder",
                },
            }
        }
        contract = {"routes": []}
        with tempfile.TemporaryDirectory() as tmpdir:
            freeze_path = os.path.join(tmpdir, "freeze.json")
            contract_path = os.path.join(tmpdir, "contract.json")
            with open(freeze_path, "w", encoding="utf-8") as f:
                json.dump(freeze, f)
            with open(contract_path, "w", encoding="utf-8") as f:
                json.dump(contract, f)

            plan = generate_plan(freeze_path, contract_path)
            self.assertEqual(plan["schema_component_count"], 6)

            schemas_by_name = {s["schema_name"]: s for s in plan["schemas"]}

            ns = schemas_by_name["NullableString"]
            self.assertEqual(ns["fields"][0]["proposed_go_type"], "*string")
            self.assertEqual(ns["fields"][0]["nullable"], True)
            self.assertEqual(ns["blocker_count"], 0)

            uv = schemas_by_name["UnionValue"]
            self.assertEqual(uv["fields"][0]["proposed_go_type"], "json.RawMessage")
            self.assertEqual(uv["blocker_count"], 1)

            lo = schemas_by_name["LooseObject"]
            self.assertEqual(lo["fields"][0]["proposed_go_type"], "map[string]any")
            self.assertEqual(lo["blocker_count"], 1)

            tm = schemas_by_name["TypedMap"]
            self.assertEqual(tm["fields"][0]["proposed_go_type"], "map[string]int64")
            self.assertEqual(tm["blocker_count"], 0)

            sl = schemas_by_name["StringList"]
            self.assertEqual(sl["fields"][0]["proposed_go_type"], "[]string")
            self.assertEqual(sl["blocker_count"], 0)

            rh = schemas_by_name["RefHolder"]
            self.assertEqual(rh["fields"][0]["proposed_go_type"], "StringList")
            self.assertEqual(rh["blocker_count"], 0)

    def test_default_and_omitempty_and_decode_notes(self):
        freeze = {
            "openapi_version": "3.1.0",
            "schemas": {
                "Config": {
                    "type": "object",
                    "properties": {
                        "required_id": {"type": "integer"},
                        "optional_count": {"type": "integer", "default": 10},
                        "optional_name": {"type": "string"},
                    },
                    "required": ["required_id"],
                }
            }
        }
        contract = {"routes": []}
        with tempfile.TemporaryDirectory() as tmpdir:
            freeze_path = os.path.join(tmpdir, "freeze.json")
            contract_path = os.path.join(tmpdir, "contract.json")
            with open(freeze_path, "w", encoding="utf-8") as f:
                json.dump(freeze, f)
            with open(contract_path, "w", encoding="utf-8") as f:
                json.dump(contract, f)

            plan = generate_plan(freeze_path, contract_path)
            schema = plan["schemas"][0]
            fields_by_name = {f["name"]: f for f in schema["fields"]}

            self.assertEqual(fields_by_name["required_id"]["json_tag"], "required_id")
            self.assertTrue(fields_by_name["required_id"]["required"])
            self.assertFalse(fields_by_name["required_id"]["has_default"])
            self.assertIsNone(fields_by_name["required_id"].get("decode_presence_note"))
            self.assertIsNone(fields_by_name["required_id"].get("default_application_note"))

            self.assertEqual(fields_by_name["optional_count"]["json_tag"], "optional_count,omitempty")
            self.assertFalse(fields_by_name["optional_count"]["required"])
            self.assertTrue(fields_by_name["optional_count"]["has_default"])
            self.assertEqual(fields_by_name["optional_count"]["default_value"], 10)
            self.assertIsNotNone(fields_by_name["optional_count"].get("default_application_note"))

            self.assertEqual(fields_by_name["optional_name"]["json_tag"], "optional_name,omitempty")
            self.assertFalse(fields_by_name["optional_name"]["required"])
            self.assertFalse(fields_by_name["optional_name"]["has_default"])
            self.assertIsNotNone(fields_by_name["optional_name"].get("decode_presence_note"))

    def test_nullable_ref_and_inline_object_and_fidelity_loss(self):
        freeze = {
            "openapi_version": "3.1.0",
            "schemas": {
                "WithNullableRef": {
                    "type": "object",
                    "properties": {
                        "child": {"$ref": "#/components/schemas/Child", "nullable": True}
                    },
                },
                "WithInlineProps": {
                    "type": "object",
                    "properties": {
                        "extra": {"type": "object", "properties": {"key": {"type": "string"}}}
                    },
                },
                "WithFidelityLoss": {
                    "type": "object",
                    "properties": {
                        "mixed": {
                            "type": "object",
                            "properties": {"name": {"type": "string"}},
                            "additionalProperties": {"type": "integer"}
                        }
                    },
                },
            }
        }
        contract = {"routes": []}
        with tempfile.TemporaryDirectory() as tmpdir:
            freeze_path = os.path.join(tmpdir, "freeze.json")
            contract_path = os.path.join(tmpdir, "contract.json")
            with open(freeze_path, "w", encoding="utf-8") as f:
                json.dump(freeze, f)
            with open(contract_path, "w", encoding="utf-8") as f:
                json.dump(contract, f)

            plan = generate_plan(freeze_path, contract_path)
            schemas_by_name = {s["schema_name"]: s for s in plan["schemas"]}

            nr = schemas_by_name["WithNullableRef"]
            self.assertEqual(nr["fields"][0]["proposed_go_type"], "*Child")
            self.assertTrue(nr["fields"][0]["nullable"])

            io = schemas_by_name["WithInlineProps"]
            self.assertEqual(io["fields"][0]["proposed_go_type"], "map[string]any")
            self.assertTrue(any("inline object with properties" in b for b in io["fields"][0]["blockers"]))

            fl = schemas_by_name["WithFidelityLoss"]
            self.assertEqual(fl["fields"][0]["proposed_go_type"], "map[string]int")
            self.assertTrue(any("fidelity loss" in b for b in fl["fields"][0]["blockers"]))
            self.assertTrue(any("inline object with properties" in b for b in fl["fields"][0]["blockers"]))

    def test_double_pointer_and_oneof_allof(self):
        freeze = {
            "openapi_version": "3.1.0",
            "schemas": {
                "DoublePtr": {
                    "type": "object",
                    "properties": {
                        "bad": {
                            "anyOf": [
                                {"anyOf": [{"type": "string"}, {"type": "null"}]},
                                {"type": "null"}
                            ]
                        }
                    },
                },
                "OneOfField": {
                    "type": "object",
                    "properties": {
                        "value": {"oneOf": [{"type": "string"}, {"type": "integer"}]}
                    },
                },
                "AllOfField": {
                    "type": "object",
                    "properties": {
                        "value": {"allOf": [{"type": "object", "properties": {"a": {"type": "string"}}}]}
                    },
                },
            }
        }
        contract = {"routes": []}
        with tempfile.TemporaryDirectory() as tmpdir:
            freeze_path = os.path.join(tmpdir, "freeze.json")
            contract_path = os.path.join(tmpdir, "contract.json")
            with open(freeze_path, "w", encoding="utf-8") as f:
                json.dump(freeze, f)
            with open(contract_path, "w", encoding="utf-8") as f:
                json.dump(contract, f)

            plan = generate_plan(freeze_path, contract_path)
            schemas_by_name = {s["schema_name"]: s for s in plan["schemas"]}

            dp = schemas_by_name["DoublePtr"]
            self.assertEqual(dp["fields"][0]["proposed_go_type"], "json.RawMessage")
            self.assertTrue(any("double-pointer" in b for b in dp["fields"][0]["blockers"]))

            of = schemas_by_name["OneOfField"]
            self.assertEqual(of["fields"][0]["proposed_go_type"], "json.RawMessage")
            self.assertTrue(any("oneOf" in b for b in of["fields"][0]["blockers"]))

            af = schemas_by_name["AllOfField"]
            self.assertEqual(af["fields"][0]["proposed_go_type"], "json.RawMessage")
            self.assertTrue(any("allOf" in b for b in af["fields"][0]["blockers"]))

if __name__ == "__main__":
    unittest.main(verbosity=2)

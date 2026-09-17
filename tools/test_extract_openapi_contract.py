#!/usr/bin/env python3
"""Unit tests for extract_openapi_contract.py using synthetic OpenAPI dicts.

These tests do NOT import the real 0.8 backend.
"""

import json
import os
import sys
import unittest

# Make the parent directory importable so we can import the tool module.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import extract_openapi_contract as extract


class TestCollectSummary(unittest.TestCase):
    def _make_spec(self):
        return {
            "openapi": "3.1.0",
            "info": {"title": "Test API", "version": "0.1.0"},
            "paths": {
                "/health": {
                    "get": {
                        "operationId": "health_check",
                        "tags": ["public"],
                        "responses": {
                            "200": {
                                "description": "OK",
                                "content": {
                                    "application/json": {
                                        "schema": {"$ref": "#/components/schemas/Health"}
                                    }
                                }
                            }
                        }
                    }
                },
                "/items": {
                    "get": {
                        "operationId": "list_items",
                        "tags": ["items"],
                        "responses": {
                            "200": {
                                "description": "OK",
                                "content": {
                                    "application/json": {
                                        "schema": {"$ref": "#/components/schemas/ItemList"}
                                    }
                                }
                            }
                        }
                    },
                    "post": {
                        "operationId": "create_item",
                        "tags": ["items"],
                        "requestBody": {
                            "content": {
                                "application/json": {
                                    "schema": {"$ref": "#/components/schemas/ItemCreate"}
                                }
                            }
                        },
                        "responses": {
                            "201": {
                                "description": "Created",
                                "content": {
                                    "application/json": {
                                        "schema": {"$ref": "#/components/schemas/Item"}
                                    }
                                }
                            }
                        }
                    }
                },
                "/items/{item_id}": {
                    "get": {
                        "operationId": "list_items",  # intentional duplicate
                        "tags": ["items"],
                        "responses": {
                            "200": {
                                "description": "OK",
                            }
                        }
                    },
                    "delete": {
                        "operationId": "delete_item",
                        "tags": ["items"],
                        "responses": {
                            "204": {
                                "description": "No Content"
                            }
                        }
                    }
                },
            },
            "components": {
                "schemas": {
                    "Health": {"type": "object"},
                    "ItemList": {"type": "array"},
                    "ItemCreate": {"type": "object"},
                    "Item": {"type": "object"},
                }
            },
        }

    def test_summary_counts(self):
        spec = self._make_spec()
        summary = extract._collect_summary(spec)
        self.assertEqual(summary["path_count"], 3)
        self.assertEqual(summary["operation_count"], 5)
        self.assertEqual(summary["schema_component_count"], 4)
        self.assertEqual(summary["method_counts"]["get"], 3)
        self.assertEqual(summary["method_counts"]["post"], 1)
        self.assertEqual(summary["method_counts"]["delete"], 1)
        self.assertEqual(summary["request_body_count"], 1)

    def test_duplicate_operation_ids(self):
        spec = self._make_spec()
        summary = extract._collect_summary(spec)
        dupes = summary["duplicate_operation_ids"]
        self.assertEqual(len(dupes), 1)
        self.assertEqual(dupes[0]["operation_id"], "list_items")
        self.assertEqual(len(dupes[0]["routes"]), 2)

    def test_routes_with_request_body(self):
        spec = self._make_spec()
        summary = extract._collect_summary(spec)
        self.assertEqual(summary["routes_with_request_body"], ["POST /items"])

    def test_routes_without_response_schema(self):
        spec = self._make_spec()
        summary = extract._collect_summary(spec)
        # DELETE /items/{item_id} has 204 with no content schema
        # GET /items/{item_id} has 200 with no content schema
        without = summary["routes_without_response_schema"]
        self.assertIn("DELETE /items/{item_id}", without)
        self.assertIn("GET /items/{item_id}", without)

    def test_status_counts(self):
        spec = self._make_spec()
        summary = extract._collect_summary(spec)
        self.assertEqual(summary["status_counts"]["200"], 3)
        self.assertEqual(summary["status_counts"]["201"], 1)
        self.assertEqual(summary["status_counts"]["204"], 1)

    def test_route_details(self):
        spec = self._make_spec()
        summary = extract._collect_summary(spec)
        routes = summary["routes"]
        self.assertEqual(len(routes), 5)
        health = [r for r in routes if r["path"] == "/health" and r["method"] == "GET"][0]
        self.assertEqual(health["operation_id"], "health_check")
        self.assertEqual(health["tags"], ["public"])
        self.assertTrue(health["has_response_schema"])
        self.assertFalse(health["has_request_body"])

    def test_empty_spec(self):
        summary = extract._collect_summary({"paths": {}, "components": {}})
        self.assertEqual(summary["path_count"], 0)
        self.assertEqual(summary["operation_count"], 0)
        self.assertEqual(summary["schema_component_count"], 0)


class TestRenderMarkdown(unittest.TestCase):
    def test_contains_counts(self):
        summary = extract._collect_summary({
            "paths": {
                "/health": {
                    "get": {
                        "operationId": "health",
                        "responses": {"200": {"description": "OK"}}
                    }
                }
            },
            "components": {"schemas": {"Health": {}}}
        })
        md = extract._render_markdown(summary)
        self.assertIn("Paths", md)
        self.assertIn("Operations", md)
        self.assertIn("Schema Components", md)
        self.assertIn("GET", md)
        self.assertIn("/health", md)
        self.assertIn("health", md)

    def test_duplicate_rendered(self):
        summary = extract._collect_summary({
            "paths": {
                "/a": {"get": {"operationId": "dup", "responses": {}}},
                "/b": {"get": {"operationId": "dup", "responses": {}}},
            },
            "components": {}
        })
        md = extract._render_markdown(summary)
        self.assertIn("Duplicate Operation IDs", md)
        self.assertIn("dup", md)


class TestRenderJson(unittest.TestCase):
    def test_json_shape(self):
        summary = extract._collect_summary({
            "paths": {
                "/health": {
                    "get": {
                        "operationId": "health",
                        "responses": {"200": {"description": "OK"}}
                    }
                }
            },
            "components": {}
        })
        js = extract._render_json(summary)
        parsed = json.loads(js)
        self.assertIn("path_count", parsed)
        self.assertIn("operation_count", parsed)
        self.assertIn("routes", parsed)
        self.assertIsInstance(parsed["routes"], list)




class TestWarningCapture(unittest.TestCase):
    def test_warnings_passed_to_summary(self):
        spec = {
            "paths": {
                "/health": {
                    "get": {
                        "operationId": "health",
                        "responses": {"200": {"description": "OK"}}
                    }
                }
            },
            "components": {}
        }
        warnings = [
            {"category": "UserWarning", "message": "Test warning A"},
            {"category": "RuntimeWarning", "message": "Test warning B"},
        ]
        summary = extract._collect_summary(spec, warnings)
        self.assertEqual(len(summary["openapi_warnings"]), 2)
        self.assertEqual(summary["openapi_warnings"][0]["message"], "Test warning A")
        self.assertEqual(summary["openapi_warnings"][1]["category"], "RuntimeWarning")

    def test_default_warnings_empty(self):
        spec = {"paths": {}, "components": {}}
        summary = extract._collect_summary(spec)
        self.assertEqual(summary["openapi_warnings"], [])

    def test_warnings_in_json_output(self):
        spec = {"paths": {}, "components": {}}
        warnings = [{"category": "UserWarning", "message": "Duplicate operation ID"}]
        summary = extract._collect_summary(spec, warnings)
        js = extract._render_json(summary)
        parsed = json.loads(js)
        self.assertIn("openapi_warnings", parsed)
        self.assertEqual(len(parsed["openapi_warnings"]), 1)
        self.assertEqual(parsed["openapi_warnings"][0]["message"], "Duplicate operation ID")


class TestRealNewlineMarkdown(unittest.TestCase):
    def test_markdown_uses_actual_newlines(self):
        summary = extract._collect_summary({
            "paths": {
                "/health": {
                    "get": {
                        "operationId": "health",
                        "responses": {"200": {"description": "OK"}}
                    }
                }
            },
            "components": {"schemas": {"Health": {}}}
        })
        md = extract._render_markdown(summary)
        # Must contain actual newline characters, not literal backslash-n.
        self.assertIn("\n", md, "Should contain actual newlines, not literal \\n")
        # This test verifies the previous literal-newline bug is fixed.
        # The string "\\n" should NOT appear in the markdown output.
        self.assertNotIn("\\n", md, "Must not contain literal backslash-n characters")

    def test_markdown_warnings_section(self):
        summary = extract._collect_summary(
            {"paths": {}, "components": {}},
            [{"category": "UserWarning", "message": "Duplicate Operation ID for get_items"}]
        )
        md = extract._render_markdown(summary)
        self.assertIn("OpenAPI Warnings", md)
        self.assertIn("Duplicate Operation ID for get_items", md)
        self.assertIn("UserWarning", md)

class TestValidateSpec(unittest.TestCase):
    def test_non_dict_spec_raises(self):
        with self.assertRaises(RuntimeError) as ctx:
            extract._validate_spec("not a dict")
        self.assertIn("app.openapi() returned str", str(ctx.exception))

    def test_missing_paths_raises(self):
        with self.assertRaises(RuntimeError) as ctx:
            extract._validate_spec({"info": {}})
        self.assertIn("'paths' is missing or not a dict", str(ctx.exception))

    def test_non_dict_paths_raises(self):
        with self.assertRaises(RuntimeError) as ctx:
            extract._validate_spec({"paths": []})
        self.assertIn("'paths' is missing or not a dict", str(ctx.exception))

    def test_missing_schemas_raises(self):
        with self.assertRaises(RuntimeError) as ctx:
            extract._validate_spec({"paths": {}})
        self.assertIn("'components.schemas' is missing or not a dict", str(ctx.exception))

    def test_non_dict_schemas_raises(self):
        with self.assertRaises(RuntimeError) as ctx:
            extract._validate_spec({"paths": {}, "components": {"schemas": []}})
        self.assertIn("'components.schemas' is missing or not a dict", str(ctx.exception))

    def test_valid_spec_passes(self):
        extract._validate_spec({"paths": {}, "components": {"schemas": {}}})


class TestWarningNormalization(unittest.TestCase):
    def test_multiline_warning_collapsed(self):
        raw = "Line one\nLine two\n  extra spaces  "
        normalized = extract._normalize_message(raw)
        self.assertEqual(normalized, "Line one Line two extra spaces")

    def test_single_line_unchanged(self):
        raw = "Already single line"
        normalized = extract._normalize_message(raw)
        self.assertEqual(normalized, "Already single line")

    def test_warnings_in_markdown_are_single_line(self):
        spec = {"paths": {}, "components": {}}
        warnings = [{"category": "UserWarning", "message": "Bad\nnews"}]
        summary = extract._collect_summary(spec, warnings)
        md = extract._render_markdown(summary)
        # The markdown line containing the warning must not have embedded newlines
        for line in md.splitlines():
            if "Bad" in line:
                self.assertNotIn("\n", line, "Rendered markdown line must not contain literal backslash-n")

    def test_warnings_in_json_are_single_line(self):
        spec = {"paths": {}, "components": {}}
        warnings = [{"category": "UserWarning", "message": "Bad\nnews"}]
        summary = extract._collect_summary(spec, warnings)
        js = extract._render_json(summary)
        parsed = json.loads(js)
        self.assertEqual(parsed["openapi_warnings"][0]["message"], "Bad news")


class TestStatusSortStability(unittest.TestCase):
    def test_mixed_int_str_status_sort(self):
        summary = extract._collect_summary({
            "paths": {
                "/a": {
                    "get": {
                        "responses": {
                            "200": {"description": "OK"},
                            "default": {"description": "Error"}
                        }
                    }
                },
                "/b": {
                    "post": {
                        "responses": {
                            "201": {"description": "Created"},
                            "default": {"description": "Error"}
                        }
                    }
                }
            },
            "components": {}
        })
        md = extract._render_markdown(summary)
        # Should not crash; verify markdown contains the statuses
        self.assertIn("200", md)
        self.assertIn("201", md)
        self.assertIn("default", md)

    def test_status_sort_order(self):
        summary = extract._collect_summary({
            "paths": {
                "/a": {
                    "get": {
                        "responses": {
                            "default": {"description": "Error"},
                            "500": {"description": "Server Error"},
                            "200": {"description": "OK"}
                        }
                    }
                }
            },
            "components": {}
        })
        md = extract._render_markdown(summary)
        # Numeric codes should appear before non-numeric 'default'
        idx_200 = md.index("200")
        idx_default = md.index("default")
        self.assertLess(idx_200, idx_default, "Numeric status codes should sort before non-numeric ones")




class TestSchemaRefs(unittest.TestCase):
    def test_collect_schema_refs_maps_routes(self):
        spec = {
            "paths": {
                "/items": {
                    "post": {
                        "operationId": "create_item",
                        "requestBody": {
                            "content": {
                                "application/json": {
                                    "schema": {"$ref": "#/components/schemas/ItemCreate"}
                                }
                            }
                        },
                        "responses": {
                            "201": {
                                "content": {
                                    "application/json": {
                                        "schema": {"$ref": "#/components/schemas/Item"}
                                    }
                                }
                            }
                        }
                    }
                },
                "/items/{id}": {
                    "get": {
                        "operationId": "get_item",
                        "responses": {
                            "200": {
                                "content": {
                                    "application/json": {
                                        "schema": {"$ref": "#/components/schemas/Item"}
                                    }
                                }
                            }
                        }
                    }
                }
            },
            "components": {
                "schemas": {
                    "ItemCreate": {"type": "object"},
                    "Item": {"type": "object"},
                }
            }
        }
        summary = extract._collect_summary(spec)
        refs = extract._collect_schema_refs(summary)
        self.assertIn("ItemCreate", refs)
        self.assertIn("Item", refs)
        self.assertEqual(len(refs["Item"]["used_in"]), 2)
        self.assertEqual(len(refs["ItemCreate"]["used_in"]), 1)

    def test_inline_schemas_ignored(self):
        spec = {
            "paths": {
                "/upload": {
                    "post": {
                        "requestBody": {
                            "content": {
                                "multipart/form-data": {
                                    "schema": {"type": "string", "format": "binary"}
                                }
                            }
                        },
                        "responses": {"200": {"description": "OK"}}
                    }
                }
            },
            "components": {"schemas": {}}
        }
        summary = extract._collect_summary(spec)
        refs = extract._collect_schema_refs(summary)
        self.assertEqual(len(refs), 0)


class TestBlockerDetection(unittest.TestCase):
    def test_anyOf_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "UnionType": {"anyOf": [{"type": "string"}, {"type": "integer"}]}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["anyOf"], ["UnionType"])
        self.assertEqual(blockers["oneOf"], [])
        self.assertEqual(blockers["allOf"], [])

    def test_oneOf_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "Discriminated": {"oneOf": [{"type": "object"}, {"type": "array"}]}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["oneOf"], ["Discriminated"])

    def test_allOf_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "Merged": {"allOf": [{"type": "object"}, {"type": "object"}]}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["allOf"], ["Merged"])

    def test_additionalProperties_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "LooseObject": {"type": "object", "additionalProperties": True}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["additionalProperties"], ["LooseObject"])

    def test_additionalProperties_false_not_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "StrictObject": {"type": "object", "additionalProperties": False}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["additionalProperties"], [])

    def test_array_without_items_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "UntypedArray": {"type": "array"}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["arrays_without_items"], ["UntypedArray"])

    def test_array_with_items_not_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "TypedArray": {"type": "array", "items": {"type": "string"}}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["arrays_without_items"], [])

    def test_nullable_without_type_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "MaybeValue": {"nullable": True}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["nullable_without_type"], ["MaybeValue"])

    def test_nullable_with_type_not_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "NullableString": {"type": "string", "nullable": True}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["nullable_without_type"], [])

    def test_object_without_properties_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "BareObject": {"type": "object"}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["object_without_properties"], ["BareObject"])

    def test_object_with_properties_not_detected(self):
        spec = {
            "components": {
                "schemas": {
                    "RichObject": {"type": "object", "properties": {"id": {"type": "integer"}}}
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertEqual(blockers["object_without_properties"], [])

    def test_nested_blocker_paths(self):
        spec = {
            "components": {
                "schemas": {
                    "Nested": {
                        "type": "object",
                        "properties": {
                            "child": {
                                "anyOf": [{"type": "string"}]
                            }
                        }
                    }
                }
            }
        }
        blockers = extract._analyze_blockers(spec)
        self.assertIn("Nested.properties.child", blockers["anyOf"])


class TestSchemaDumpShape(unittest.TestCase):
    def test_dump_contains_required_keys(self):
        spec = {
            "openapi": "3.1.0",
            "info": {"title": "T", "version": "1"},
            "paths": {},
            "components": {
                "schemas": {
                    "A": {"type": "object"}
                }
            }
        }
        summary = extract._collect_summary(spec)
        dump = extract._build_schema_dump(spec, summary)
        self.assertEqual(dump["openapi_version"], "3.1.0")
        self.assertEqual(dump["info"]["title"], "T")
        self.assertEqual(dump["schema_component_count"], 1)
        self.assertIn("schemas", dump)
        self.assertIn("schema_usage", dump)
        self.assertIn("route_schema_refs", dump)
        self.assertIn("openapi_warnings", dump)
        self.assertIn("go_struct_mapping_blockers", dump)
        blockers = dump["go_struct_mapping_blockers"]
        self.assertIn("counts", blockers)
        self.assertIn("details", blockers)

    def test_dump_counts_are_integers(self):
        spec = {
            "openapi": "3.1.0",
            "info": {"title": "T", "version": "1"},
            "paths": {},
            "components": {
                "schemas": {
                    "Loose": {"type": "object", "additionalProperties": True}
                }
            }
        }
        summary = extract._collect_summary(spec)
        dump = extract._build_schema_dump(spec, summary)
        counts = dump["go_struct_mapping_blockers"]["counts"]
        for k, v in counts.items():
            self.assertIsInstance(v, int, f"count {k} should be int")


class TestInventoryRendering(unittest.TestCase):
    def test_inventory_contains_schema_list(self):
        dump = {
            "openapi_version": "3.1.0",
            "info": {"title": "T", "version": "1"},
            "schema_component_count": 1,
            "schema_usage": {
                "Health": {"used_in": ["GET /health"]}
            },
            "route_schema_refs": [],
            "openapi_warnings": [],
            "go_struct_mapping_blockers": {
                "counts": {"anyOf": 0, "oneOf": 0, "allOf": 0, "additionalProperties": 0, "arrays_without_items": 0, "nullable_without_type": 0, "object_without_properties": 0},
                "details": {"anyOf": [], "oneOf": [], "allOf": [], "additionalProperties": [], "arrays_without_items": [], "nullable_without_type": [], "object_without_properties": []}
            }
        }
        md = extract._render_inventory(dump)
        self.assertIn("## Schema Components", md)
        self.assertIn("### Health", md)
        self.assertIn("GET /health", md)
        self.assertIn("## Go Struct Mapping Blockers", md)
        self.assertIn("## Route Schema Refs Summary", md)

    def test_inventory_warnings_rendered(self):
        dump = {
            "openapi_version": "3.1.0",
            "info": {"title": "T", "version": "1"},
            "schema_component_count": 0,
            "schema_usage": {},
            "route_schema_refs": [],
            "openapi_warnings": [{"category": "UserWarning", "message": "Test warning"}],
            "go_struct_mapping_blockers": {
                "counts": {},
                "details": {}
            }
        }
        md = extract._render_inventory(dump)
        self.assertIn("Test warning", md)
        self.assertIn("UserWarning", md)

    def test_inventory_blocker_truncation(self):
        dump = {
            "openapi_version": "3.1.0",
            "info": {"title": "T", "version": "1"},
            "schema_component_count": 0,
            "schema_usage": {},
            "route_schema_refs": [],
            "openapi_warnings": [],
            "go_struct_mapping_blockers": {
                "counts": {"anyOf": 7},
                "details": {"anyOf": [f"Schema{i}" for i in range(7)]}
            }
        }
        md = extract._render_inventory(dump)
        self.assertIn("... (2 more)", md)


if __name__ == "__main__":
    unittest.main()

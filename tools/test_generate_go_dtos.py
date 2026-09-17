import json
import os
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import generate_go_dtos as gen

class TestGenerateGoDtos(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.plan_path = Path(self.tmpdir.name) / "plan.json"
        self.out_path = Path(self.tmpdir.name) / "types_gen.go"

    def tearDown(self):
        self.tmpdir.cleanup()

    def _generate(self, plan):
        self.plan_path.write_text(json.dumps(plan), encoding="utf-8")
        gen.generate_go_dto(str(self.plan_path), str(self.out_path))
        return self.out_path.read_text(encoding="utf-8")

    def test_generates_structs_with_pointer_defaults(self):
        plan = {
            "schemas": [
                {
                    "schema_name": "TestRequest",
                    "fields": [
                        {
                            "name": "required_field",
                            "go_field_name": "RequiredField",
                            "json_tag": "required_field",
                            "required": True,
                            "nullable": False,
                            "proposed_go_type": "string",
                            "has_default": False,
                            "decode_presence_note": "",
                            "default_value": "",
                            "blockers": {},
                        },
                        {
                            "name": "optional_with_default",
                            "go_field_name": "OptionalWithDefault",
                            "json_tag": "optional_with_default,omitempty",
                            "required": False,
                            "nullable": False,
                            "proposed_go_type": "int",
                            "has_default": True,
                            "decode_presence_note": "Optional non-null scalar int",
                            "default_value": "42",
                            "blockers": {},
                        },
                        {
                            "name": "nullable_string",
                            "go_field_name": "NullableString",
                            "json_tag": "nullable_string,omitempty",
                            "required": False,
                            "nullable": True,
                            "proposed_go_type": "string",
                            "has_default": False,
                            "decode_presence_note": "",
                            "default_value": "",
                            "blockers": {},
                        },
                        {
                            "name": "raw_map",
                            "go_field_name": "RawMap",
                            "json_tag": "raw_map,omitempty",
                            "required": False,
                            "nullable": False,
                            "proposed_go_type": "map[string]any",
                            "has_default": False,
                            "decode_presence_note": "",
                            "default_value": "",
                            "blockers": {},
                        },
                    ],
                }
            ]
        }
        code = self._generate(plan)

        self.assertIn("type TestRequest struct", code)
        self.assertRegex(code, r"RequiredField\s+string.*`json:\"required_field\"`")
        self.assertRegex(code, r"OptionalWithDefault\s+\*int.*`json:\"optional_with_default,omitempty\"`")
        self.assertRegex(code, r"NullableString\s+\*string.*`json:\"nullable_string,omitempty\"`")
        self.assertRegex(code, r"RawMap\s+map\[string\]any.*`json:\"raw_map,omitempty\"`")

        self.assertIn("func (dto *TestRequest) ApplyDefaults()", code)
        self.assertIn("v := 42", code)
        self.assertIn("dto.OptionalWithDefault = &v", code)

    def test_gofmt_clean(self):
        plan = {
            "schemas": [
                {
                    "schema_name": "FmtRequest",
                    "fields": [
                        {
                            "name": "x",
                            "go_field_name": "X",
                            "json_tag": "x",
                            "required": True,
                            "nullable": False,
                            "proposed_go_type": "string",
                            "has_default": False,
                            "decode_presence_note": "",
                            "default_value": "",
                            "blockers": {},
                        }
                    ],
                }
            ]
        }
        self._generate(plan)

        gofmt = gen.find_gofmt()
        result = subprocess.run([gofmt, "-d", str(self.out_path)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, f"gofmt failed: {result.stderr}")
        self.assertEqual(result.stdout.strip(), "", f"gofmt diff not empty:\n{result.stdout}")

    def test_blocker_comment(self):
        plan = {
            "schemas": [
                {
                    "schema_name": "BlockerRequest",
                    "fields": [
                        {
                            "name": "field",
                            "go_field_name": "Field",
                            "json_tag": "field",
                            "required": True,
                            "nullable": False,
                            "proposed_go_type": "string",
                            "has_default": False,
                            "decode_presence_note": "",
                            "default_value": "",
                            "blockers": {"complex_type": "needs custom decoder"},
                        }
                    ],
                }
            ]
        }
        code = self._generate(plan)
        self.assertIn("// BLOCKER [complex_type]: needs custom decoder", code)

    def test_preserves_explicit_zero(self):
        plan = {
            "schemas": [
                {
                    "schema_name": "ZeroRequest",
                    "fields": [
                        {
                            "name": "count",
                            "go_field_name": "Count",
                            "json_tag": "count,omitempty",
                            "required": False,
                            "nullable": False,
                            "proposed_go_type": "int",
                            "has_default": True,
                            "decode_presence_note": "presence",
                            "default_value": "5",
                            "blockers": {},
                        }
                    ],
                }
            ]
        }
        code = self._generate(plan)
        self.assertIn("if dto.Count == nil {", code)
        self.assertIn("v := 5", code)

    def test_json_raw_message_import(self):
        plan = {
            "schemas": [
                {
                    "schema_name": "RawRequest",
                    "fields": [
                        {
                            "name": "payload",
                            "go_field_name": "Payload",
                            "json_tag": "payload",
                            "required": True,
                            "nullable": False,
                            "proposed_go_type": "json.RawMessage",
                            "has_default": False,
                            "decode_presence_note": "",
                            "default_value": "",
                            "blockers": {},
                        }
                    ],
                }
            ]
        }
        code = self._generate(plan)
        self.assertIn('"encoding/json"', code)
        self.assertIn("Payload json.RawMessage", code)


    def test_blocker_list_comment(self):
        plan = {
            "schemas": [
                {
                    "schema_name": "BlockerListRequest",
                    "fields": [
                        {
                            "name": "field",
                            "go_field_name": "Field",
                            "json_tag": "field",
                            "required": True,
                            "nullable": False,
                            "proposed_go_type": "string",
                            "has_default": False,
                            "decode_presence_note": "",
                            "default_value": "",
                            "blockers": ["needs custom decoder", "untyped property"],
                        }
                    ],
                }
            ]
        }
        code = self._generate(plan)
        self.assertIn("// BLOCKER: needs custom decoder", code)
        self.assertIn("// BLOCKER: untyped property", code)

    def test_schema_comment_wording(self):
        plan = {
            "schemas": [
                {
                    "schema_name": "WordingRequest",
                    "fields": [
                        {
                            "name": "x",
                            "go_field_name": "X",
                            "json_tag": "x",
                            "required": True,
                            "nullable": False,
                            "proposed_go_type": "string",
                            "has_default": False,
                            "decode_presence_note": "",
                            "default_value": "",
                            "blockers": {},
                        }
                    ],
                }
            ]
        }
        code = self._generate(plan)
        self.assertIn("// WordingRequest generated from OpenAPI schema.", code)
        self.assertNotIn("generated from JS schema", code)
if __name__ == "__main__":
    unittest.main()

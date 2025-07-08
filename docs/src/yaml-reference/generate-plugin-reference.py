import json
from typing import List, Tuple, Dict, Any

from pathlib import Path

# Get current script directory
BASE_DIR = Path(__file__).resolve().parent

# Paths
SCHEMA_PATH = BASE_DIR.parent.parent.parent / "internal" / "dsl" / "schema.json"
OUTPUT_MD_PATH = BASE_DIR / "plugin-reference.md"

def heading(title: str, level: int = 2) -> str:
    return f"\n{'#' * level} {title}\n\n"


def render_table(headers: List[str], rows: List[List[str]]) -> str:
    md = "| " + " | ".join(headers) + " |\n"
    md += "| " + " | ".join(["-" * len(h) for h in headers]) + " |\n"
    for row in rows:
        md += "| " + " | ".join(row) + " |\n"
    return md + "\n"


def extract_fields(
    properties: dict,
    required_fields: list,
    parent_key: str = ""
) -> List[Tuple[str, str, str, str, str, str]]:
    rows = []

    for key, val in properties.items():
        full_key = f"{parent_key}.{key}" if parent_key else key
        field_type = val.get("type", "any")
        desc = val.get("description", "No description")
        is_required = "✅" if key in required_fields else ""
        enum_vals = val.get("enum", [])
        enum_str = ", ".join(f"`{v}`" for v in enum_vals) if enum_vals else "-"
        notes = "-"
        display_key = f"{full_key}"

        if field_type == "object" and "properties" in val:
            rows.append((display_key, key, is_required, desc, "`object`", notes))
            rows += extract_fields(val["properties"], val.get("required", []), full_key)
        elif field_type == "array":
            item_type = val.get("items", {}).get("type")
            display_key = f"{full_key}[]"
            if item_type == "object" and "properties" in val["items"]:
                rows.append((display_key, key, is_required, desc, "`array of objects`", notes))
                rows += extract_fields(val["items"]["properties"], val["items"].get("required", []), full_key + "[]")
            else:
                item_type_str = f"`array of {item_type or 'any'}`"
                rows.append((display_key, key, is_required, desc, item_type_str, notes))
        else:
            type_display = enum_str if enum_vals else f"`{field_type}`"
            rows.append((display_key, key, is_required, desc, type_display, notes))

    return rows


def generate_step_base_template(step_props: Dict[str, Any]) -> str:
    rows = []
    for key, val in step_props.items():
        desc = val.get("description", "No description")
        required = "✅" if key in ["name", "plugin", "config"] else ""
        rows.append([f"`{key}`", required, desc])
    return heading("Test Step Structure") + render_table(["Field", "Required", "Description"], rows)


def generate_plugin_list(steps_schema: dict) -> str:
    md = heading("Supported Plugins")
    plugin_enum = steps_schema["properties"]["plugin"].get("enum", [])
    plugin_descriptions = {
        "http": "Make HTTP requests and validate responses",
        "delay": "Pause execution for a given duration",
        "script": "Run JavaScript or shell scripts",
        "sql": "Run SQL queries and validate results",
        "log": "Emit logs for visibility or debugging",
        "agent": "Run steps on remote agents",
        "browser": "Automate browser actions using Playwright",
        "supabase": "Interact with Supabase APIs or database"
    }
    for plugin in plugin_enum:
        desc = plugin_descriptions.get(plugin, "No description")
        md += f"- `{plugin}`: {desc}\n"
    return md + "\n"


def generate_plugin_configs_table(all_of_blocks: List[dict]) -> str:
    md = heading("Plugin Configurations")

    for block in all_of_blocks:
        plugin = block.get("if", {}).get("properties", {}).get("plugin", {}).get("const")
        if not plugin:
            continue

        md += heading(f"Plugin: `{plugin}`", level=3)

        properties = block.get("then", {}).get("properties", {})
        config = properties.get("config", {})
        save = properties.get("save", {})
        assertions = properties.get("assertions", {})
        props = config.get("properties", {})
        required = config.get("required", [])
        one_of_required = set()

        for clause in config.get("oneOf", []):
            one_of_required.update(clause.get("required", []))

        table = []
        for display_field, raw_key, req, desc, type_str, notes in extract_fields(props, required):
            if raw_key in one_of_required:
                req += " (oneOf)"
            table.append([f"`{display_field}`", req, desc, type_str, notes])

        md += render_table(["Field", "Required", "Description", "Type / Allowed Values", "Notes"], table)

        if assertions:
            md += generate_assertion_fields_table(assertions, plugin, level=5)
        if save:
            md += generate_save_fields_table(save, plugin, level=5)

    return md


def generate_assertion_fields_table(schema, plugin=None, level=2) -> str:
    heading_text = f"`{plugin}` Assertions" if plugin else "Assertions"
    md = heading(heading_text, level)

    props = schema["items"]["properties"]
    required = schema["items"].get("required", [])
    conditional_required = {}

    for cond in schema["items"].get("allOf", []):
        types = cond.get("if", {}).get("properties", {}).get("type", {}).get("enum", [])
        for t in types:
            conditional_required[t] = cond.get("then", {}).get("required", [])

    table = []
    for key, val in props.items():
        desc = val.get("description", "No description")
        enums = val.get("enum", [])
        enum_str = ", ".join(f"`{v}`" for v in enums) if enums else "-"
        is_required = "✅" if key in required else ""
        for t, fields in conditional_required.items():
            if key in fields:
                is_required += f" (if `type` is `{t}`)"
        table.append([f"`{key}`", is_required, desc, enum_str])

    return md + render_table(["Field", "Required", "Description", "Allowed Values"], table)


def generate_save_fields_table(schema, plugin=None, level=2) -> str:
    heading_text = f"`{plugin}` Save Fields" if plugin else "Save Fields"
    md = heading(heading_text, level)

    props = schema["items"]["properties"]
    required = schema["items"].get("required", [])
    one_of_required = set()
    for cond in schema["items"].get("oneOf", []):
        one_of_required.update(cond.get("required", []))

    table = []
    for display_field, raw_key, req, desc, _, _ in extract_fields(props, required):
        if raw_key in one_of_required:
            req += " (oneOf)"
        table.append([f"`{display_field}`", req, desc, "-"])

    return md + render_table(["Field", "Required", "Description", "Notes"], table)


def generate_full_markdown(schema: dict) -> str:
    try:
        steps_schema = schema["properties"]["tests"]["items"]["properties"]["steps"]["items"]
        base_md = generate_step_base_template(steps_schema["properties"])
        plugin_list_md = generate_plugin_list(steps_schema)
        plugin_config_md = generate_plugin_configs_table(steps_schema.get("allOf", []))
        assertions_md = generate_assertion_fields_table(steps_schema["properties"]["assertions"])
        save_md = generate_save_fields_table(steps_schema["properties"]["save"])

        return (
            base_md +
            "\n---\n" +
            plugin_list_md +
            "\n---\n" +
            plugin_config_md +
            "\n---\n" +
            assertions_md +
            "\n---\n" +
            save_md
        )
    except KeyError as e:
        return f"❌ Error: Schema structure is unexpected or incomplete — missing key: {e}"


if __name__ == "__main__":
    try:
        with open(SCHEMA_PATH, "r") as f:
            schema = json.load(f)
    except FileNotFoundError:
        print("❌ Error: schema.json not found.")
        exit(1)
    except json.JSONDecodeError as e:
        print(f"❌ Error: Invalid JSON - {e}")
        exit(1)

    markdown_doc = generate_full_markdown(schema)

    with open(OUTPUT_MD_PATH, "w") as out:
        out.write(markdown_doc)

    print(f"✅ Markdown documentation generated: {OUTPUT_MD_PATH}")

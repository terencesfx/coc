#!/usr/bin/env python3
"""Extract all skill descriptions used by the workbook's 技能注释 sheet."""

import json
import re
import sys
import xml.etree.ElementTree as ET
from pathlib import Path
from zipfile import ZipFile

NS = {"m": "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}
COLUMNS = {
    "B": "name", "C": "base", "D": "applicability", "E": "difficulty",
    "F": "description", "G": "regularExample", "H": "hardExample",
    "I": "pushExamples", "J": "pushFailure", "K": "insaneFailure",
}


def extract(source: Path) -> list[dict[str, str]]:
    with ZipFile(source) as archive:
        shared_root = ET.fromstring(archive.read("xl/sharedStrings.xml"))
        shared = ["".join(t.text or "" for t in item.findall(".//m:t", NS)) for item in shared_root.findall("m:si", NS)]
        sheet = ET.fromstring(archive.read("xl/worksheets/sheet5.xml"))
    result = []
    for row in sheet.findall(".//m:sheetData/m:row", NS):
        number = int(row.attrib["r"])
        if number < 95 or number > 211:
            continue
        values: dict[str, str] = {}
        for cell in row.findall("m:c", NS):
            column = re.match(r"[A-Z]+", cell.attrib["r"]).group(0)
            if column not in COLUMNS:
                continue
            node = cell.find("m:v", NS)
            if node is None:
                continue
            value = node.text or ""
            if cell.attrib.get("t") == "s":
                value = shared[int(value)]
            values[COLUMNS[column]] = value.strip()
        name = values.get("name", "").rstrip("：:").strip()
        if not name or "——" in name:
            continue
        values["name"] = name
        result.append({key: values.get(key, "") for key in COLUMNS.values()})
    return result


def main() -> None:
    source = Path(sys.argv[1] if len(sys.argv) > 1 else "COC7版自动卡.xlsx")
    target = Path(sys.argv[2] if len(sys.argv) > 2 else "web/src/data/coc7-skill-descriptions.json")
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(json.dumps(extract(source), ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"wrote {target}")


if __name__ == "__main__":
    main()

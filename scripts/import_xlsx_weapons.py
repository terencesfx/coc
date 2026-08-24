#!/usr/bin/env python3
"""Extract the weapon catalogue from COC7版自动卡.xlsx without modifying it."""

import json
import re
import sys
import xml.etree.ElementTree as ET
from pathlib import Path
from zipfile import ZipFile

NS = {"m": "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}
COLUMNS = {
    "B": "name",
    "C": "skill",
    "D": "damage",
    "E": "range",
    "F": "penetration",
    "G": "attacksText",
    "H": "ammoText",
    "I": "malfunctionText",
    "J": "era",
    "K": "price",
    "L": "invention",
    "M": "category",
    "N": "notes",
}


def cell_column(reference: str) -> str:
    return re.match(r"[A-Z]+", reference).group(0)


def extract(source: Path) -> list[dict[str, str]]:
    with ZipFile(source) as archive:
        shared_root = ET.fromstring(archive.read("xl/sharedStrings.xml"))
        shared = [
            "".join(node.text or "" for node in item.findall(".//m:t", NS))
            for item in shared_root.findall("m:si", NS)
        ]
        sheet = ET.fromstring(archive.read("xl/worksheets/sheet9.xml"))

    weapons = []
    for row in sheet.findall(".//m:sheetData/m:row", NS):
        number = int(row.attrib["r"])
        if number < 2 or number > 105:
            continue
        values: dict[str, str] = {}
        for cell in row.findall("m:c", NS):
            column = cell_column(cell.attrib["r"])
            if column not in COLUMNS:
                continue
            value_node = cell.find("m:v", NS)
            if value_node is None:
                continue
            value = value_node.text or ""
            if cell.attrib.get("t") == "s":
                value = shared[int(value)]
            values[COLUMNS[column]] = value.strip()
        if values.get("name"):
            weapons.append({key: values.get(key, "") for key in COLUMNS.values()})
    return weapons


def main() -> None:
    source = Path(sys.argv[1] if len(sys.argv) > 1 else "COC7版自动卡.xlsx")
    target = Path(
        sys.argv[2] if len(sys.argv) > 2 else "web/src/data/coc7-weapons.json"
    )
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(
        json.dumps(extract(source), ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(f"wrote {target}")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Import occupations from the COC7 automatic-sheet workbook using stdlib only."""

import argparse
import json
import re
import zipfile
from pathlib import Path
from xml.etree import ElementTree as ET


NS = "{http://schemas.openxmlformats.org/spreadsheetml/2006/main}"
MARKERS = ("☆", "⊙", "☯", "※")
ATTR_LABELS = {
    "STR": "力量",
    "CON": "体质",
    "SIZ": "体型",
    "DEX": "敏捷",
    "APP": "外貌",
    "INT": "智力",
    "POW": "意志",
    "EDU": "教育",
}


def column_number(reference: str) -> int:
    result = 0
    for char in re.match(r"[A-Z]+", reference).group(0):
        result = result * 26 + ord(char) - 64
    return result


def worksheet(zip_file, name: str, shared: list[str]) -> dict[int, dict[int, dict[str, str]]]:
    root = ET.fromstring(zip_file.read(name))
    result = {}
    for row in root.findall(f".//{NS}row"):
        values = {}
        for cell in row.findall(f"{NS}c"):
            value_node = cell.find(f"{NS}v")
            formula_node = cell.find(f"{NS}f")
            value = "" if value_node is None else (shared[int(value_node.text)] if cell.get("t") == "s" else value_node.text or "")
            values[column_number(cell.get("r"))] = {"value": value, "formula": "" if formula_node is None else formula_node.text or ""}
        result[int(row.get("r"))] = values
    return result


def cell(rows, row, column, field="value"):
    return rows.get(row, {}).get(column, {}).get(field, "")


def formulas(expression: str) -> list[dict]:
    alternatives = [expression]
    match = re.search(r"MAX\(([^)]+)\)", expression)
    if match:
        alternatives = [expression[: match.start()] + choice + expression[match.end() :] for choice in match.group(1).split(",")]
    result = []
    for alternative in alternatives:
        terms = []
        for attribute, multiplier in re.findall(r"(STR|CON|SIZ|DEX|APP|INT|POW|EDU)\*(\d+)", alternative):
            terms.append({"attribute": attribute.lower(), "multiplier": int(multiplier)})
        label = " + ".join(f"{ATTR_LABELS[t['attribute'].upper()]}×{t['multiplier']}" for t in terms)
        result.append({"label": label, "terms": terms})
    return result


def normalized_skill(row: int, raw_name: str, cell_value: str) -> str | None:
    del row, cell_value
    name = raw_name.replace(" Ω", "").rstrip("：").strip()
    if name in {"自定义技能", "0"}:
        return None
    return "侦察" if name == "侦查" else name


def eras_for(name: str) -> list[str]:
    if "现代" in name:
        return ["modern"]
    if "古典" in name or "原作向" in name:
        return ["1920s"]
    return ["1920s", "modern"]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("workbook", type=Path)
    parser.add_argument("output", type=Path)
    args = parser.parse_args()

    with zipfile.ZipFile(args.workbook) as archive:
        strings_root = ET.fromstring(archive.read("xl/sharedStrings.xml"))
        shared = ["".join(node.text or "" for node in item.iter(f"{NS}t")) for item in strings_root.findall(f"{NS}si")]
        occupation_rows = worksheet(archive, "xl/worksheets/sheet6.xml", shared)
        skill_rows = worksheet(archive, "xl/worksheets/sheet3.xml", shared)

    occupations = []
    for row_number in range(4, 233):
        sequence = int(float(cell(occupation_rows, row_number, 1)))
        name = cell(occupation_rows, row_number, 2).strip()
        skill_column = sequence + 2
        fixed = []
        grouped = {marker: [] for marker in MARKERS}
        fallback_choices = []

        for skill_row in range(8, 75):
            raw_skill = cell(skill_rows, skill_row, 1).strip()
            marker = cell(skill_rows, skill_row, skill_column).strip()
            if not marker:
                continue
            skill = normalized_skill(skill_row, raw_skill, marker)
            marker_group = next((item for item in MARKERS if item in marker), None)
            if skill:
                if marker == "★":
                    fixed.append(skill)
                elif marker_group:
                    grouped[marker_group].append(skill)
                else:
                    fixed.append(skill)
            elif marker == "★":
                fallback_choices.append(raw_skill.rstrip("：①②③ Ω"))

        choice_groups = []
        for index, marker in enumerate(MARKERS, start=3):
            count_text = cell(skill_rows, index, skill_column).strip()
            if not count_text:
                continue
            count = int(float(count_text))
            skills = sorted(set(grouped[marker]))
            if skills:
                choice_groups.append({"count": min(count, len(skills)), "skills": skills})
            else:
                choice_groups.append({"count": count, "category": "任意符合职业说明的技能"})
        for category in sorted(set(fallback_choices)):
            choice_groups.append({"count": 1, "category": category})

        free_text = cell(skill_rows, 7, skill_column).strip()
        summary = " ".join(cell(occupation_rows, row_number, 7).split())
        credit_text = cell(occupation_rows, row_number, 4).strip()
        credit_min, credit_max = (int(part) for part in credit_text.split("-", 1))
        occupations.append(
            {
                "id": f"custom.xlsx.{sequence:03d}",
                "name": name,
                "eras": eras_for(name),
                "creditRating": {
                    "min": credit_min,
                    "max": credit_max,
                },
                # A few cells in this workbook omit their cached shared-formula XML;
                # their displayed value is EDU×4, so use that as the safe fallback.
                "skillPointFormulas": formulas(cell(occupation_rows, row_number, 6, "formula") or "EDU*4"),
                "fixedSkills": sorted(set(fixed)),
                "choiceGroups": choice_groups,
                "freeChoiceCount": int(float(free_text)) if free_text else 0,
                "description": summary,
            }
        )

    preserved = []
    if args.output.exists():
        current = json.loads(args.output.read_text(encoding="utf-8"))
        preserved = [item for item in current.get("occupations", []) if not item.get("id", "").startswith("custom.xlsx.")]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps({"schemaVersion": 1, "occupations": preserved + occupations}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    print(f"Imported {len(occupations)} occupations into {args.output}; preserved {len(preserved)} manual occupations")


if __name__ == "__main__":
    main()

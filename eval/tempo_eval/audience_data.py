from __future__ import annotations

import hashlib
import json
import re
import sqlite3
import xml.etree.ElementTree as ET
from dataclasses import dataclass
from datetime import date, datetime, timedelta
from pathlib import Path
from typing import Any
from zipfile import ZipFile

from .models import AudienceQuery

_NS = {"m": "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}
_EVENT_SHEET = "События"
_CLIENT_KEY = "client_id"
_TIMESTAMP = "event_timestamp"
_EVENT_DATE = "event_date"
_FORBIDDEN_FIELDS = {
    "client_id",
    "event_id",
    "gender",
    "age_years",
    "estimated_monthly_income_rub",
    "region_at_event",
    "previous_region",
    "city_at_event",
    "previous_city",
    "region_since",
    "campaign_id",
    "campaign_action",
    "campaign_touches_prior_90d",
    "balance_change_rub",
    "balance_after_event_rub",
}
_QUERYABLE_FIELDS = {
    "source_system",
    "event_type",
    "account_scope",
    "transaction_category",
    "expense_category",
    "mcc_code",
    "mcc_role",
    "account_types",
    "bank_products",
    "app_section",
}
_PROFILE_EXCLUDED_EVENT_TYPES = {"показ_кампании", "отклик_на_кампанию"}
_REVIEW_MAX_ROWS = 100


@dataclass(frozen=True)
class SegmentCalculation:
    dataset_ref: str
    rule_ref: str
    membership_ref: str
    query: dict[str, Any]
    count: int
    base_count: int
    share: float
    population: str
    feature_summaries: list[dict[str, Any]]
    aggregate_summary: dict[str, Any]
    quality: dict[str, Any]


class AudienceDataset:
    """Private SQLite-backed event dataset and client-level segment calculator."""

    def __init__(self, path: Path, db_path: Path | None = None) -> None:
        if path.suffix.lower() != ".xlsx":
            raise ValueError("audience data must be an .xlsx workbook")
        self.path = path
        self.db_path = db_path or path.with_suffix(".sqlite")
        self.dataset_ref = f"xlsx:{_file_digest(path)}"
        self._ensure_database()

    def profile(self) -> dict[str, Any]:
        with self._connect() as connection:
            columns = _table_columns(connection)
            queryable = sorted(_QUERYABLE_FIELDS.intersection(columns))
            values = {
                field: [
                    {"value": value, "event_count": count}
                    for value, count in connection.execute(
                        f"SELECT {_quote(field)}, COUNT(*) FROM events "
                        f"WHERE {_quote(field)} IS NOT NULL AND {_quote(field)} != '' "
                        f"GROUP BY {_quote(field)} ORDER BY COUNT(*) DESC LIMIT 30"
                    ).fetchall()
                ]
                for field in queryable
            }
            event_count, unique_clients, start, end = connection.execute(
                "SELECT COUNT(*), COUNT(DISTINCT client_id), MIN(event_date), MAX(event_date) FROM events"
            ).fetchone()
        return {
            "dataset_ref": self.dataset_ref,
            "source": self.path.name,
            "storage": "private analytics database",
            "event_sheet": _EVENT_SHEET,
            "unit": "client",
            "event_count": event_count,
            "unique_client_count": unique_clients,
            "date_range": {"start": start, "end": end},
            "queryable_fields": queryable,
            "forbidden_fields": sorted(_FORBIDDEN_FIELDS.intersection(columns)),
            "value_catalog": values,
            "privacy_boundary": (
                "Raw event rows and client_id remain in the private SQLite database and are never sent to the model."
            ),
        }

    def calculate(self, query: AudienceQuery) -> SegmentCalculation:
        start, end = _parse_window(query.window_start, query.window_end)
        errors = self.validate_query(query, start, end)
        if errors:
            raise ValueError("; ".join(errors))
        months = _months_in_window(start, end)
        query_data = query.model_dump(mode="json")
        rule_ref = f"rule:{_digest_json(query_data)}"
        with self._connect() as connection:
            base_count = connection.execute(
                "SELECT COUNT(DISTINCT client_id) FROM events WHERE event_date >= ? AND event_date < ?",
                (start.isoformat(), end.isoformat()),
            ).fetchone()[0]
            having_sql: list[str] = ["COUNT(DISTINCT substr(event_date, 1, 7)) = ?"]
            having_params: list[Any] = [len(months)]
            for condition in query.conditions:
                condition_sql, condition_params = _condition_sql(condition)
                having_sql.append(f"COUNT(DISTINCT CASE WHEN {condition_sql} THEN substr(event_date, 1, 7) END) = ?")
                having_params.extend(condition_params)
                having_params.append(len(months))
            qualifying_sql = (
                "SELECT client_id FROM events WHERE event_date >= ? AND event_date < ? GROUP BY client_id HAVING "
                + " AND ".join(having_sql)
            )
            qualifying_params = [start.isoformat(), end.isoformat(), *having_params]
            qualifying = connection.execute(
                f"SELECT COUNT(*) FROM ({qualifying_sql})",
                qualifying_params,
            ).fetchone()[0]
            feature_summaries = []
            for condition in query.conditions:
                condition_sql, condition_params = _condition_sql(condition)
                monthly = dict(
                    connection.execute(
                        "SELECT substr(event_date, 1, 7), COUNT(DISTINCT client_id) FROM events "
                        f"WHERE event_date >= ? AND event_date < ? AND {condition_sql} GROUP BY 1",
                        [start.isoformat(), end.isoformat(), *condition_params],
                    ).fetchall()
                )
                feature_summaries.append(
                    {
                        "name": condition.name,
                        "source_columns": sorted({filter_.field for filter_ in condition.filters}),
                        "filters": [filter_.model_dump(mode="json") for filter_ in condition.filters],
                        "aggregation": "At least one matching event for every calendar month in the selected window.",
                        "monthly_client_counts": [
                            {"month": month.isoformat(), "client_count": monthly.get(month.strftime("%Y-%m"), 0)}
                            for month in months
                        ],
                    }
                )
            required_fields = {filter_.field for condition in query.conditions for filter_ in condition.filters}
            missing = {
                field: connection.execute(
                    f"SELECT COUNT(*) FROM events WHERE {_quote(field)} IS NULL OR {_quote(field)} = ''"
                ).fetchone()[0]
                for field in sorted(required_fields)
            }
            aggregate_summary = _segment_aggregate_summary(
                connection,
                qualifying_sql,
                qualifying_params,
                start,
                end,
                months,
                qualifying,
            )
        return SegmentCalculation(
            dataset_ref=self.dataset_ref,
            rule_ref=rule_ref,
            membership_ref=f"private-membership:{rule_ref.removeprefix('rule:')}",
            query=query_data,
            count=qualifying,
            base_count=base_count,
            share=qualifying / base_count if base_count else 0.0,
            population=f"Уникальные клиенты из {self.dataset_ref}; client_id использован только внутри SQLite.",
            feature_summaries=feature_summaries,
            aggregate_summary=aggregate_summary,
            quality={"missing_required_event_values": missing, "raw_records_shared_with_model": False},
        )

    def review_evidence(self, query: AudienceQuery) -> dict[str, Any]:
        """Return safe aggregate evidence for critic and validator review."""
        start, end = _parse_window(query.window_start, query.window_end)
        errors = self.validate_query(query, start, end)
        if errors:
            raise ValueError("; ".join(errors))
        with self._connect() as connection:
            conditions: list[dict[str, Any]] = []
            for condition in query.conditions:
                condition_sql, condition_params = _condition_sql(condition)
                event_count, unique_clients = connection.execute(
                    "SELECT COUNT(*), COUNT(DISTINCT client_id) FROM events "
                    f"WHERE event_date >= ? AND event_date < ? AND {condition_sql}",
                    [start.isoformat(), end.isoformat(), *condition_params],
                ).fetchone()
                monthly = [
                    {"month": month, "event_count": event_count, "unique_client_count": unique_clients}
                    for month, event_count, unique_clients in connection.execute(
                        "SELECT substr(event_date, 1, 7), COUNT(*), COUNT(DISTINCT client_id) FROM events "
                        f"WHERE event_date >= ? AND event_date < ? AND {condition_sql} GROUP BY 1 ORDER BY 1 "
                        f"LIMIT {_REVIEW_MAX_ROWS}",
                        [start.isoformat(), end.isoformat(), *condition_params],
                    ).fetchall()
                ]
                value_distributions = []
                for field in sorted({filter_.field for filter_ in condition.filters}):
                    rows = connection.execute(
                        f"SELECT {_quote(field)}, COUNT(*), COUNT(DISTINCT client_id) FROM events "
                        f"WHERE event_date >= ? AND event_date < ? GROUP BY {_quote(field)} "
                        f"ORDER BY COUNT(*) DESC LIMIT {_REVIEW_MAX_ROWS}",
                        [start.isoformat(), end.isoformat()],
                    ).fetchall()
                    value_distributions.append(
                        {
                            "field": field,
                            "values": [
                                {"value": row[0], "event_count": row[1], "unique_client_count": row[2]}
                                for row in rows
                                if row[0] is not None
                            ],
                        }
                    )
                conditions.append(
                    {
                        "name": condition.name,
                        "filters": [filter_.model_dump(mode="json") for filter_ in condition.filters],
                        "event_count": event_count,
                        "unique_client_count": unique_clients,
                        "monthly": monthly,
                        "value_distributions": value_distributions,
                    }
                )
            overlaps = []
            for left_index, left in enumerate(query.conditions):
                left_sql, left_params = _condition_sql(left)
                for right in query.conditions[left_index + 1 :]:
                    right_sql, right_params = _condition_sql(right)
                    overlap = connection.execute(
                        "SELECT COUNT(*) FROM (SELECT client_id FROM events "
                        "WHERE event_date >= ? AND event_date < ? GROUP BY client_id HAVING "
                        f"MAX(CASE WHEN {left_sql} THEN 1 ELSE 0 END) = 1 AND "
                        f"MAX(CASE WHEN {right_sql} THEN 1 ELSE 0 END) = 1)",
                        [start.isoformat(), end.isoformat(), *left_params, *right_params],
                    ).fetchone()[0]
                    overlaps.append({"left": left.name, "right": right.name, "unique_client_count": overlap})
        return {
            "window": {"start": start.isoformat(), "end": end.isoformat()},
            "conditions": conditions,
            "condition_overlaps": overlaps,
            "privacy_boundary": (
                "Aggregate evidence only; no raw rows, client_id values, contacts, or membership lists are included."
            ),
        }

    def inspect_evidence(self, query: AudienceQuery, request: Any) -> dict[str, Any]:
        """Execute one bounded, aggregate-only evidence request."""
        evidence = self.review_evidence(query)
        kind = request.kind if hasattr(request, "kind") else request["kind"]
        condition_name = request.condition_name if hasattr(request, "condition_name") else request.get("condition_name")
        field = request.field if hasattr(request, "field") else request.get("field")
        if kind == "condition_breakdown":
            matches = [item for item in evidence["conditions"] if not condition_name or item["name"] == condition_name]
            if not matches:
                raise ValueError(f"condition is not present in the current query: {condition_name}")
            return {"kind": kind, "conditions": matches}
        if kind == "monthly_breakdown":
            return {
                "kind": kind,
                "conditions": [{"name": item["name"], "monthly": item["monthly"]} for item in evidence["conditions"]],
            }
        if kind == "condition_overlap":
            return {"kind": kind, "condition_overlaps": evidence["condition_overlaps"]}
        if kind == "value_distribution":
            distributions = [
                distribution
                for condition in evidence["conditions"]
                for distribution in condition["value_distributions"]
                if not field or distribution["field"] == field
            ]
            if not distributions:
                raise ValueError(f"field is not present in the current query evidence: {field}")
            return {"kind": kind, "value_distributions": distributions}
        raise ValueError(f"unsupported evidence request: {kind}")

    def validate_query(self, query: AudienceQuery, start: date, end: date) -> list[str]:
        errors: list[str] = []
        with self._connect() as connection:
            columns = _table_columns(connection)
            min_day, max_day = connection.execute("SELECT MIN(event_date), MAX(event_date) FROM events").fetchone()
            if (
                start.isoformat() < min_day
                or end.isoformat() > (date.fromisoformat(max_day) + timedelta(days=1)).isoformat()
            ):
                errors.append(
                    f"query window must be within the dataset date range {min_day} through {max_day}; "
                    "window_end is exclusive and must not include an incomplete final month"
                )
            catalog = {
                field: {
                    row[0]
                    for row in connection.execute(
                        f"SELECT DISTINCT {_quote(field)} FROM events WHERE {_quote(field)} IS NOT NULL"
                    )
                    if row[0] is not None
                }
                for field in _QUERYABLE_FIELDS.intersection(columns)
            }
        if not _months_in_window(start, end):
            errors.append(
                "query window boundaries must both be the first day of a calendar month; window_end is exclusive"
            )
        for condition in query.conditions:
            for filter_ in condition.filters:
                if filter_.field not in catalog:
                    errors.append(f"field is not allowed for segmentation: {filter_.field}")
                elif filter_.value not in catalog[filter_.field]:
                    errors.append(f"value is not present for {filter_.field}: {filter_.value}")
        return list(dict.fromkeys(errors))

    def _connect(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self.db_path)
        connection.row_factory = sqlite3.Row
        return connection

    def _ensure_database(self) -> None:
        self.db_path.parent.mkdir(parents=True, exist_ok=True)
        try:
            with self._connect() as connection:
                existing = connection.execute("SELECT value FROM dataset_metadata WHERE key = 'dataset_ref'").fetchone()
                if existing and existing[0] == self.dataset_ref:
                    return
        except sqlite3.OperationalError:
            pass
        records = _read_workbook(self.path)
        if not records:
            raise ValueError("audience workbook has no event rows")
        columns = list(records[0])
        missing = {_CLIENT_KEY, _TIMESTAMP}.difference(columns)
        if missing:
            raise ValueError("audience workbook is missing required columns: " + ", ".join(sorted(missing)))
        with self._connect() as connection:
            connection.executescript("DROP TABLE IF EXISTS events; DROP TABLE IF EXISTS dataset_metadata;")
            schema = ", ".join(f"{_quote(column)} TEXT" for column in columns)
            connection.execute(f"CREATE TABLE events ({schema}, {_quote(_EVENT_DATE)} TEXT NOT NULL)")
            insert_columns = [*columns, _EVENT_DATE]
            placeholders = ", ".join("?" for _ in insert_columns)
            connection.executemany(
                f"INSERT INTO events ({', '.join(_quote(column) for column in insert_columns)}) "
                f"VALUES ({placeholders})",
                [
                    [record.get(column) for column in columns] + [_event_date(record[_TIMESTAMP]).isoformat()]
                    for record in records
                ],
            )
            connection.execute("CREATE INDEX events_client_date ON events (client_id, event_date)")
            connection.execute("CREATE INDEX events_date ON events (event_date)")
            connection.execute("CREATE TABLE dataset_metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL)")
            connection.execute("INSERT INTO dataset_metadata VALUES ('dataset_ref', ?)", (self.dataset_ref,))
            connection.execute("INSERT INTO dataset_metadata VALUES ('source_sheet', ?)", (_EVENT_SHEET,))


def _condition_sql(condition: Any) -> tuple[str, list[str]]:
    return " AND ".join(f"{_quote(filter_.field)} = ?" for filter_ in condition.filters), [
        filter_.value for filter_ in condition.filters
    ]


def _segment_aggregate_summary(
    connection: sqlite3.Connection,
    qualifying_sql: str,
    qualifying_params: list[Any],
    start: date,
    end: date,
    months: list[date],
    client_count: int,
) -> dict[str, Any]:
    """Return aggregate characteristics for qualifying clients only."""
    join = f"JOIN ({qualifying_sql}) AS segment_clients ON segment_clients.client_id = events.client_id"
    columns = _table_columns(connection)
    profile_filter = ""
    profile_params: list[str] = []
    if "event_type" in columns:
        placeholders = ", ".join("?" for _ in _PROFILE_EXCLUDED_EVENT_TYPES)
        profile_filter = f" AND (events.event_type IS NULL OR events.event_type NOT IN ({placeholders}))"
        profile_params = sorted(_PROFILE_EXCLUDED_EVENT_TYPES)
    event_count = connection.execute(
        f"SELECT COUNT(*) FROM events {join} WHERE events.event_date >= ? AND events.event_date < ?{profile_filter}",
        [*qualifying_params, start.isoformat(), end.isoformat(), *profile_params],
    ).fetchone()[0]
    monthly = dict(
        connection.execute(
            f"SELECT substr(events.event_date, 1, 7), COUNT(*) FROM events {join} "
            f"WHERE events.event_date >= ? AND events.event_date < ?{profile_filter} GROUP BY 1",
            [*qualifying_params, start.isoformat(), end.isoformat(), *profile_params],
        ).fetchall()
    )
    monthly_event_counts = [
        {"month": month.isoformat(), "event_count": monthly.get(month.strftime("%Y-%m"), 0)} for month in months
    ]
    distributions = []
    for field in sorted(_QUERYABLE_FIELDS.intersection(_table_columns(connection))):
        rows = connection.execute(
            f"SELECT events.{_quote(field)}, COUNT(*), COUNT(DISTINCT events.client_id) "
            f"FROM events {join} WHERE events.event_date >= ? AND events.event_date < ? "
            f"{profile_filter} AND events.{_quote(field)} IS NOT NULL AND events.{_quote(field)} != '' "
            f"GROUP BY events.{_quote(field)} ORDER BY COUNT(*) DESC LIMIT 5",
            [*qualifying_params, start.isoformat(), end.isoformat(), *profile_params],
        ).fetchall()
        if rows:
            distributions.append(
                {
                    "field": field,
                    "values": [
                        {
                            "value": row[0],
                            "event_count": row[1],
                            "unique_client_count": row[2],
                            "event_share": row[1] / event_count if event_count else 0.0,
                        }
                        for row in rows
                    ],
                }
            )
    return {
        "matched_client_count": client_count,
        "event_count": event_count,
        "average_events_per_client": event_count / client_count if client_count else 0.0,
        "average_events_per_client_per_month": event_count / client_count / len(months)
        if client_count and months
        else 0.0,
        "monthly_event_counts": monthly_event_counts,
        "value_distributions": distributions,
        "excluded_event_types": sorted(_PROFILE_EXCLUDED_EVENT_TYPES) if "event_type" in columns else [],
        "privacy_boundary": "Aggregate characteristics only; client_id and membership remain private.",
    }


def _quote(identifier: str) -> str:
    if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", identifier):
        raise ValueError(f"invalid SQL identifier: {identifier}")
    return f'"{identifier}"'


def _table_columns(connection: sqlite3.Connection) -> set[str]:
    return {row[1] for row in connection.execute("PRAGMA table_info(events)")}


def _read_workbook(path: Path) -> list[dict[str, str | None]]:
    with ZipFile(path) as workbook:
        shared = _shared_strings(workbook)
        sheet_paths = _sheet_paths(workbook)
        if _EVENT_SHEET not in sheet_paths:
            raise ValueError(f"audience workbook must include a '{_EVENT_SHEET}' sheet")
        event_rows = _sheet_rows(workbook, sheet_paths[_EVENT_SHEET], shared)
        if not event_rows:
            return []
        headers = event_rows[0]
        return [{header: row.get(column) for column, header in headers.items() if header} for row in event_rows[1:]]


def _shared_strings(workbook: ZipFile) -> list[str]:
    if "xl/sharedStrings.xml" not in workbook.namelist():
        return []
    root = ET.fromstring(workbook.read("xl/sharedStrings.xml"))
    return ["".join(item.itertext()) for item in root.findall("m:si", _NS)]


def _sheet_paths(workbook: ZipFile) -> dict[str, str]:
    workbook_xml = ET.fromstring(workbook.read("xl/workbook.xml"))
    rels_xml = ET.fromstring(workbook.read("xl/_rels/workbook.xml.rels"))
    relationships = {item.attrib["Id"]: item.attrib["Target"] for item in rels_xml}
    paths = {}
    for sheet in workbook_xml.findall("m:sheets/m:sheet", _NS):
        target = relationships[sheet.attrib["{http://schemas.openxmlformats.org/officeDocument/2006/relationships}id"]]
        paths[sheet.attrib["name"]] = target.lstrip("/") if target.startswith("/") else f"xl/{target}"
    return paths


def _sheet_rows(workbook: ZipFile, path: str, shared: list[str]) -> list[dict[str, str | None]]:
    root = ET.fromstring(workbook.read(path))
    return [
        {_column(cell.attrib["r"]): _cell_value(cell, shared) for cell in row.findall("m:c", _NS)}
        for row in root.findall(".//m:sheetData/m:row", _NS)
    ]


def _column(reference: str) -> str:
    match = re.match(r"[A-Z]+", reference)
    if match is None:
        raise ValueError(f"invalid spreadsheet cell reference: {reference}")
    return match.group(0)


def _cell_value(cell: ET.Element, shared: list[str]) -> str | None:
    if cell.get("t") == "inlineStr":
        return "".join(cell.itertext())
    value = cell.find("m:v", _NS)
    if value is None:
        return None
    return shared[int(value.text)] if cell.get("t") == "s" else value.text


def _event_date(value: str | None) -> date:
    if value is None:
        raise ValueError("event_timestamp cannot be empty")
    return (datetime(1899, 12, 30) + timedelta(days=float(value))).date()


def _parse_window(start: str, end: str) -> tuple[date, date]:
    try:
        parsed_start = date.fromisoformat(start)
        parsed_end = date.fromisoformat(end)
    except ValueError as error:
        raise ValueError("window_start and window_end must use YYYY-MM-DD") from error
    if parsed_start >= parsed_end:
        raise ValueError("window_start must be before window_end")
    return parsed_start, parsed_end


def _months_in_window(start: date, end: date) -> list[date]:
    if start.day != 1 or end.day != 1:
        return []
    months = []
    current = start
    while current < end:
        months.append(current)
        current = date(current.year + (current.month == 12), current.month % 12 + 1, 1)
    return months


def _file_digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()[:12]


def _digest_json(value: dict[str, Any]) -> str:
    return hashlib.sha256(json.dumps(value, ensure_ascii=False, sort_keys=True).encode()).hexdigest()[:12]

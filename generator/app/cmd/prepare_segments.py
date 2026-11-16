from __future__ import annotations

import argparse
import csv
import hashlib
import json
import os
import subprocess
import tempfile
from pathlib import Path
from uuid import uuid4

from psycopg.types.json import Jsonb

from app.db import connect
from app.migrations import apply

ROOT = Path(__file__).resolve().parents[3]
EVAL = ROOT / "eval"
SYNTHETIC_DATASET_SHA256 = "1a00d779e0b668f31c0b74547d83a084b87ced0f75498ec7762fae6da071f99b"
TBANK_DATASET_SHA256 = "0eaca9efdbf3ba64276d026d7fa2d362f9cf7cba9cfabff26bd765cf5f963b7a"
PREPARATION_NOTEBOOK = EVAL / "farida_oct/Подготовка данных аудиторий.ipynb"
SEGMENTATION_NOTEBOOK = EVAL / "farida_oct/Сегментация аудиторий.ipynb"
NOTEBOOK_RUNNER = """
import json, sys
from pathlib import Path
notebook = json.loads(Path(sys.argv[1]).read_text(encoding='utf-8'))
namespace = {'__name__': '__main__'}
for index, cell in enumerate(notebook['cells']):
    if cell['cell_type'] == 'code':
        exec(compile(''.join(cell['source']), f'{sys.argv[1]}:cell-{index}', 'exec'), namespace)
"""


def main() -> None:
    parser = argparse.ArgumentParser(description="Prepare and store a versioned audience segment dataset")
    parser.add_argument("--dataset", type=Path, default=EVAL / "События аудиторий (датасет).xlsx")
    parser.add_argument("--python", type=Path, default=EVAL / ".venv/bin/python")
    args = parser.parse_args()
    dataset = args.dataset.resolve(strict=True)
    python = args.python.absolute()
    if not python.is_file():
        raise ValueError(f"notebook interpreter does not exist: {python}")

    database_url = os.environ.get("DATABASE_URL")
    if not database_url:
        raise ValueError("DATABASE_URL is required")

    with dataset.open("rb") as file:
        digest = hashlib.file_digest(file, "sha256").hexdigest()

    if digest == SYNTHETIC_DATASET_SHA256:
        dataset_environment = {
            "AUDIENCE_ID_COLUMN": "",
            "AUDIENCE_TIME_COLUMN": "",
            "AUDIENCE_DECIMAL_COMMA": "false",
            "DATA_PROVENANCE": "synthetic_demo",
        }
    elif digest == TBANK_DATASET_SHA256:
        dataset_environment = {
            "AUDIENCE_ID_COLUMN": "id",
            "AUDIENCE_TIME_COLUMN": "date",
            "AUDIENCE_DECIMAL_COMMA": "true",
            "DATA_PROVENANCE": "real_clients",
        }
    else:
        raise ValueError("dataset is not one of the supported demo or T-Bank datasets")

    with tempfile.TemporaryDirectory(prefix="audience-preparation-") as temporary:
        directory = Path(temporary)
        environment = {
            **os.environ,
            **dataset_environment,
            "AUDIENCE_INPUT_PATH": str(dataset),
            "AUDIENCE_OUTPUT_DIR": str(directory / "prep"),
        }
        subprocess.run(  # noqa: S603 -- interpreter is explicitly chosen by the CLI operator
            [str(python), "-c", NOTEBOOK_RUNNER, str(PREPARATION_NOTEBOOK)],
            env=environment,
            cwd=directory,
            check=True,
        )
        environment.update(
            {
                "SEGMENTATION_INPUT_PATH": str(directory / "prep.zip"),
                "SEGMENTATION_OUTPUT_DIR": str(directory / "segments"),
            }
        )
        subprocess.run(  # noqa: S603 -- interpreter is explicitly chosen by the CLI operator
            [str(python), "-c", NOTEBOOK_RUNNER, str(SEGMENTATION_NOTEBOOK)],
            env=environment,
            cwd=directory,
            check=True,
        )
        output = directory / "segments"
        candidates = json.loads((output / "segment_candidates.json").read_text(encoding="utf-8"))
        report = json.loads((output / "segmentation_report.json").read_text(encoding="utf-8"))
        preparation_report = json.loads((directory / "prep/run_report.json").read_text(encoding="utf-8"))
        segmentation_model = json.loads((output / "segmentation_model.json").read_text(encoding="utf-8"))
        with (output / "client_segments.csv").open(encoding="utf-8-sig", newline="") as file:
            members = list(csv.DictReader(file))

        segment_set_id = uuid4()
        apply(database_url)
        with connect(database_url) as connection, connection.cursor() as cursor:
            cursor.execute(
                "INSERT INTO generator.segment_set (id, source_digest, metadata, created_at) "
                "VALUES (%s, %s, %s, NOW())",
                (
                    segment_set_id,
                    digest,
                    Jsonb(
                        {
                            "report": report,
                            "preparation_report": preparation_report,
                            "segmentation_model": segmentation_model,
                            "source": dataset.name,
                            "notebook_sha256": {
                                notebook.name: hashlib.sha256(notebook.read_bytes()).hexdigest()
                                for notebook in (PREPARATION_NOTEBOOK, SEGMENTATION_NOTEBOOK)
                            },
                        }
                    ),
                ),
            )
            for segment in candidates["segments"]:
                cursor.execute(
                    "INSERT INTO generator.segment (segment_set_id, segment_id, candidate) VALUES (%s, %s, %s)",
                    (segment_set_id, segment["segment_id"], Jsonb(segment)),
                )

            for member in members:
                cursor.execute(
                    "INSERT INTO generator.segment_membership (segment_set_id, segment_id, client_id) "
                    "VALUES (%s, %s, %s)",
                    (segment_set_id, member["segment_id"], member["client_id"]),
                )

        print(f"Stored segment dataset: {segment_set_id}")


if __name__ == "__main__":
    main()

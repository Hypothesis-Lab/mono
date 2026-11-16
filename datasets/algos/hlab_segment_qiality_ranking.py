"""Оценка сегментов после двух ноутбуков H-Lab; исходные ноутбуки не меняет.

В Colab:
    !pip -q install pandas numpy scikit-learn
    !python HLab_quality_top3_after_segmentation.py \
        --prepared "/content/АРХИВ_ПОДГОТОВКИ.zip" \
        --segmented "/content/АРХИВ_СЕГМЕНТАЦИИ.zip" \
        --out /content/quality_top3

Результат: segment_quality_ranking.csv и segment_quality_top3.json.
Это топ-3 по внутреннему качеству групп, не по соответствию продукту.
"""

from __future__ import annotations

import argparse
import io
import json
import zipfile
from pathlib import Path

import numpy as np
import pandas as pd
from sklearn.metrics import silhouette_samples


# Пилотные значения: уточнять на реальных данных, не считать проверкой бизнес-эффекта.
WEIGHTS = {"completeness": 0.30, "separation": 0.35, "stability": 0.35}
PASS = {"completeness": 0.80, "separation": 0.00, "stability": 0.70}
WEAK = {"completeness": 0.60, "separation": -0.10, "stability": 0.40}


def _read_zip(path: Path, required: set[str]) -> dict[str, bytes]:
    if not path.is_file():
        raise FileNotFoundError(f"Архив не найден: {path}")
    with zipfile.ZipFile(path) as archive:
        # Иногда ZIP содержит папку верхнего уровня: разрешаем ее, но не дубли.
        found: dict[str, str] = {}
        for member in archive.namelist():
            base = Path(member).name
            if base in required:
                if base in found:
                    raise ValueError(f"В {path.name} несколько файлов {base}")
                found[base] = member
        missing = required - found.keys()
        if missing:
            raise ValueError(f"В {path.name} отсутствуют файлы: {', '.join(sorted(missing))}")
        return {name: archive.read(member) for name, member in found.items()}


def _csv(data: bytes) -> pd.DataFrame:
    # ID читается как строка, чтобы не терялись лидирующие нули.
    return pd.read_csv(io.BytesIO(data), dtype={"__entity_id": "string", "client_id": "string"})


def _json(data: bytes) -> dict:
    result = json.loads(data)
    if not isinstance(result, dict):
        raise ValueError("Ожидался JSON-объект")
    return result


def _unique_ids(frame: pd.DataFrame, name: str, column: str) -> pd.Index:
    if column not in frame.columns or frame[column].isna().any() or frame[column].duplicated().any():
        raise ValueError(f"{name}: колонка {column} должна содержать уникальные непустые ID")
    return pd.Index(frame[column].astype(str), name=column)


def _sample_indices(labels: np.ndarray, limit: int, seed: int) -> np.ndarray:
    if len(labels) <= limit:
        return np.arange(len(labels))
    rng = np.random.default_rng(seed)
    groups = [np.flatnonzero(labels == key) for key in np.unique(labels)]
    quota = min(20, max(1, limit // len(groups)))
    first = np.concatenate([rng.choice(g, size=min(quota, len(g)), replace=False) for g in groups])
    remaining = np.setdiff1d(np.arange(len(labels)), first)
    extra = rng.choice(remaining, size=limit - len(first), replace=False)
    return np.sort(np.concatenate([first, extra]))


def evaluate_data(matrix: pd.DataFrame, profile: pd.DataFrame, assignment: pd.DataFrame,
                  candidates: dict, report: dict, model: dict, *, sample: int = 2000,
                  seed: int = 42) -> tuple[list[dict], dict]:
    if sample < 2:
        raise ValueError("sample должен быть не меньше 2")

    ids = _unique_ids(matrix, "Матрица", "__entity_id")
    profile_ids = _unique_ids(profile, "Профиль", "__entity_id")
    assigned_ids = _unique_ids(assignment, "Сегментация", "client_id")
    if set(ids) != set(profile_ids) or set(ids) != set(assigned_ids):
        raise ValueError("ID в архивах отличаются: проверьте, что сегментация сделана на этом ZIP подготовки")
    profile = profile.set_index(profile_ids).loc[ids].reset_index(drop=True)
    assignment = assignment.set_index(assigned_ids).loc[ids].reset_index(drop=True)
    if "segment_id" not in assignment or assignment.segment_id.isna().any():
        raise ValueError("Не найдены заполненные segment_id")
    labels = assignment.segment_id.astype(str).to_numpy()
    names = list(matrix.columns.drop("__entity_id"))
    if names != model.get("features"):
        raise ValueError("Признаки модели и матрицы не совпадают: вероятно, архивы от разных запусков")
    numeric = matrix[names]
    if numeric.empty or any(not pd.api.types.is_numeric_dtype(numeric[c]) for c in names):
        raise ValueError("Матрица должна содержать числовые признаки")
    raw_x = numeric.to_numpy(dtype=float)
    feature_weights = model.get("weights", {})
    if set(feature_weights) != set(names):
        raise ValueError("Веса модели не соответствуют признакам матрицы")
    weights = np.array([feature_weights[c] for c in names], dtype=float)
    x = raw_x * weights
    centers = np.asarray(model.get("weighted_centers"), dtype=float)
    if (centers.ndim != 2 or centers.shape[1] != len(names) or
            not np.isfinite(x).all() or not np.isfinite(weights).all() or
            not np.isfinite(centers).all()):
        raise ValueError("Взвешенная матрица или центры модели некорректны")
    mapping = model.get("label_mapping", {})
    if set(mapping.values()) != set(labels) or len(mapping) != len(centers):
        raise ValueError("Метки сегментов не соответствуют сохраненной модели")
    # Проверяем не только совпадение ID, но и что метки от той же обученной модели.
    distances = ((x[:, None, :] - centers[None, :, :]) ** 2).sum(axis=2)
    predicted = np.array([mapping[str(index)] for index in distances.argmin(axis=1)])
    if not np.array_equal(predicted, labels):
        raise ValueError("Метки не воспроизводятся центрами модели: проверьте соответствие двух ZIP")

    segment_ids = sorted(set(labels))
    if len(segment_ids) < 2:
        raise ValueError("Для Silhouette нужны хотя бы два сегмента")
    if len(segment_ids) >= len(labels):
        raise ValueError("Для Silhouette число сегментов должно быть меньше числа клиентов")
    sample_indices = _sample_indices(labels, sample, seed)
    sampled_labels = labels[sample_indices]
    if len(set(sampled_labels)) >= len(sample_indices):
        raise ValueError("Подвыборка Silhouette содержит только уникальные метки")
    sil_values = silhouette_samples(x[sample_indices], sampled_labels)

    original = candidates.get("segments")
    if not isinstance(original, list) or {s.get("segment_id") for s in original} != set(segment_ids):
        raise ValueError("segment_candidates.json не соответствует назначенным сегментам")
    original_by_id = {s["segment_id"]: s for s in original}
    if len(original_by_id) != len(original):
        raise ValueError("В segment_candidates.json повторяются segment_id")
    bootstrap = report.get("bootstrap_stability", {}).get("per_original_label_mean_jaccard", {})
    if set(mapping) != set(bootstrap):
        raise ValueError("Для всех сегментов нужны сохраненные оценки устойчивости из отчета")
    if report.get("selected_k") != len(segment_ids) or report.get("clients") != len(labels):
        raise ValueError("Число клиентов или групп в segmentation_report.json не совпадает")

    # Это полнота агрегированного профиля, а не исходных событий и не истинность значений.
    source_cols = [c for c in profile if c != "__entity_id"]
    if not source_cols:
        raise ValueError("В профиле клиентов нет ни одного признака")
    observed = profile[source_cols].notna().to_numpy()
    rows: list[dict] = []
    for segment_id in segment_ids:
        member = labels == segment_id
        in_sample = sampled_labels == segment_id
        if not in_sample.any():
            raise ValueError(f"Сегмент {segment_id} не попал в выборку Silhouette")
        old_label = next(old for old, new in mapping.items() if new == segment_id)
        completeness = float(observed[member].mean())
        separation = float(sil_values[in_sample].mean())
        stability = float(bootstrap[old_label])
        if not 0 <= stability <= 1 or not -1 <= separation <= 1:
            raise ValueError(f"Метрики сегмента {segment_id} вне допустимых границ")
        score = 100 * (WEIGHTS["completeness"] * completeness +
                       WEIGHTS["separation"] * (separation + 1) / 2 +
                       WEIGHTS["stability"] * stability)
        measures = {"completeness": completeness, "separation": separation,
                    "stability": stability}
        weak = [metric for metric, value in measures.items() if value < WEAK[metric]]
        below_pass = [metric for metric, value in measures.items() if value < PASS[metric]]
        status = "weak" if weak else "review" if below_pass else "pass"
        candidate = original_by_id[segment_id]
        if candidate.get("size") != int(member.sum()):
            raise ValueError(f"Размер сегмента {segment_id} в JSON не совпадает с назначениями")
        rows.append({
            "segment_id": segment_id,
            "n_clients": int(member.sum()),
            "completeness": round(completeness, 4),
            "mean_silhouette": round(separation, 4),
            "silhouette_sample_clients": int(in_sample.sum()),
            "bootstrap_jaccard": round(stability, 4),
            "quality_score": round(score, 2),
            "quality_status": status,
            "below_pass_metrics": below_pass,
            "key_features": candidate.get("key_features", []),
            "existing_risks": candidate.get("risks", []),
        })
    rows.sort(key=lambda row: (-row["quality_score"], row["segment_id"]))
    for rank, row in enumerate(rows, start=1):
        row["rank"] = rank
    warnings = list(report.get("warnings", []))
    warnings.extend([
        "Полнота измерена по агрегированному профилю; пропуски в сырых событиях здесь не видны",
        "Устойчивость взята из bootstrap отчета сегментации; LOFO и проверка на эталоне здесь не выполнены",
        "Качество сегментации не означает пригодность сегмента для продукта или кампании",
    ])
    if len(rows) < 3:
        warnings.append("Алгоритм выделил меньше трех сегментов: топ содержит все доступные")
    summary = {
        "status": "exploratory_quality_ranking",
        "n_clients": len(labels), "n_segments": len(rows),
        "global_sample_silhouette": round(float(sil_values.mean()), 4),
        "silhouette_sample_size": len(sample_indices),
        "weights": WEIGHTS, "pass_thresholds": PASS, "weak_thresholds": WEAK,
        "top3_quality_candidates": rows[:3], "ranking": rows,
        "warnings": warnings,
    }
    return rows, summary


def evaluate(prepared: Path, segmented: Path, *, sample: int = 2000,
             seed: int = 42) -> tuple[list[dict], dict]:
    prep = _read_zip(prepared, {"segmentation_matrix.csv", "client_features_combined.csv"})
    seg = _read_zip(segmented, {"client_segments.csv", "segment_candidates.json",
                                "segmentation_report.json", "segmentation_model.json"})
    rows, summary = evaluate_data(
        _csv(prep["segmentation_matrix.csv"]), _csv(prep["client_features_combined.csv"]),
        _csv(seg["client_segments.csv"]), _json(seg["segment_candidates.json"]),
        _json(seg["segmentation_report.json"]), _json(seg["segmentation_model.json"]),
        sample=sample, seed=seed,
    )
    summary["source_archives"] = {"prepared": prepared.name, "segmented": segmented.name}
    return rows, summary


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    prepared = parser.add_mutually_exclusive_group(required=True)
    prepared.add_argument("--prepared", type=Path, help="ZIP подготовки данных")
    prepared.add_argument("--prepared-dir", type=Path, help="Папка результатов подготовки")
    segmented = parser.add_mutually_exclusive_group(required=True)
    segmented.add_argument("--segmented", type=Path, help="ZIP сегментации")
    segmented.add_argument("--segmented-dir", type=Path, help="Папка результатов сегментации")
    parser.add_argument("--out", type=Path, default=Path("quality_top3"), help="Папка результатов")
    parser.add_argument("--sample", type=int, default=2000, help="Максимум клиентов для Silhouette")
    args = parser.parse_args()
    if args.prepared and args.segmented:
        ranking, summary = evaluate(args.prepared, args.segmented, sample=args.sample)
    elif args.prepared_dir and args.segmented_dir:
        prep, seg = args.prepared_dir, args.segmented_dir
        ranking, summary = evaluate_data(
            _csv((prep / "segmentation_matrix.csv").read_bytes()),
            _csv((prep / "client_features_combined.csv").read_bytes()),
            _csv((seg / "client_segments.csv").read_bytes()),
            _json((seg / "segment_candidates.json").read_bytes()),
            _json((seg / "segmentation_report.json").read_bytes()),
            _json((seg / "segmentation_model.json").read_bytes()),
            sample=args.sample,
        )
    else:
        parser.error("используйте два ZIP или две папки")
    args.out.mkdir(parents=True, exist_ok=True)
    csv_rows = [{k: json.dumps(v, ensure_ascii=False) if isinstance(v, list) else v
                 for k, v in row.items()} for row in ranking]
    pd.DataFrame(csv_rows).to_csv(args.out / "segment_quality_ranking.csv", index=False, encoding="utf-8-sig")
    (args.out / "segment_quality_top3.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2, allow_nan=False), encoding="utf-8"
    )
    print(f"Клиенты: {summary['n_clients']}; сегменты: {summary['n_segments']}")
    for row in ranking[:3]:
        print(f"{row['rank']}. {row['segment_id']}: {row['quality_score']:.2f} / 100 "
              f"({row['quality_status']}), клиентов: {row['n_clients']}")
    print(f"Результаты: {args.out.resolve()}")


if __name__ == "__main__":
    main()

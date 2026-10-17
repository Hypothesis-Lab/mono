from __future__ import annotations

import hashlib
import json
import math
from contextlib import suppress
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from fastembed import TextEmbedding
from fastembed.common.model_description import ModelSource, PoolingType

MODEL_NAME = "intfloat/multilingual-e5-small"
_FIELD_DESCRIPTIONS = {
    "source_system": "Система, из которой поступило событие.",
    "event_type": "Тип наблюдаемого события.",
    "account_scope": "Контекст счёта, к которому относится событие.",
    "transaction_category": "Категория наблюдаемой операции.",
    "expense_category": "Категория расхода, если она применима к событию.",
    "mcc_code": "Стандартный MCC-код категории торговой точки; расшифровывается отдельным справочником MCC.",
    "mcc_role": "Роль торговой точки в наблюдаемой операции.",
    "account_types": "Доступные типы счетов клиента.",
    "bank_products": "Банковские продукты, связанные с событием.",
    "app_section": "Раздел приложения, посещённый пользователем.",
}


@dataclass(frozen=True)
class FeatureCard:
    id: str
    text: str
    source_columns: list[str]
    filters: list[dict[str, str]]

    def for_model(self, score: float | None = None) -> dict[str, Any]:
        return {
            "id": self.id,
            "description": self.text,
            "source_columns": self.source_columns,
            "filters": self.filters,
            **({"semantic_score": round(score, 4)} if score is not None else {}),
        }


class LocalFeatureRetriever:
    """Persistent local vector retrieval over safe feature cards only."""

    def __init__(self, profile: dict[str, Any], index_dir: Path) -> None:
        self.cards = build_feature_cards(profile)
        self.index_path = index_dir / f"{profile['dataset_ref'].replace(':', '-')}.json"
        self._vectors: list[list[float]] | None = None
        self._card_digest = _card_digest(self.cards)

    def search(self, product_text: str, limit: int = 16) -> list[dict[str, Any]]:
        vectors = self._load_or_build_vectors()
        query_vector = _embed([f"query: {product_text}"])[0]
        scored = sorted(
            ((_cosine(query_vector, vector), card) for card, vector in zip(self.cards, vectors, strict=True)),
            key=lambda item: item[0],
            reverse=True,
        )[:limit]
        return [card.for_model(score) for score, card in scored]

    def _load_or_build_vectors(self) -> list[list[float]]:
        if self._vectors is not None:
            return self._vectors
        try:
            cached = json.loads(self.index_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            cached = None
        if (
            isinstance(cached, dict)
            and cached.get("model") == MODEL_NAME
            and cached.get("card_digest") == self._card_digest
            and isinstance(cached.get("vectors"), list)
        ):
            self._vectors = cached["vectors"]
            return self._vectors
        vectors = _embed([f"passage: {card.text}" for card in self.cards])
        self.index_path.parent.mkdir(parents=True, exist_ok=True)
        self.index_path.write_text(
            json.dumps(
                {
                    "model": MODEL_NAME,
                    "card_digest": self._card_digest,
                    "vectors": vectors,
                },
                ensure_ascii=False,
            ),
            encoding="utf-8",
        )
        self._vectors = vectors
        return vectors


def build_feature_cards(profile: dict[str, Any]) -> list[FeatureCard]:
    cards = []
    for field, values in profile["value_catalog"].items():
        description = _FIELD_DESCRIPTIONS.get(field, f"Допустимое поле набора данных: {field}.")
        for item in values:
            value = item["value"]
            filters = [{"field": field, "value": value}]
            card_id = f"{field}:{value}"
            cards.append(
                FeatureCard(
                    id=card_id,
                    text=(
                        f"Признак данных: {field} = {value}. {description} "
                        f"Наблюдаемых событий с этим значением: {item['event_count']}. "
                        f"Точный фильтр: {field} = {value}."
                    ),
                    source_columns=[field],
                    filters=filters,
                )
            )
    return cards


def product_text(passport: dict[str, Any]) -> str:
    records = passport.get("records", [])
    return "\n".join(
        f"{record.get('category', '')}: {record.get('subject', '')}. {record.get('statement', '')}"
        for record in records
        if isinstance(record, dict)
    )


def _embed(texts: list[str]) -> list[list[float]]:
    with suppress(ValueError):
        TextEmbedding.add_custom_model(
            model=MODEL_NAME,
            pooling=PoolingType.MEAN,
            normalization=True,
            sources=ModelSource(hf=MODEL_NAME),
            dim=384,
            model_file="onnx/model.onnx",
        )
    model = TextEmbedding(model_name=MODEL_NAME, providers=["CPUExecutionProvider"])
    return [[float(value) for value in vector] for vector in model.embed(texts)]


def _cosine(left: list[float], right: list[float]) -> float:
    numerator = sum(a * b for a, b in zip(left, right, strict=True))
    left_norm = math.sqrt(sum(value * value for value in left))
    right_norm = math.sqrt(sum(value * value for value in right))
    return numerator / (left_norm * right_norm) if left_norm and right_norm else 0.0


def _card_digest(cards: list[FeatureCard]) -> str:
    payload = [{"id": card.id, "text": card.text} for card in cards]
    return hashlib.sha256(json.dumps(payload, ensure_ascii=False, sort_keys=True).encode()).hexdigest()

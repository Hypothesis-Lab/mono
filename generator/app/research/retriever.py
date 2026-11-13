from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any


class ResearchRetriever:
    def __init__(self, cards_path: Path | None = None, *, max_cards: int = 5) -> None:
        if max_cards < 1:
            raise ValueError("max_cards must be positive")

        path = (
            cards_path or Path(__file__).resolve().parents[3] / "datasets" / "audience_sources" / "research_cards.json"
        )
        payload = json.loads(path.read_text(encoding="utf-8"))
        cards = payload.get("cards")
        if not isinstance(cards, list) or not cards:
            raise ValueError(f"research cards file has no cards: {path}")

        self._cards = tuple(card for card in cards if isinstance(card, dict) and card.get("source_id"))
        self._max_cards = max_cards
        self.cards_path = path

    def retrieve(
        self,
        product_passport: dict[str, Any],
        segments: list[dict[str, Any]],
        limitations: list[str],
    ) -> list[dict[str, Any]]:
        query = " ".join(
            (
                json.dumps(product_passport, ensure_ascii=False),
                json.dumps(segments, ensure_ascii=False),
                json.dumps(limitations, ensure_ascii=False),
            )
        )
        query_tokens = _tokens(query)
        ranked = []
        for card in self._cards:
            card_tokens = _tokens(" ".join(str(card.get(key, "")) for key in ("title", "keywords", "use_for")))
            score = len(query_tokens & card_tokens)
            ranked.append((score, str(card["source_id"]), card))

        ranked.sort(key=lambda item: (-item[0], item[1]))
        return [
            {
                **card,
                "retrieval_score": score,
            }
            for score, _, card in ranked[: self._max_cards]
        ]


def _tokens(value: str) -> set[str]:
    return set(re.findall(r"[a-zа-яё0-9]+", value.lower()))

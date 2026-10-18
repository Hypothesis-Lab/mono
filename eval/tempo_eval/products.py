from __future__ import annotations

from pathlib import Path

PRODUCT_SLUGS = ("cafe", "bank")
PRODUCT_NAMES = {
    "cafe": "Tempo",
    "bank": "Lumen",
}


def product_source_dir(product: str) -> Path:
    if product not in PRODUCT_SLUGS:
        choices = ", ".join(PRODUCT_SLUGS)
        raise ValueError(f"unknown product {product!r}; choose one of: {choices}")
    return Path(__file__).resolve().parents[2] / "n8n" / "product" / product


def product_name(product: str) -> str:
    try:
        return PRODUCT_NAMES[product]
    except KeyError as error:
        choices = ", ".join(PRODUCT_SLUGS)
        raise ValueError(f"unknown product {product!r}; choose one of: {choices}") from error

# Audience source datasets

This directory contains the deduplicated public datasets referenced by
`docs_dump/audience_sources/30 источников для сегментации.docx`.

The original archives are retained alongside extracted files. Checksums and
source-to-dataset mappings are in `manifest.json`.

## Included datasets

| Directory | Reference sources | Contents | License/source |
|---|---:|---|---|
| `uci_bank_marketing/` | #06 | Portuguese bank-marketing CSVs; 45,211-row and 41,188-row variants | UCI, CC BY 4.0 |
| `uci_online_retail/` | #22, #25, #30 | `Online Retail.xlsx`, 541,909 transactions | UCI, CC BY 4.0 |
| `uci_online_retail_ii/` | #24, #27 | `online_retail_II.xlsx`, two years of transactions | UCI, CC BY 4.0 |
| `olist_brazilian_ecommerce/` | #26 | Nine CSV tables covering approximately 100,000 orders | Olist/Kaggle, CC BY-NC-SA 4.0 |
| `berka/` | #14 | Eight Czech banking CSV tables, including approximately 1 million transactions | Kaggle listing: CC0; preserve original attribution |

## Not included

The remaining references either use private/proprietary bank, card, mobile,
or retail data, or do not publish a raw-data download. Source #10 explicitly
states that its raw credit-card and mobile-phone data is unavailable for
contractual and privacy reasons. The papers themselves can be collected
separately, but they are not raw datasets.

## Reading notes

- UCI archives are ZIP files containing the original publisher files.
- Olist and Berka archives come from their public Kaggle dataset pages.
- The UCI and Olist licenses should be checked before redistribution or
  commercial use.
- `manifest.json` records the download URLs, archive SHA-256 values, and the
  mapping back to the numbered references.

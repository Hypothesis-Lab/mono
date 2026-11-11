# Audience sources index

This is the index for the 30 sources in
`docs_dump/audience_sources/30 источников для сегментации.docx`.

Paths in this table are relative to `datasets/audience_sources/`.

## Status legend

- **Full public dataset**: the raw dataset described by the source is stored locally.
- **No public raw dataset**: the paper describes data, but the raw data was private, restricted, or not published.
- **Full paper**: a local full-text PDF, HTML, or XML artifact exists.
- **Abstract only**: only an abstract was available without restricted access.
- **Summary page**: an institutional or repository summary page was saved.

## Source index

| # | Source | Publication type | Dataset status | Dataset location | Paper status | Paper location |
|---:|---|---|---|---|---|---|
| 01 | Methods of bank-client segmentation for transfers | HSE thesis/VKR | No public raw dataset | — | Summary page | `01_bank_transfers/papers/01.html` |
| 02 | Bank cashback-loyalty customer segmentation | HSE thesis/VKR | No public raw dataset; private bank data | — | Summary page | `02_bank_cashback/papers/02.html` |
| 03 | Highly-valued bank customers by transaction frequency and amount | Journal article | No public raw dataset | — | Full paper | `03_valued_bank_customers/papers/03.xml` |
| 04 | Customer segmentation with Open Banking data | Conference paper | No public raw dataset identified | — | Abstract only | `04_open_banking/papers/abstract.txt` |
| 05 | Interpretable customer segmentation in financial services | Journal article | No public raw dataset identified | — | Full paper | `05_financial_services/papers/05.pdf` |
| 06 | Bank customer segmentation with improved DBSCAN | Journal article | **Full public dataset** | `06_bank_customer_segmentation/datasets/uci_bank_marketing/` | Full paper | `06_bank_customer_segmentation/papers/06.pdf` |
| 07 | Mobile banking users and feature engineering | Journal article | No public raw dataset identified | — | Full paper | `07_mobile_banking/papers/07.pdf` |
| 08 | BIAT banking segmentation with RFM and CLV | Conference paper | No public raw dataset; private bank data | — | Abstract only | `08_biat_trading_room/papers/abstract.txt` |
| 09 | Credit-card CLV segments and loyalty | Journal article | No public raw dataset; private card data | — | Full paper | `09_credit_card_loyalty/papers/09.pdf` |
| 10 | Purchase sequences and lifestyles | Journal article | No raw data; explicitly unavailable for privacy/contractual reasons | — | Full paper | `10_credit_card_lifestyles/papers/10.xml` |
| 11 | Geospatial activity profiles of bank customers | Journal article | No public raw dataset; private bank data | — | Abstract only | `11_geospatial_bank_activity/papers/abstract.txt` |
| 12 | City consumption profiles | Journal article | No public raw dataset; private financial-group data | — | Full paper | `12_city_consumption/papers/12.pdf` |
| 13 | Banking platform and data-driven marketing | Journal article | No public raw dataset; private bank data | — | Full paper | `13_banking_marketing/papers/13.pdf` |
| 14 | Financial time-series clustering for inclusive credit | arXiv preprint | **Full public dataset** | `14_financial_time_series/datasets/berka/` | Full paper | `14_financial_time_series/papers/14.pdf` |
| 15 | Internet-banking clickstream clustering | Journal article | No public raw dataset; private clickstream data | — | Abstract only | `15_bank_clickstream/papers/abstract.txt` |
| 16 | Temporal fluidity in credit-card personas | Journal article | No public raw dataset; private card data | — | Full paper | `16_credit_card_personas/papers/16.pdf` |
| 17 | Spending communities and carbon emissions | Journal article | No public raw dataset; restricted bank data | — | Full paper | `17_carbon_spending_communities/papers/17.pdf` |
| 18 | UK household carbon-reduction segmentation | Journal article | No public raw dataset; restricted Lloyds data | — | Abstract only | `18_uk_household_carbon/papers/abstract.txt` |
| 19 | Extended RFM segmentation in e-commerce | Journal article | No public raw dataset; private company data | — | Full paper | `19_ecommerce_extended_rfm/papers/19.pdf` |
| 20 | Density-based RFMI segmentation | Journal article | No public raw dataset; private company data | — | Summary page | `20_density_rfm/papers/20.html` |
| 21 | Dynamic LRFMS and time-series clustering | Journal article | No public raw dataset; private company data | — | Full paper | `21_dynamic_auto_parts/papers/21.xml` |
| 22 | Consumer choices with K-Means and fuzzy logic | Journal article | **Full public dataset** | `22_consumer_choices/datasets/uci_online_retail/` | Full paper | `22_consumer_choices/papers/22.pdf` |
| 23 | Longitudinal supermarket segmentation validation | Journal article | No public raw dataset; private supermarket data | — | Full paper | `23_supermarket_longitudinal/papers/23.html` |
| 24 | Monitoring customer-segment dynamics | Journal article | **Full public dataset** | `24_segment_dynamics/datasets/uci_online_retail_ii/` | Full paper | `24_segment_dynamics/papers/24.pdf` |
| 25 | UK retail clustering algorithm comparison | Journal article | **Full public dataset** | `25_uk_retail_clustering/datasets/uci_online_retail/` | Full paper | `25_uk_retail_clustering/papers/25.pdf` |
| 26 | CRM and predictive retention in e-commerce | Journal article | **Full public dataset** | `26_olist_customer_retention/datasets/olist_brazilian_ecommerce/` | Full paper | `26_olist_customer_retention/papers/26.xml` |
| 27 | Graph-based RFM segmentation | Journal article | **Full public dataset** | `27_retail_rfm_graph/datasets/uci_online_retail_ii/` | Full paper | `27_retail_rfm_graph/papers/27.pdf` |
| 28 | Fuzzy-XAI segmentation and risk detection | Journal article | No public raw dataset; private company data | — | Full paper | `28_fuzzy_xai_rfm/papers/28.pdf` |
| 29 | Empirical comparison of RFM clustering approaches | Journal article | No public raw dataset; private company data | — | Full paper | `29_rfm_clustering_comparison/papers/29.pdf` |
| 30 | Deep-learning segmentation for online retail | Journal article | **Full public dataset** | `30_deep_online_retail/datasets/uci_online_retail/` | Full paper | `30_deep_online_retail/papers/30.pdf` |

## Dataset inventory

The eight source references with public raw data reduce to five distinct
datasets because several papers reuse the same benchmark:

1. **UCI Bank Marketing** — source #06; CC BY 4.0.
2. **Berka banking dataset** — source #14; listed as CC0 by the Kaggle publisher.
3. **UCI Online Retail** — sources #22, #25, and #30; CC BY 4.0.
4. **UCI Online Retail II** — sources #24 and #27; CC BY 4.0.
5. **Olist Brazilian E-commerce** — source #26; CC BY-NC-SA 4.0.

Shared datasets are stored once and linked from the other source folders. The
dataset checksums and download URLs are in `manifest.json`; paper access and
artifact details are in `papers_manifest.json`.

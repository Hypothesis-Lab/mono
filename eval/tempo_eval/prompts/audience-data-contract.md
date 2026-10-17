## Trusted event-data contract

This is metadata about an event dataset, not a description of the expected
audience and not a target segment. Use it to interpret fields and choose
queryable signals. Never infer a segment from this contract alone.

- One row is one event. The same private client grouping key may occur on many
  rows; the grouping key is never returned to the model.
- `event_timestamp` is the event time in UTC. Queries must specify a cutoff and
  an observation window. A monthly condition means at least one matching event
  in every calendar month of that window.
- Empty values mean that a property is not applicable to the event or was not
  observed. Empty is not proof of a negative fact.
- `source_system` identifies the source domain of an event, such as
  transactions, acquiring, product analytics, a warehouse snapshot, or
  campaign history.
- `event_type` identifies the event kind, such as expense, incoming payment,
  app visit, campaign impression or response, or balance snapshot.
- `account_types`, `bank_products`, and `account_scope` describe the account
  and banking product associated with an event. `account_scope` distinguishes
  a personal account from a business-account context.
- `transaction_category`, `expense_category`, and `expense_amount_rub`
  describe the operation and its amount when applicable.
- `mcc_code` and `mcc_role` describe the merchant-category code and the role of
  the merchant point in a transaction. MCC meanings come from the supplied
  taxonomy/reference; a code does not prove a person's role, ownership, staff
  count, or product need.
- `expense_count_prior_30d` and `expense_sum_prior_30d_rub` are historical
  aggregates for the preceding 30 days; the current event is excluded.
- `app_section`, `app_visits_prior_30d`, and `days_since_prior_app_visit`
  describe observed application usage. A visit does not prove intent.
- Campaign impressions, responses, campaign identifiers, current-test fields,
  and future events are excluded from audience features to prevent leakage.
- Demographics, estimated income, geography, balances, and contact or identity
  fields are excluded from this pipeline unless a separate trusted policy
  explicitly permits them. Do not derive sensitive traits from proxy behavior.

Transactions alone cannot establish business ownership, employee count, a
scheduling problem, motivation, or willingness to buy. Those are hypotheses or
unknowns requiring separate evidence.

The local analytics tool is the only component that calculates membership,
counts, shares, and month-by-month conditions. The model may propose a query
using catalog-approved fields and observed values, but it must not invent
values or claim that a query has already been executed.

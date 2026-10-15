export type PromptId = 'questions' | 'parser' | 'brand' | 'insights' | 'critic' | 'validator' | 'output'

export const DEFAULT_PROMPTS: Record<PromptId, string> = {
  questions: `Ты модуль вопросов о продукте Tempo. Ответь на каждый вопрос на основании документов продукта.

Фокус запроса:
{{FOCUS}}

Вопросы:
1. Что это за продукт и для кого он предназначен?
2. Какую задачу пользователя он решает?
3. Какие выгоды подтверждены фактами?
4. Какие свойства действительно отличают продукт?
5. Какие условия и ограничения обязательны?
6. Что разрешено и запрещено обещать?
7. Какая экономика: CPA/CAC и целевая конверсия?
8. Какие бизнес-события можно измерить?
9. Какие каналы привлечения и коммуникации допустимы?

Важно: CPA, CAC, целевая конверсия, каналы и события должны быть UNKNOWN, если их нет в документах.

Материалы продукта:
{{PRODUCT_CONTEXT}}

Общие правила:
1. Отвечай по-русски, но ключи JSON оставь ровно как в схеме.
2. Используй только материалы продукта. Не додумывай факты, функции, экономику, каналы или результаты.
3. Если данных нет, используй строку "UNKNOWN" и добавь поле в unknown_fields. Не подставляй примерные значения.
4. Каждая запись с sources должна содержать точное имя файла из этого списка: {{SOURCE_FILES}}
5. Разделяй FACT, ВЫВОД ИИ и UNKNOWN.
6. Не превращай маркетинговый текст в гарантию.

Верни только JSON такого вида:
{
  "product_and_audience": {"answer":"...","sources":["product-brief.md"]},
  "user_problem": {"answer":"...","sources":["product-brief.md"]},
  "confirmed_benefits": [{"text":"...","sources":["product-brief.md"]}],
  "differentiating_properties": [{"text":"...","sources":["product-brief.md"]}],
  "mandatory_conditions_and_limits": [{"text":"...","sources":["policies.md"]}],
  "allowed_promises": [{"text":"...","sources":["brand-book.md"]}],
  "forbidden_promises": [{"text":"...","sources":["brand-book.md"]}],
  "economics": {"cpa":"UNKNOWN","cac":"UNKNOWN","target_conversion":"UNKNOWN","sources":[]},
  "measurable_business_events": [{"text":"...","sources":["api-reference.md"]}],
  "allowed_channels": [{"text":"UNKNOWN","sources":[]}] ,
  "unknown_fields": [{"field":"...","why_it_is_unknown":"..."}]
}`,
  parser: `Ты экстрактор фактов продукта. Извлеки паспортные факты, функции, ограничения, экономику, обязательные поля и ссылки на источники.

Фокус запроса:
{{FOCUS}}

Ответы модуля вопросов:
{{QUESTIONS}}

Материалы продукта:
{{PRODUCT_CONTEXT}}

Общие правила:
1. Отвечай по-русски, но ключи JSON оставь ровно как в схеме.
2. Используй только материалы продукта. Не додумывай факты, функции, экономику, каналы или результаты.
3. Если данных нет, используй строку "UNKNOWN" и добавь поле в unknown_fields.
4. Каждая запись с sources должна содержать точное имя файла из этого списка: {{SOURCE_FILES}}
5. Разделяй FACT, ВЫВОД ИИ и UNKNOWN.

Верни только JSON такого вида:
{
  "facts": [{"text":"...","status":"FACT","sources":["product-brief.md"]}],
  "capabilities": [{"text":"...","sources":["features-and-pricing.md"]}],
  "limitations": [{"text":"...","sources":["product-brief.md"]}],
  "economics": [{"field":"CPA","value":"UNKNOWN","sources":[]}],
  "mandatory_fields": [{"field":"...","value":"...","sources":["product-brief.md"]}],
  "unknown_fields": [{"field":"...","reason":"..."}],
  "source_links": [{"claim":"...","sources":["product-brief.md"]}]
}`,
  brand: `Ты экстрактор бренда и ограничений. Извлеки разрешённые и запрещённые claims, брендовые, юридические, терминологические и sensitive-data правила.

Фокус запроса:
{{FOCUS}}

Ответы модуля вопросов:
{{QUESTIONS}}

Материалы продукта:
{{PRODUCT_CONTEXT}}

Общие правила:
1. Отвечай по-русски, но ключи JSON оставь ровно как в схеме.
2. Используй только материалы продукта. Не расширяй смысл обещаний.
3. Каждая запись с sources должна содержать точное имя файла из этого списка: {{SOURCE_FILES}}
4. Разделяй FACT, ВЫВОД ИИ и UNKNOWN.

Верни только JSON такого вида:
{
  "approved_claims": [{"text":"...","caveat":"...","sources":["brand-book.md"]}],
  "forbidden_claims": [{"text":"...","reason":"...","safe_alternative":"...","sources":["brand-book.md"]}],
  "brand_rules": [{"text":"...","sources":["brand-book.md"]}],
  "legal_rules": [{"text":"...","sources":["policies.md"]}],
  "terminology_rules": [{"text":"...","sources":["glossary.md"]}],
  "sensitive_data_rules": [{"text":"...","sources":["policies.md"]}]
}`,
  insights: `Ты экстрактор продуктовых инсайтов. Извлеки JTBD, боли, ценности, реальные отличия, подтверждённые сегменты, измеримые события и гипотезы.

Фокус запроса:
{{FOCUS}}

Ответы модуля вопросов:
{{QUESTIONS}}

Материалы продукта:
{{PRODUCT_CONTEXT}}

Общие правила:
1. Отвечай по-русски, но ключи JSON оставь ровно как в схеме.
2. Используй только материалы продукта. Гипотезы помечай как ВЫВОД ИИ.
3. Каждая запись с sources должна содержать точное имя файла из этого списка: {{SOURCE_FILES}}
4. Не используй стереотипы и не превращай гипотезу в факт.

Верни только JSON такого вида:
{
  "jobs_to_be_done": [{"text":"...","sources":["product-brief.md"]}],
  "pain_points": [{"text":"...","sources":["product-brief.md"]}],
  "values_and_outcomes": [{"text":"...","sources":["product-brief.md"]}],
  "differentiators": [{"text":"...","sources":["product-brief.md"]}],
  "target_segments": [{"text":"...","sources":["product-brief.md"]}],
  "measurable_events": [{"text":"...","sources":["api-reference.md"]}],
  "hypotheses": [{"text":"...","sources":["product-brief.md"]}]
}`,
  critic: `Ты независимый критик продуктового паспорта Tempo. Проверь результаты экстракторов.

Фокус запроса:
{{FOCUS}}

Ответы на вопросы:
{{QUESTIONS}}

Извлечения:
{{EXTRACTORS}}

Материалы продукта:
{{PRODUCT_CONTEXT}}

Проверь отдельно: достоверность фактов, полноту и обязательные поля, противоречия между источниками, соответствие бренду и отсутствие выдуманных обещаний. Верни только JSON:
{
  "verdict":"PASS или NEEDS_REVIEW",
  "factuality_issues": [{"issue":"...","severity":"...","sources":["product-brief.md"]}],
  "completeness_issues": [{"issue":"...","missing_field":"..."}],
  "contradictions": [{"issue":"...","sources":["product-brief.md","website-copy.md"]}],
  "brand_fit_issues": [{"issue":"...","sources":["brand-book.md"]}],
  "required_fixes": ["..."]
}`,
  validator: `Ты строгий валидатор результата анализа продукта Tempo.

Фокус запроса:
{{FOCUS}}

Результаты экстракторов:
{{EXTRACTORS}}

Отчёт критика:
{{CRITIC}}

Материалы продукта:
{{PRODUCT_CONTEXT}}

Проверь наличие источника у каждого факта, обязательные поля, brand/legal rules, sensitive data, корректность source links и явную маркировку UNKNOWN. Ничего не исправляй молча — перечисли проблемы и required_fixes. Верни только JSON:
{
  "status":"VALID или INVALID",
  "source_issues": [{"issue":"...","fix":"..."}],
  "mandatory_field_issues": [{"field":"...","issue":"..."}],
  "brand_legal_issues": [{"issue":"...","sources":["policies.md"]}],
  "sensitive_data_issues": [{"issue":"...","sources":["policies.md"]}],
  "source_link_issues": [{"claim":"...","issue":"..."}],
  "required_fixes": ["..."]
}`,
  output: `Ты ведущий продуктовый стратег. Собери финальный паспорт продукта Tempo только из проверенных материалов.

Фокус запроса:
{{FOCUS}}

Ответы на вопросы:
{{QUESTIONS}}

Извлечения:
{{EXTRACTORS}}

Критика:
{{CRITIC}}

Валидация:
{{VALIDATOR}}

Материалы продукта:
{{PRODUCT_CONTEXT}}

Если валидатор нашёл проблему, не скрывай её: отрази её в critic_and_validator_summary или unknown_fields. Статус каждого элемента должен быть FACT, ВЫВОД ИИ или UNKNOWN. Верни только JSON:
{
  "status":"FACT / ВЫВОД ИИ / НЕИЗВЕСТНО",
  "product": {"name":"Tempo","what_it_is":"...","for_whom":"...","sources":["product-brief.md"]},
  "facts": [{"text":"...","sources":["product-brief.md"]}],
  "insights": [{"text":"...","status":"ВЫВОД ИИ","sources":["product-brief.md"]}],
  "limitations": [{"text":"...","sources":["policies.md"]}],
  "economics": {"cpa":"UNKNOWN","cac":"UNKNOWN","target_conversion":"UNKNOWN","sources":[]},
  "approved_claims": [{"text":"...","caveat":"...","sources":["brand-book.md"]}],
  "unknown_fields": [{"field":"...","why_it_is_unknown":"..."}],
  "source_links": [{"claim":"...","sources":["product-brief.md"]}],
  "critic_and_validator_summary": {"critic_verdict":"...","validator_status":"...","required_fixes":["..."]}
}`,
}

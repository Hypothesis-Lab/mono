%pip install -q -U openai
%pip install -q "pydantic>=2.7,<3" json-repair

from openai import OpenAI
from google.colab import userdata

RUN_LLM = True
MODEL_NAME = "gpt-5.6-luna"
REASONING_EFFORT = "low"

client = (
    OpenAI(
        api_key=userdata.get("OPENAI_API_KEY"),
        timeout=120.0,
        max_retries=2,
    )
    if RUN_LLM else None
)

import copy
import hashlib
import json
import math
import re
from datetime import datetime, timezone
from pathlib import Path
from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field, ValidationError, model_validator

PIPELINE_VERSION = "5.0_pain_need_grounded"
Text = Annotated[str, Field(min_length=1, max_length=1600)]


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


class ProductFact(StrictModel):
    fact_id: Text
    text: Text


class ProductPrice(StrictModel):
    # Для кредита цена — ставка/ПСК/комиссии, а не размер кредитного лимита.
    status: Literal["unknown", "known"] = "unknown"
    description: Text = "Стоимость не указана; не считать продукт бесплатным."
    source_fact_ids: list[str] = Field(default_factory=list)

    @model_validator(mode="after")
    def known_price_has_source(self):
        if self.status == "known" and not self.source_fact_ids:
            raise ValueError("Известную цену нужно подтвердить source_fact_ids.")
        return self


class EligibilityRule(StrictModel):
    rule_id: str
    field: str
    operator: Literal["eq", "in", "gte", "lte", "between", "truthy"]
    value: str | int | bool | list[str] | list[int]
    description: str


class Product(StrictModel):
    brand: Text
    name: Text
    description: Text
    category: Text
    geography: Text
    price: ProductPrice = Field(default_factory=ProductPrice)
    keywords: list[str] = Field(default_factory=list)  # Тематика, НЕ факты.
    key_fact_ids: list[str] = Field(default_factory=list)
    source_url: str | None = None
    facts_checked_at: str | None = None
    facts: list[ProductFact] = Field(min_length=1)
    eligibility_rules: list[EligibilityRule] = Field(default_factory=list)
    forbidden_claims: list[str] = Field(default_factory=list)
    known_constraints: list[str] = Field(default_factory=list)
    unknowns: list[str] = Field(default_factory=list)

    @model_validator(mode="after")
    def product_references(self):
        fact_ids = [f.fact_id for f in self.facts]
        if len(set(fact_ids)) != len(fact_ids):
            raise ValueError("Дубли fact_id в продукте.")
        for ref in self.key_fact_ids + self.price.source_fact_ids:
            if ref not in fact_ids:
                raise ValueError(f"Неизвестный fact_id: {ref}")
        return self


class ExperimentConfig(StrictModel):
    business_goal: str = ""
    goal_origin: Literal["demo_assumption", "user_provided"] = "user_provided"
    target_action: str = ""
    primary_metric: str = ""
    metric_direction: Literal["increase", "decrease"] = "increase"
    metric_numerator: str = ""
    metric_denominator: str = ""
    # Порог результата теста пользователь определяет ПОСЛЕ baseline/power analysis.
    success_criterion: str | None = None


class CampaignConfig(StrictModel):
    allowed_channels: list[str] = Field(default_factory=list)
    contactable_persona_ids: list[str] = Field(default_factory=list)
    note: str = "Каналы не используются для интервью или ранжирования."


class InputBundle(StrictModel):
    product: Product
    experiment: ExperimentConfig
    campaign: CampaignConfig = Field(default_factory=CampaignConfig)


class InterviewQuestion(StrictModel):
    question_id: str
    dimension: str
    text: str


class InterviewAnswer(StrictModel):
    question_id: str
    answer: Text


class AnswerBatch(StrictModel):
    answers: list[InterviewAnswer] = Field(min_length=5, max_length=5)


class EvidenceQuote(StrictModel):
    question_id: str
    quote: Text


class InsightDraft(StrictModel):
    text: Text
    present: bool  # False = явно сказано, что такой боли/потребности нет.
    evidence: list[EvidenceQuote] = Field(min_length=1, max_length=2)


class InsightBatch(StrictModel):
    # Не заставляем придумывать вторую боль, если основание только одно.
    pains: list[InsightDraft] = Field(min_length=1, max_length=3)
    needs: list[InsightDraft] = Field(min_length=1, max_length=3)
    desires: list[InsightDraft] = Field(default_factory=list, max_length=3)
    objections: list[InsightDraft] = Field(default_factory=list, max_length=3)
    triggers: list[InsightDraft] = Field(default_factory=list, max_length=3)
    use_cases: list[InsightDraft] = Field(default_factory=list, max_length=3)


class ResearchRecord(StrictModel):
    persona_id: str
    input_signature: str
    answers: list[InterviewAnswer] = Field(min_length=10, max_length=10)
    insights: InsightBatch
    synthetic: Literal[True] = True


class MetricVote(StrictModel):
    value: int = Field(ge=0, le=4, strict=True)
    reason: Text
    evidence_ids: list[str] = Field(min_length=1, max_length=5)
    fact_ids: list[str] = Field(default_factory=list, max_length=3)


METRIC_WEIGHTS = {
    "need_strength": 0.30,
    "product_value_fit": 0.35,
    "adoption_readiness": 0.15,
    "digital_access_fit": 0.10,
    "hypothesis_testability": 0.10,
}
MIN_TOTAL_SCORE = 55.0  # Экспертный порог MVP, НЕ эмпирическая калибровка.
MIN_NEED_LEVEL = 2
MIN_FIT_LEVEL = 2
TOP_K = 3


class ScoreDraft(StrictModel):
    need_strength: MetricVote
    product_value_fit: MetricVote
    adoption_readiness: MetricVote
    digital_access_fit: MetricVote
    hypothesis_testability: MetricVote
    main_barrier: Text
    unknowns: list[str] = Field(default_factory=list, max_length=5)


class ScoreRecord(StrictModel):
    persona_id: str
    input_signature: str
    research_signature: str
    scores: ScoreDraft


class HypothesisDraft(StrictModel):
    # Identity/ранг/insight_basis выставляет Python; LLM не может сменить аудиторию.
    pain_id: str
    need_id: str
    supporting_insight_ids: list[str] = Field(default_factory=list, max_length=2)
    selected_fact_ids: list[str] = Field(min_length=1, max_length=3)
    headline: Text
    offer_value: Text
    change_to_test: Text
    mechanism: Text
    why_for_persona: Text
    channel: str | None
    risks: list[str] = Field(min_length=1, max_length=3)


def jdump(value):
    if isinstance(value, BaseModel):
        value = value.model_dump(mode="json")
    return json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2)


def fingerprint(value):
    return hashlib.sha256(jdump(value).encode("utf-8")).hexdigest()


def norm(text):
    return re.sub(r"\s+", " ", text.lower()).strip()


def unique_by_id(items, label):
    ids = [item.persona_id for item in items]
    if len(ids) != len(set(ids)):
        raise ValueError(f"{label}: дубли persona_id.")
    return dict(zip(ids, items))



class SyntheticPersona(StrictModel):
    persona_id: str
    label: str
    gender: Literal["женщина", "мужчина"]
    age: int = Field(ge=18, le=80)
    monthly_income_rub: int = Field(gt=0)
    occupation: str
    employment_type: str
    citizenship: str
    permanent_registration_country: str
    device: str
    operating_system: str
    digital_behavior: str
    interests: list[str] = Field(min_length=2)
    geography: str
    cifra_relationship: str
    has_cifra_debit_or_salary_card: bool
    financial_behavior: str
    credit_attitude: str


CALL_LOG = []
CACHE_DIR = Path("/content/hypothesis_lab_v5/cache")


def split_json_object_fragments(text):
    # Кавычки в сопроводительном тексте не должны скрыть первый объект.
    fragments, start, depth, in_string, escaped = [], None, 0, False, False
    for i, char in enumerate(text):
        if start is None:
            if char == "{":
                start, depth = i, 1
            continue
        if in_string:
            if escaped:
                escaped = False
            elif char == "\\":
                escaped = True
            elif char == '"':
                in_string = False
        elif char == '"':
            in_string = True
        elif char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                fragments.append(text[start:i + 1])
                start = None
    # Оборванный объект не достраиваем: просим модель заново.
    return fragments


def extract_json_candidates(text):
    text = re.sub(r"<think>.*?</think>", "", text, flags=re.DOTALL)
    candidates = []
    for fragment in split_json_object_fragments(text):
        try:
            obj = json.loads(fragment)
        except json.JSONDecodeError:
            from json_repair import repair_json
            obj = repair_json(fragment, return_objects=True)
        if isinstance(obj, dict) and not ({"$defs", "properties", "$schema"} & set(obj)):
            candidates.append(obj)
    return candidates


def compact_error(exc):
    if isinstance(exc, ValidationError):
        return "; ".join(
            f"{'.'.join(map(str, x['loc']))}: {x['msg']}"
            for x in exc.errors(include_url=False, include_input=False)
        )[:1400]
    return str(exc)[:1400]


def llm_runtime_signature():
    return {
        "provider": "openai",
        "model": MODEL_NAME,
        "reasoning_effort": REASONING_EFFORT,
        "adapter": "responses.parse.v1",
    }


def run_llm_json(
    system_prompt,
    user_prompt,
    response_model,
    *,
    validator=None,
    max_attempts=5,
    temperature=0.0,
    max_new_tokens=1800,
    retry_feedback=None,
    use_cache=True,
):
    if client is None:
        raise RuntimeError("Сначала выполните пункт 1 с RUN_LLM=True.")

    # Аргумент оставлен для совместимости с вызовами в ноутбуке.
    # temperature в API не передаем.
    key = fingerprint({
        "runtime": llm_runtime_signature(),
        "pipeline": PIPELINE_VERSION,
        "system": system_prompt,
        "user": user_prompt,
        "schema": response_model.model_json_schema(),
        "feedback": retry_feedback,
        "max_new_tokens": max_new_tokens,
    })
    cache_file = CACHE_DIR / f"{key}.json"

    if use_cache and cache_file.exists():
        try:
            result = response_model.model_validate_json(
                cache_file.read_text(encoding="utf-8")
            )
            if validator:
                validator(result)

            CALL_LOG.append({
                "key": key,
                "cached": True,
                "schema": response_model.__name__,
            })
            return result
        except (ValidationError, ValueError, KeyError):
            pass

    feedback = retry_feedback or ""
    last_error = None

    for attempt in range(1, max_attempts + 1):
        prompt = user_prompt + "\nВерни JSON с данными по заданной схеме."
        if feedback:
            prompt += "\nИсправь ошибку и верни весь объект: " + feedback

        try:
            response = client.responses.parse(
                model=MODEL_NAME,
                reasoning={"effort": REASONING_EFFORT},
                input=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": prompt},
                ],
                text_format=response_model,
                # Лимит включает рассуждение модели и итоговый JSON.
                max_output_tokens=min(
                    32768, max(8192, max_new_tokens * 4) * attempt
                ),
                store=False,
            )

            if (
                response.status == "incomplete"
                and getattr(
                    response.incomplete_details, "reason", None
                ) == "max_output_tokens"
            ):
                raise ValueError("Ответ оборван по лимиту токенов.")

            if response.status != "completed":
                raise RuntimeError(
                    f"API не завершил ответ: {response.status}"
                )

            if any(
                getattr(content, "type", None) == "refusal"
                for item in response.output
                for content in getattr(item, "content", [])
            ):
                raise RuntimeError(
                    "Модель отказалась отвечать на этот запрос."
                )

            result = response.output_parsed
            if result is None:
                raise ValueError("API не вернул объект нужной структуры.")

            # Сохраняем проверки персон, цитат, болей и потребностей.
            if validator:
                validator(result)

            if use_cache:
                CACHE_DIR.mkdir(parents=True, exist_ok=True)
                cache_file.write_text(
                    result.model_dump_json(indent=2),
                    encoding="utf-8",
                )

            CALL_LOG.append({
                "key": key,
                "cached": False,
                "attempts": attempt,
                "schema": response_model.__name__,
                "model": response.model,
                "reasoning_effort": REASONING_EFFORT,
                "usage": (
                    response.usage.model_dump()
                    if response.usage else None
                ),
            })
            return result

        except (ValidationError, ValueError, KeyError) as exc:
            last_error = exc
            feedback = compact_error(exc)
            print(f"Попытка {attempt}/{max_attempts}: {feedback}")

    raise RuntimeError(
        f"Нет валидного результата после {max_attempts} попыток: "
        f"{last_error}"
    )

def product_for_research(product):
    # Ни каналы, ни техническая цель, ни ключевые слова не доказывают ценность.
    fields = {"brand", "name", "description", "category", "geography", "price",
              "facts", "key_fact_ids", "known_constraints", "unknowns"}
    return product.model_dump(include=fields)


def validate_product_rules(product, personas):
    known = set(type(personas[0]).model_fields) if personas else set()
    for rule in product.eligibility_rules:
        if rule.field not in known:
            raise ValueError(f"{rule.rule_id}: неизвестное поле персоны {rule.field}")
        if rule.operator in {"in", "between"} and not isinstance(rule.value, list):
            raise ValueError(f"{rule.rule_id}: ожидается список value")
        if rule.operator == "between":
            if (len(rule.value) != 2 or
                not all(type(x) in {int, float} for x in rule.value) or
                rule.value[0] > rule.value[1]):
                raise ValueError(f"{rule.rule_id}: неверные границы")


def readiness_questions(bundle):
    p, e = bundle.product, bundle.experiment
    questions = []
    if len(p.facts) < 3:
        questions.append("Добавьте минимум три подтвержденных продуктовых факта.")
    if not p.key_fact_ids:
        questions.append("Какие fact_id являются ключевыми характеристиками продукта?")
    if not e.business_goal.strip():
        questions.append("Какого бизнес-результата хотите достичь? Не пишите 'выбрать топ-3'.")
    if re.search(r"выбрать.*(?:три|3).*персон|сформировать.*гипотез", e.business_goal.lower()):
        questions.append("Замените техническую задачу алгоритма на бизнес-цель эксперимента.")
    for field, question in {
        "target_action": "Какое действие должен совершить клиент?",
        "primary_metric": "Как называется основная метрика эксперимента?",
        "metric_numerator": "Что является числителем метрики?",
        "metric_denominator": "Что является знаменателем метрики?",
    }.items():
        if not getattr(e, field).strip():
            questions.append(question)
    warnings = list(p.unknowns)
    if p.price.status == "unknown":
        warnings.append("Цена неизвестна: не обещать бесплатность и экономию; перед тестом уточнить тарифы.")
    if not bundle.campaign.allowed_channels:
        warnings.append("Каналы пока не заданы. Исследование продолжится, в гипотезах channel=null.")
    if not e.success_criterion:
        warnings.append("Порог успеха теста не задан: определить после baseline и расчета выборки.")
    return questions, warnings


def load_input_bundle(use_demo, demo):
    if use_demo:
        return demo.model_copy(deep=True)
    from google.colab import files
    uploaded = files.upload()
    if len(uploaded) != 1:
        raise ValueError("Загрузите ровно один JSON с product, experiment и campaign.")
    data = json.loads(next(iter(uploaded.values())).decode("utf-8-sig"))
    if "product" not in data:
        raise ValueError("Старый v4 JSON: перенесите продукт в product, бизнес-цель в experiment, каналы в campaign. См. демо.")
    return InputBundle.model_validate(data)


DEMO_PRODUCT = Product(brand='Цифра банк', name='Комбо Цифра карта', description='Дебетовая и кредитная карта в одной: кредитный лимит подключается к дебетовой или зарплатной карте Цифра банка через мобильное приложение.', category='banking_card', geography='Россия', source_url='https://cifra-bank.ru/combo/', facts=[ProductFact(fact_id='F1', text='Комбо Цифра карта совмещает дебетовую и кредитную карту в одной карте.'), ProductFact(fact_id='F2', text='Кредитный лимит можно подключить к дебетовой или зарплатной карте Цифра банка через мобильное приложение.'), ProductFact(fact_id='F3', text='Беспроцентный период длится 55 дней при своевременной уплате обязательных платежей и полном погашении долга до его окончания.'), ProductFact(fact_id='F4', text='В заявлении клиент выбирает желаемый кредитный лимит от 30 000 до 100 000 рублей; решение принимает банк.'), ProductFact(fact_id='F5', text='Минимальный платеж составляет 5% от суммы основного долга на конец расчетного периода, но не менее 300 рублей.'), ProductFact(fact_id='F6', text='Заявление и настройка лимита выполняются в мобильном приложении Цифра банка.'), ProductFact(fact_id='F7', text='Кредитный лимит используется при отсутствии или недостаточности собственных средств на счетах, привязанных к карте.')], eligibility_rules=[EligibilityRule(rule_id='E1', field='citizenship', operator='eq', value='Россия', description='Гражданство РФ'), EligibilityRule(rule_id='E2', field='permanent_registration_country', operator='eq', value='Россия', description='Постоянная регистрация на территории РФ'), EligibilityRule(rule_id='E3', field='age', operator='between', value=[21, 70], description='Возраст от 21 до 70 лет на дату возврата кредита'), EligibilityRule(rule_id='E4', field='employment_type', operator='eq', value='постоянная работа', description='Постоянное основное место работы'), EligibilityRule(rule_id='E5', field='operating_system', operator='in', value=['iOS', 'Android'], description='Смартфон с поддерживаемым приложением на iOS или Android')], forbidden_claims=['гарантированное одобрение', 'одобрение для всех', 'без проверки', 'без обязательных платежей', 'без переплаты при любых условиях', 'бесплатные деньги'], known_constraints=['Для подключения лимита нужна дебетовая или зарплатная карта Цифра банка.', 'Льготный период требует своевременных обязательных платежей и полного погашения.', 'Окончательное решение по кредитному лимиту принимает банк.'], price=ProductPrice(status='unknown', description='Ставка, ПСК, комиссии и стоимость обслуживания во входном описании не указаны.'), keywords=['банковская карта', 'кредитный лимит', 'резерв на краткосрочные расходы'], key_fact_ids=['F2', 'F3', 'F7'], facts_checked_at=None, unknowns=['Демо-факты перенесены из приложенного файла; их актуальность автоматически не проверена.', 'Не указаны ставка, ПСК, комиссии, исключения и полный действующий тариф.', 'Полные требования банка и возраст на дату погашения требуют отдельной проверки.'])

# Это ограничение аудитории ДЕМО-ЭКСПЕРИМЕНТА для прямого подключения лимита,
# а не новое утверждение об отказе банка остальным клиентам.
DEMO_PRODUCT.eligibility_rules.append(EligibilityRule(
    rule_id="EXP_EXISTING_CARD", field="has_cifra_debit_or_salary_card", operator="eq", value=True,
    description="Сегмент демо: действующие владельцы дебетовой/зарплатной карты Цифра банка"))

DEMO_INPUT = InputBundle(
    product=DEMO_PRODUCT,
    experiment=ExperimentConfig(
        business_goal="Повысить долю действующих держателей карты, подающих заявление на подключение кредитного лимита после знакомства с предложением.",
        goal_origin="demo_assumption",  # Рабочий пример, НЕ цель, подтвержденная пользователем.
        target_action="Подать заявление на подключение кредитного лимита в приложении",
        primary_metric="application_submit_rate", metric_direction="increase",
        metric_numerator="Число уникальных клиентов группы, отправивших заявление за заранее установленное окно наблюдения",
        metric_denominator="Число уникальных клиентов, рандомизированных в эту группу эксперимента",
        success_criterion=None),
    campaign=CampaignConfig(
        allowed_channels=["in_app_banner", "push", "email", "landing", "sms"],
        contactable_persona_ids=[],  # Не считать наличие карты согласием на рекламные сообщения.
        note="Используется только после отбора аудитории. Доступность канала и реальные согласия проверяются перед тестом."))

# Для своего продукта выберите False и загрузите один JSON с тремя секциями.
USE_DEMO_PRODUCT = True
BUNDLE = load_input_bundle(USE_DEMO_PRODUCT, DEMO_INPUT)
questions_to_user, input_warnings = readiness_questions(BUNDLE)
if questions_to_user:
    raise ValueError("Заполните ввод до запуска:\n- " + "\n- ".join(questions_to_user))
print("Ввод готов к синтетическому исследованию.")
print("Происхождение цели:", BUNDLE.experiment.goal_origin)
print("Стоимость:", BUNDLE.product.price.model_dump())
for warning in input_warnings:
    print("Требует внимания:", warning)


PERSONAS = [
    SyntheticPersona(
        persona_id="persona_001",
        label="Мобильная специалистка с регулярными расходами",
        gender="женщина", age=27, monthly_income_rub=95_000,
        occupation="маркетолог", employment_type="постоянная работа",
        citizenship="Россия", permanent_registration_country="Россия",
        device="iPhone 14", operating_system="iOS",
        digital_behavior="почти все финансовые операции делает в приложениях",
        interests=["фитнес", "сериалы", "путешествия"], geography="Москва",
        cifra_relationship="есть дебетовая карта Цифра банка",
        has_cifra_debit_or_salary_card=True,
        financial_behavior="ведет бюджет, но иногда переносит крупные покупки до зарплаты",
        credit_attitude="готова к короткому льготному кредитованию, боится скрытых условий",
    ),
    SyntheticPersona(
        persona_id="persona_002",
        label="Автомобилист — IT-аналитик",
        gender="мужчина", age=35, monthly_income_rub=180_000,
        occupation="системный аналитик", employment_type="постоянная работа",
        citizenship="Россия", permanent_registration_country="Россия",
        device="Samsung Galaxy S24", operating_system="Android",
        digital_behavior="активно использует банковские приложения и уведомления",
        interests=["автомобиль", "технологии", "инвестиции", "фитнес"], geography="Казань",
        cifra_relationship="зарплатная карта Цифра банка",
        has_cifra_debit_or_salary_card=True,
        financial_behavior="доход стабильный, но расходы на автомобиль возникают неравномерно",
        credit_attitude="использует кредит только при понятном сроке и полном контроле долга",
    ),
    SyntheticPersona(
        persona_id="persona_003",
        label="Семейная бухгалтер с консервативным бюджетом",
        gender="женщина", age=42, monthly_income_rub=130_000,
        occupation="бухгалтер", employment_type="постоянная работа",
        citizenship="Россия", permanent_registration_country="Россия",
        device="Xiaomi Redmi Note 13", operating_system="Android",
        digital_behavior="пользуется приложениями, но перепроверяет тарифы и документы",
        interests=["семья", "дом", "фитнес", "телесериалы"], geography="Екатеринбург",
        cifra_relationship="не является клиентом Цифра банка",
        has_cifra_debit_or_salary_card=False,
        financial_behavior="планирует семейные расходы и держит небольшой резерв",
        credit_attitude="осторожна, опасается комиссий и сложных правил льготного периода",
    ),
    SyntheticPersona(
        persona_id="persona_004",
        label="Региональный менеджер в разъездах",
        gender="мужчина", age=51, monthly_income_rub=155_000,
        occupation="региональный менеджер по продажам", employment_type="постоянная работа",
        citizenship="Россия", permanent_registration_country="Россия",
        device="iPhone 13", operating_system="iOS",
        digital_behavior="часто платит телефоном и решает вопросы банка дистанционно",
        interests=["автомобиль", "телевизор", "футбол", "поездки"], geography="Нижний Новгород",
        cifra_relationship="есть дебетовая карта Цифра банка",
        has_cifra_debit_or_salary_card=True,
        financial_behavior="бывают кассовые разрывы между командировочными расходами и компенсацией",
        credit_attitude="рассматривает лимит как резерв, не хочет носить вторую карту",
    ),
    SyntheticPersona(
        persona_id="persona_005",
        label="Работающая преподавательница старшего возраста",
        gender="женщина", age=63, monthly_income_rub=75_000,
        occupation="преподаватель колледжа", employment_type="постоянная работа",
        citizenship="Россия", permanent_registration_country="Россия",
        device="Realme C55", operating_system="Android",
        digital_behavior="использует базовые функции банка, сложные настройки вызывают вопросы",
        interests=["телевизор", "сад", "здоровье", "пешие прогулки"], geography="Самара",
        cifra_relationship="зарплатная карта Цифра банка",
        has_cifra_debit_or_salary_card=True,
        financial_behavior="предпочитает заранее накопить, но иногда помогает семье до зарплаты",
        credit_attitude="к кредиту относится настороженно и нуждается в простом объяснении",
    ),
    SyntheticPersona(
        persona_id="persona_006",
        label="Молодой специалист и геймер",
        gender="мужчина", age=23, monthly_income_rub=85_000,
        occupation="специалист технической поддержки", employment_type="постоянная работа",
        citizenship="Россия", permanent_registration_country="Россия",
        device="Google Pixel 8", operating_system="Android",
        digital_behavior="быстро осваивает приложения, предпочитает короткие интерфейсы",
        interests=["игры", "гаджеты", "фитнес", "каршеринг"], geography="Новосибирск",
        cifra_relationship="не является клиентом Цифра банка",
        has_cifra_debit_or_salary_card=False,
        financial_behavior="регулярные расходы контролирует, крупные покупки планирует не всегда",
        credit_attitude="интересуется гибкостью, но не хочет формировать привычку жить в долг",
    ),
    SyntheticPersona(
        persona_id="persona_007",
        label="Руководительница digital-команды",
        gender="женщина", age=34, monthly_income_rub=290_000,
        occupation="руководитель digital-команды", employment_type="постоянная работа",
        citizenship="Россия", permanent_registration_country="Россия",
        device="iPhone 16 Pro", operating_system="iOS",
        digital_behavior="ожидает полностью цифровой путь и прозрачные уведомления",
        interests=["фитнес", "путешествия", "инвестиции", "технологии"], geography="Санкт-Петербург",
        cifra_relationship="есть дебетовая карта Цифра банка",
        has_cifra_debit_or_salary_card=True,
        financial_behavior="есть резерв, кредитные продукты использует для управления ликвидностью",
        credit_attitude="готова пользоваться при высокой прозрачности, но небольшой лимит может быть нерелевантен",
    ),
    SyntheticPersona(
        persona_id="persona_008",
        label="Инженер с семейным бюджетом",
        gender="мужчина", age=46, monthly_income_rub=145_000,
        occupation="инженер производственного предприятия", employment_type="постоянная работа",
        citizenship="Россия", permanent_registration_country="Россия",
        device="Honor 200", operating_system="Android",
        digital_behavior="регулярно использует зарплатное банковское приложение",
        interests=["автомобиль", "ремонт дома", "хоккей", "телевизор"], geography="Челябинск",
        cifra_relationship="зарплатная карта Цифра банка",
        has_cifra_debit_or_salary_card=True,
        financial_behavior="доход предсказуемый, но ремонт автомобиля или дома создает разовые пики расходов",
        credit_attitude="готов держать кредитный лимит как запас, если правила погашения понятны",
    ),
]


import pandas as pd
from IPython.display import display
assert len(PERSONAS) == 8
unique_by_id(PERSONAS, "Персоны")
validate_product_rules(BUNDLE.product, PERSONAS)
display(pd.DataFrame([p.model_dump(include={"persona_id", "label", "gender", "age", "monthly_income_rub", "occupation", "device", "interests"}) for p in PERSONAS]))


INTERVIEW_QUESTIONS = [
    InterviewQuestion(question_id="Q01", dimension="problem", text="В каких ситуациях вам приходится решать задачи в этой продуктовой категории и что вы делаете сейчас?"),
    InterviewQuestion(question_id="Q02", dimension="pain", text="Что в текущем решении неудобно и к каким последствиям приводит? Если проблемы нет, так и ответьте."),
    InterviewQuestion(question_id="Q03", dimension="need", text="Что вам необходимо изменить, чтобы решать эту задачу удобнее? Опишите потребность, а не функцию конкретного продукта."),
    InterviewQuestion(question_id="Q04", dimension="desire", text="Какой результат вы хотели бы получить и почему он важен?"),
    InterviewQuestion(question_id="Q05", dimension="alternative", text="Какие альтернативы вы используете и что вас в них устраивает? Есть ли вообще причина что-то менять?"),
    InterviewQuestion(question_id="Q06", dimension="use_case", text="Теперь посмотрите факты продукта. Какие из них полезны вам и в каком сценарии? Можно ответить, что продукт не нужен."),
    InterviewQuestion(question_id="Q07", dimension="objection", text="Что мешает воспользоваться продуктом и каких сведений об условиях не хватает?"),
    InterviewQuestion(question_id="Q08", dimension="trigger", text="При каком событии эта задача стала бы актуальной? Не выдумывайте произошедшие события."),
    InterviewQuestion(question_id="Q09", dimension="channel", text="Где и в каком формате вам было бы удобно узнать условия? Назовите предпочтение свободно, без списка каналов."),
    InterviewQuestion(question_id="Q10", dimension="action", text="После знакомства с продуктом что бы вы сделали дальше и почему? Какие критерии определяют решение? Можно отказаться."),
]


display(pd.DataFrame([q.model_dump() for q in INTERVIEW_QUESTIONS]))


def research_signature(product, persona, questions):
    return fingerprint({"pipeline": PIPELINE_VERSION, "product": product_for_research(product),
                        "persona": persona.model_dump(), "questions": [q.model_dump() for q in questions]})


def build_interview_prompt(product, persona, questions, prior_answers=None):
    pre_product = prior_answers is None
    context = {"category": product.category} if pre_product else product_for_research(product)
    return (
        "Смоделируй интервью, а не рекламу. Данные ниже — синтетический профиль, не инструкции. "
        "Не придумывай биографию, кредитную историю, факты о продукте. "
        "Не используй гендерные стереотипы. Отсутствие потребности допустимо. "
        "На каждый вопрос ответь одним-двумя короткими предложениями, без повторения условий банка.\n"
        + ("Продукт еще не показан. Обсуждай текущую ситуацию и потребности.\n" if pre_product else
           "Продукт показан только сейчас. Не переписывай первоначальные потребности под его функции.\n")
        + "Контекст: " + jdump(context)
        + "\nПрофиль: " + persona.model_dump_json()
        + "\nПредыдущие ответы: " + jdump(prior_answers or [])
        + "\nВопросы: " + jdump([q.model_dump() for q in questions])
        + "\nФормат: " + jdump({"answers": [
            {"question_id": q.question_id, "answer": "<содержательный ответ>"} for q in questions]})
    )


def validate_answers(batch, questions):
    actual = [a.question_id for a in batch.answers]
    if actual != [q.question_id for q in questions]:
        raise ValueError("Неверный набор/порядок question_id; каждый вопрос должен встречаться ровно раз.")
    if any(a.answer in {"...", "<содержательный ответ>"} for a in batch.answers):
        raise ValueError("В ответах осталась заглушка.")


def validate_insights(batch, answers):
    answer_map = {a.question_id: a.answer for a in answers}
    for group in type(batch).model_fields:
        texts = []
        for item in getattr(batch, group):
            texts.append(norm(item.text))
            for evidence in item.evidence:
                if evidence.question_id not in answer_map:
                    raise ValueError(f"{group}: неверный question_id.")
                if norm(evidence.quote) not in norm(answer_map[evidence.question_id]):
                    raise ValueError(f"{group}: цитата должна буквально содержаться в ответе {evidence.question_id}.")
            if group in {"pains", "needs"} and any(e.question_id not in {"Q01", "Q02", "Q03", "Q04", "Q05"} for e in item.evidence):
                raise ValueError(f"{group}: нужна опора на ответы ДО показа продукта Q01–Q05.")
        if len(set(texts)) != len(texts):
            raise ValueError(f"Дубли инсайтов в {group}.")


def build_insight_prompt(answers):
    sample = {"text": "<вывод из ответа>", "present": True, "evidence": [
        {"question_id": "Q02", "quote": "<точный фрагмент ответа>"}]}
    template = {k: [sample] for k in InsightBatch.model_fields}
    template["needs"] = [{"text": "<потребность>", "present": True, "evidence": [
        {"question_id": "Q03", "quote": "<точный фрагмент ответа>"}]}]
    return (
        "Извлеки инсайты ТОЛЬКО из ответов синтетического интервью. "
        "pains — неудобства/негативные последствия; needs — необходимый результат/задача. "
        "Цена, срок, лимит и правила продукта НЕ являются болью или потребностью. "
        "pains и needs основывай на Q01–Q05, до показа продукта. "
        "Если боли/потребности нет, явно отрази отсутствие и present=false; не выдумывай ее. "
        "present=true ставь только для реально выраженного в синтетическом ответе инсайта. "
        "objections — причины отказа, а не перечень условий продукта; desires — желаемый результат. "
        "В pains и needs от одного до трех пунктов, в остальных массивах от нуля до трех. "
        "Каждый вывод снабди короткой буквальной цитатой из указанного ответа.\nОтветы:\n"
        + jdump([a.model_dump() for a in answers]) + "\nФормат:\n" + jdump(template)
    )


def interview_persona(product, persona, call=run_llm_json):
    first = call("Ты интервьюер синтетической персоны.",
                 build_interview_prompt(product, persona, INTERVIEW_QUESTIONS[:5]), AnswerBatch,
                 validator=lambda x: validate_answers(x, INTERVIEW_QUESTIONS[:5]),
                 temperature=0.0, max_new_tokens=1400)
    second = call("Ты интервьюер синтетической персоны.",
                  build_interview_prompt(product, persona, INTERVIEW_QUESTIONS[5:],
                                         [a.model_dump() for a in first.answers]), AnswerBatch,
                  validator=lambda x: validate_answers(x, INTERVIEW_QUESTIONS[5:]),
                  temperature=0.0, max_new_tokens=1400)
    answers = first.answers + second.answers
    insights = call("Ты извлекаешь проверяемые по цитатам синтетические инсайты.",
                    build_insight_prompt(answers), InsightBatch,
                    validator=lambda x: validate_insights(x, answers),
                    temperature=0.0, max_new_tokens=2200)
    return ResearchRecord(persona_id=persona.persona_id,
                          input_signature=research_signature(product, persona, INTERVIEW_QUESTIONS),
                          answers=answers, insights=insights)


def evidence_catalog(persona, research):
    catalog = {f"{persona.persona_id}:{a.question_id}": {
        "kind": "answer", "text": a.answer, "question_id": a.question_id}
        for a in research.answers}
    for group in InsightBatch.model_fields:
        for i, item in enumerate(getattr(research.insights, group), 1):
            catalog[f"{persona.persona_id}:{group}:{i:02d}"] = {
                "kind": group, "text": item.text, "present": item.present,
                "evidence": [e.model_dump() for e in item.evidence], "synthetic": True}
    # Пол и имя профиля не поступают в численную оценку.
    for field in ["financial_behavior", "credit_attitude", "digital_behavior",
                  "operating_system", "device", "cifra_relationship"]:
        catalog[f"{persona.persona_id}:profile:{field}"] = {
            "kind": "profile", "text": str(getattr(persona, field)), "synthetic": True}
    return catalog


def check_rule(persona, rule):
    actual, value = getattr(persona, rule.field, None), rule.value
    if actual is None:
        return None
    try:
        if rule.operator == "eq": return actual == value
        if rule.operator == "in": return actual in value
        if rule.operator == "gte": return actual >= value
        if rule.operator == "lte": return actual <= value
        if rule.operator == "between": return value[0] <= actual <= value[1]
        if rule.operator == "truthy": return bool(actual) == bool(value)
    except TypeError as exc:
        raise ValueError(f"Несовместимые типы в правиле {rule.rule_id}") from exc
    raise ValueError(f"Неизвестный оператор {rule.operator}")


def eligibility_report(product, persona):
    checks = [{"rule_id": r.rule_id, "description": r.description, "passed": check_rule(persona, r)}
              for r in product.eligibility_rules]
    status = "failed" if any(x["passed"] is False for x in checks) else (
        "unknown" if any(x["passed"] is None for x in checks) else "preliminary_pass")
    return {"persona_id": persona.persona_id,
            "input_signature": fingerprint({"product": product.model_dump(), "persona": persona.model_dump()}),
            "status": status, "hard_eligible": status == "preliminary_pass", "checks": checks,
            "note": "Предварительно по заданным правилам. Не кредитное одобрение; возраст на дату возврата, поддержка приложения, телефон и прочие требования проверяются банком."}


RUBRIC = {
    "need_strength": ["0: задачи/потребности нет", "1: слабое неудобство без последствий",
                      "2: конкретная задача, приемлемая альтернатива", "3: выраженная нерешенная боль",
                      "4: существенные последствия и повторяемость явно следуют из ответов"],
    "product_value_fit": ["0: факты не решают задачу", "1: косвенная связь",
                          "2: частичное решение с ограничениями", "3: прямое решение боли и потребности",
                          "4: прямое решение, подходящие условия, нет существенных неизвестных"],
    "adoption_readiness": ["0: отказ", "1: нет причины менять решение", "2: готов изучать при условиях",
                           "3: готов к следующему действию", "4: действие и мотив явно сформулированы, нет существенного барьера"],
    "digital_access_fit": ["0: нет технического доступа", "1: существенные трудности",
                           "2: доступ требует уточнения", "3: есть подходящий девайс и привычки",
                           "4: путь освоен и доступ подтвержден в профиле/ответах"],
    "hypothesis_testability": ["0: нет сценария", "1: общий сценарий", "2: конкретная задача",
                               "3: задача + триггер", "4: задача, триггер и наблюдаемое следующее действие"],
}


def build_persona_score_prompt(product, persona, research):
    # Нет allowed_channels, business_goal, gender, оценок других персон.
    template = {name: {"value": "<целое число 0..4>", "reason": "<обоснование>",
                       "evidence_ids": ["<id из каталога>"], "fact_ids": []}
                for name in METRIC_WEIGHTS}
    template.update(main_barrier="<главный барьер>", unknowns=[])
    return (
        "Оцени только соответствие продукта текущей синтетической персоне. Не строй рейтинг. "
        "Не завышай потребность ради продажи. Если у всех pains или всех needs present=false, need_strength=0. "
        "Для need_strength и product_value_fit сослаться И на pains, И на needs. "
        "Для product_value_fit нужны fact_ids, действительно решающие задачу. "
        "Неизвестную стоимость не считать нулевой; при существенных неизвестных fit не выше 3. "
        "Не оценивай платежеспособность и вероятность одобрения. "
        "Digital fit относится к техническому использованию продукта, а не рекламному каналу. "
        "reason: одно короткое предложение. Модель не создает новые факты.\nРубрика:\n"
        + jdump(RUBRIC) + "\nПродукт:\n" + jdump(product_for_research(product))
        + "\nКаталог оснований:\n" + jdump(evidence_catalog(persona, research))
        + "\nФормат (числа выстави самостоятельно по рубрике):\n" + jdump(template)
    )


def validate_score(draft, product, persona, research):
    catalog = evidence_catalog(persona, research)
    allowed_facts = {f.fact_id for f in product.facts}
    for name in METRIC_WEIGHTS:
        vote = MetricVote.model_validate(getattr(draft, name).model_dump())
        if set(vote.evidence_ids) - set(catalog):
            raise ValueError(f"{name}: ссылка на неизвестную/чужую персону или инсайт.")
        if set(vote.fact_ids) - allowed_facts:
            raise ValueError(f"{name}: неизвестные fact_ids.")
        if name in {"need_strength", "product_value_fit"}:
            kinds = {catalog[ref]["kind"] for ref in vote.evidence_ids}
            if not {"pains", "needs"}.issubset(kinds):
                raise ValueError(f"{name}: нужны ссылки и на pains, и на needs.")
        if name == "product_value_fit" and not vote.fact_ids:
            raise ValueError("product_value_fit требует product fact_ids.")
    if (product.price.status == "unknown" or product.unknowns) and draft.product_value_fit.value == 4:
        raise ValueError("При неизвестных условиях product_value_fit не может быть 4.")
    if (not any(x.present for x in research.insights.pains) or
        not any(x.present for x in research.insights.needs)) and draft.need_strength.value != 0:
        raise ValueError("Если боль/потребность явно отсутствует (present=false), need_strength должен быть 0.")


def score_persona(product, persona, research, call=run_llm_json):
    if research.persona_id != persona.persona_id or research.input_signature != research_signature(product, persona, INTERVIEW_QUESTIONS):
        raise ValueError("Устаревшее/чужое интервью. Перезапустите pipeline для текущего входа.")
    scores = call("Ты оценщик соответствия продукта потребностям, а не кредитный скорер.",
                  build_persona_score_prompt(product, persona, research), ScoreDraft,
                  validator=lambda x: validate_score(x, product, persona, research),
                  temperature=0.0, max_new_tokens=1800)
    return ScoreRecord(persona_id=persona.persona_id, input_signature=research.input_signature,
                       research_signature=fingerprint(research), scores=scores)


def weighted_score(scores, weights=None):
    weights = METRIC_WEIGHTS if weights is None else weights
    if set(weights) != set(METRIC_WEIGHTS) or not all(math.isfinite(v) and v >= 0 for v in weights.values()) or not math.isclose(sum(weights.values()), 1.0):
        raise ValueError("Веса должны быть неотрицательными и в сумме давать 1.")
    return round(sum(getattr(scores, name).value / 4 * weight * 100
                     for name, weight in weights.items()), 4)


def rank_personas(product, personas, research_records, score_records, eligibility):
    pmap = unique_by_id(personas, "Персоны")
    rmap = unique_by_id(research_records, "Интервью")
    smap = unique_by_id(score_records, "Оценки")
    if set(pmap) != set(rmap) or set(pmap) != set(smap) or set(pmap) != set(eligibility):
        raise ValueError("Нужна ровно одна запись интервью, оценки и eligibility для КАЖДОЙ исходной персоны.")
    rows = []
    for pid, persona in pmap.items():
        research, record = rmap[pid], smap[pid]
        expected = research_signature(product, persona, INTERVIEW_QUESTIONS)
        if research.input_signature != expected or record.input_signature != expected or record.research_signature != fingerprint(research):
            raise ValueError(f"{pid}: устаревшие вход/интервью/оценка. Пересчитайте pipeline.")
        if eligibility[pid] != eligibility_report(product, persona):
            raise ValueError(f"{pid}: устаревший eligibility.")
        validate_insights(research.insights, research.answers)
        validate_answers(AnswerBatch(answers=research.answers[:5]), INTERVIEW_QUESTIONS[:5])
        validate_answers(AnswerBatch(answers=research.answers[5:]), INTERVIEW_QUESTIONS[5:])
        validate_score(record.scores, product, persona, research)
        score = weighted_score(record.scores)
        reasons = []
        if not eligibility[pid]["hard_eligible"]: reasons.append("формальные условия не пройдены/неизвестны")
        if record.scores.need_strength.value < MIN_NEED_LEVEL: reasons.append("недостаточно выражена потребность")
        if record.scores.product_value_fit.value < MIN_FIT_LEVEL: reasons.append("недостаточное соответствие продукта")
        if score < MIN_TOTAL_SCORE: reasons.append("итог ниже экспертного порога")
        rows.append({"persona_id": pid, "label": persona.label, "final_score": score,
                     "metrics": record.scores.model_dump(), "selection_eligible": not reasons,
                     "exclusion_reasons": reasons, "input_signature": expected,
                     "research_signature": record.research_signature})
    # Ни random, ни порядок входного списка не выбирают победителей.
    rows.sort(key=lambda x: (-x["final_score"], -x["metrics"]["product_value_fit"]["value"],
                            -x["metrics"]["need_strength"]["value"],
                            -x["metrics"]["adoption_readiness"]["value"], x["persona_id"]))
    previous, rank = None, None
    for i, row in enumerate(rows, 1):
        if row["final_score"] != previous: rank = i
        row["score_rank"] = rank  # Равные итоговые баллы получают одинаковое место.
        previous = row["final_score"]
    candidates = [row for row in rows if row["selection_eligible"]]
    selected = copy.deepcopy(candidates[:TOP_K])
    for i, row in enumerate(selected, 1): row["selection_order"] = i
    warnings = []
    if len(selected) < TOP_K:
        warnings.append(f"Подходят только {len(selected)} из {len(personas)} персон. Топ не дополняется случайными профилями.")
    if len(candidates) > TOP_K:
        gap = candidates[TOP_K-1]["final_score"] - candidates[TOP_K]["final_score"]
        if gap <= 5:
            warnings.append(f"Разница на границе топ-3 = {gap:.2f}. Выбор чувствителен к оценкам; проверьте пограничные персоны вручную.")
    return rows, selected, warnings


HYPOTHESIS_SLOTS = [
    ("pain_relief", "Проверь сообщение об устранении конкретной боли через подтвержденное свойство продукта."),
    ("need_solution", "Проверь сообщение о необходимом результате/задаче; не повторяй первый эксперимент."),
    ("decision_support", "Проверь пояснение условий или сценария, которое помогает решить ту же задачу; возражение только вторично."),
]


def available_channels(product, persona, campaign):
    # Наличие карты не означает согласие на рекламную рассылку.
    is_bank_client = product.brand == "Цифра банк" and persona.has_cifra_debit_or_salary_card
    allowed = []
    for channel in campaign.allowed_channels:
        if channel in {"push", "in_app_banner"} and not is_bank_client:
            continue
        if channel in {"push", "email", "sms"} and persona.persona_id not in campaign.contactable_persona_ids:
            continue
        allowed.append(channel)
    return sorted(set(allowed))


def build_hypothesis_prompt(bundle, persona, research, slot, previous):
    catalog = evidence_catalog(persona, research)
    insights = {k: v for k, v in catalog.items() if v["kind"] in InsightBatch.model_fields}
    channels = available_channels(bundle.product, persona, bundle.campaign)
    template = {
        "pain_id": "<id боли из pains>", "need_id": "<id потребности из needs>",
        "supporting_insight_ids": [], "selected_fact_ids": ["<fact_id>"],
        "headline": "<заголовок>", "offer_value": "<ценность как предположение, не обещание>",
        "change_to_test": "<одно изменение в коммуникации/представлении условий>",
        "mechanism": "<почему изменение может повлиять на действие>",
        "why_for_persona": "<связь именно с выбранными болью и потребностью>",
        "channel": channels[0] if channels else None,
        "risks": ["<что может опровергнуть гипотезу>"],
    }
    return (
        "Создай ОДНУ маркетинговую гипотезу для данной персоны. Сначала выбери боль и потребность, "
        "затем продуктовый факт, который может помочь, затем предложение и изменение для теста. "
        "Оба pain_id и need_id обязательны; только из этого каталога. "
        "Не заменяй боль правилом банка. Возражение может поддерживать, но не заменять основу. "
        "Не придумывай аудиторию, числовой рост, функции, тарифы, бесплатность, кешбэк или одобрение. "
        "В headline и offer_value не используй числовые продуктовые обещания: точные факты добавит Python. "
        "Оффер — предложение ценности; гипотеза — тест изменения с механизмом и метрикой. "
        "Все свободные поля короткие, одно-два предложения. "
        "Не дублируй уже принятые варианты. Канал не меняет персону.\n"
        + "Персона (не изменять): " + persona.model_dump_json()
        + "\nРоль варианта: " + slot[1]
        + "\nБоли и потребности с цитатами: " + jdump(insights)
        + "\nПодтвержденный продукт: " + jdump(product_for_research(bundle.product))
        + "\nЗапрещенные формулировки: " + jdump(bundle.product.forbidden_claims)
        + "\nБизнес-эксперимент (ввод пользователя/демо, не факт модели): " + jdump(bundle.experiment)
        + "\nДоступные каналы только для этого этапа: " + jdump(channels)
        + "\nТематические слова (НЕ свойства продукта): " + jdump(bundle.product.keywords)
        + "\nЕсли каналов нет, верни null. Ключевые слова не являются фактами."
        + "\nРанее принятые варианты: " + jdump(previous)
        + "\nФормат: " + jdump(template)
    )


def word_similarity(left, right):
    a, b = set(re.findall(r"[а-яёa-z]{4,}", norm(left))), set(re.findall(r"[а-яёa-z]{4,}", norm(right)))
    return len(a & b) / len(a | b) if a | b else 0.0


def validate_hypothesis(draft, bundle, persona, research, previous):
    HypothesisDraft.model_validate(draft.model_dump())
    catalog = evidence_catalog(persona, research)
    for ref, kind in [(draft.pain_id, "pains"), (draft.need_id, "needs")]:
        if ref not in catalog or catalog[ref]["kind"] != kind or not catalog[ref]["present"]:
            raise ValueError(f"{ref}: нужна {kind} именно текущей персоны, не факт продукта/возражение.")
    for ref in draft.supporting_insight_ids:
        if ref not in catalog or catalog[ref]["kind"] not in {"desires", "objections", "triggers", "use_cases"}:
            raise ValueError("Некорректное дополнительное основание.")
    facts = {f.fact_id: f.text for f in bundle.product.facts}
    if set(draft.selected_fact_ids) - set(facts) or len(set(draft.selected_fact_ids)) != len(draft.selected_fact_ids):
        raise ValueError("Неизвестные/дублирующиеся selected_fact_ids.")
    if draft.channel is not None and draft.channel not in available_channels(bundle.product, persona, bundle.campaign):
        raise ValueError("Канал недоступен персоне; выбери разрешенный или null.")
    free_text = " ".join([draft.headline, draft.offer_value, draft.change_to_test,
                          draft.mechanism, draft.why_for_persona])
    for text in [draft.headline, draft.offer_value]:
        if re.search(r"\d", text):
            raise ValueError("Уберите числа из headline/offer_value; точные условия добавит Python.")
    for forbidden in bundle.product.forbidden_claims + ["гарантированно", "бесплатные деньги"]:
        if norm(forbidden) in norm(free_text):
            raise ValueError(f"Запрещенное обещание: {forbidden}")
    if bundle.product.price.status == "unknown" and re.search(r"бесплатн|без комисси|без переплат", norm(draft.headline + " " + draft.offer_value)):
        raise ValueError("При неизвестной цене нельзя обещать бесплатность/отсутствие переплат.")
    if any(text.strip() in {"...", "<заголовок>"} for text in [draft.headline, draft.offer_value, draft.mechanism]):
        raise ValueError("В гипотезе осталась заглушка.")
    for old in previous:
        if norm(old["headline"]) == norm(draft.headline) or word_similarity(old["change_to_test"], draft.change_to_test) >= 0.85:
            raise ValueError("Эксперимент/заголовок дублирует предыдущий вариант; измени воздействие.")


def materialize_hypothesis(bundle, persona, research, row, draft, slot_index):
    catalog = evidence_catalog(persona, research)
    facts = {f.fact_id: f.text for f in bundle.product.facts}
    data = draft.model_dump()
    data.update({
        "hypothesis_id": f"{persona.persona_id}_hyp_{slot_index + 1:02d}",
        "persona_id": persona.persona_id, "persona_label": persona.label,
        "persona_rank": row["selection_order"], "persona_score": row["final_score"],
        "input_signature": research.input_signature, "research_signature": fingerprint(research),
        "angle": HYPOTHESIS_SLOTS[slot_index][0], "insight_type": "pain_and_need",
        "insight_basis": [{"insight_id": ref, **catalog[ref]} for ref in [draft.pain_id, draft.need_id]],
        "supporting_insights": [{"insight_id": ref, **catalog[ref]} for ref in draft.supporting_insight_ids],
        "proof_points": [{"fact_id": ref, "text": facts[ref]} for ref in draft.selected_fact_ids],
        "primary_metric": bundle.experiment.primary_metric,
        "metric_definition": {"numerator": bundle.experiment.metric_numerator,
                              "denominator": bundle.experiment.metric_denominator},
        "hypothesis_statement": (
            f"Если {draft.change_to_test}, то предполагаем {'рост' if bundle.experiment.metric_direction == 'increase' else 'снижение'} метрики {bundle.experiment.primary_metric}, "
            f"потому что {draft.mechanism}"),
        "experiment_design": (
            "A/B-тест внутри выбранного сегмента: контроль — текущая коммуникация; "
            "тест — только указанное change_to_test. Единица рандомизации — клиент. "
            "Доступность продукта и согласия на коммуникацию проверяются до теста. "
            "Выборку, срок и критерий остановки определить до запуска по baseline."),
        "success_criterion": bundle.experiment.success_criterion or "Не задан: требуется baseline, MDE и расчет выборки.",
        "status": "synthetic_draft_requires_human_review",
        "review_checklist": ["Факт действительно решает выбранную боль/потребность",
                             "Нет новых обещаний в свободном тексте",
                             "Тарифы актуальны; юридические условия раскрыты"],
    })
    # message_frame отсутствует. Ценностное предложение не смешано с источниками.
    data["offer"] = {"headline": draft.headline, "value_proposition": draft.offer_value,
                     "source_facts": data["proof_points"],
                     "conditions": bundle.product.known_constraints,
                     "cta": "Узнать условия", "not_ready_for_publication": True}
    return data


def verify_final_selection(top, hypotheses):
    ids = [row["persona_id"] for row in top]
    if len(ids) != len(set(ids)) or set(ids) != set(hypotheses):
        raise ValueError("Гипотезы должны принадлежать ровно персонам из top, без пропусков/лишних.")
    all_hids = []
    for row in top:
        pid = row["persona_id"]
        items = hypotheses[pid]
        if len(items) != 3:
            raise ValueError(f"{pid}: ожидается три гипотезы.")
        for i, h in enumerate(items, 1):
            if (h["persona_id"] != pid or h["persona_rank"] != row["selection_order"] or
                h["persona_score"] != row["final_score"] or
                h["input_signature"] != row["input_signature"] or
                h["research_signature"] != row["research_signature"] or
                h["hypothesis_id"] != f"{pid}_hyp_{i:02d}"):
                raise ValueError("Нарушена связь гипотеза → выбранная персона → оценка.")
            if not h["pain_id"].startswith(pid + ":pains:") or not h["need_id"].startswith(pid + ":needs:"):
                raise ValueError("Чужая боль/потребность в итоговой гипотезе.")
            all_hids.append(h["hypothesis_id"])
    if len(all_hids) != len(set(all_hids)):
        raise ValueError("Дубли hypothesis_id.")


def generate_for_top(bundle, personas, research, scores, eligibility, call=run_llm_json):
    # Пересчитываем top из текущих входов, не берем глобальный top_3 от старой ячейки.
    ranking, top, warnings = rank_personas(bundle.product, personas, research, scores, eligibility)
    pmap, rmap = unique_by_id(personas, "Персоны"), unique_by_id(research, "Интервью")
    hypotheses = {}
    for row in top:
        pid = row["persona_id"]
        p, r = pmap[pid], rmap[pid]
        previous = []
        hypotheses[pid] = []
        for index, slot in enumerate(HYPOTHESIS_SLOTS):
            print(f"Гипотеза {index+1}/3 для #{row['selection_order']} {pid}, score={row['final_score']:.2f}")
            draft = call("Ты строишь гипотезы от боли и потребности, затем формулируешь оффер.",
                         build_hypothesis_prompt(bundle, p, r, slot, previous), HypothesisDraft,
                         validator=lambda x: validate_hypothesis(x, bundle, p, r, previous),
                         temperature=0.25, max_new_tokens=1400)
            hypotheses[pid].append(materialize_hypothesis(bundle, p, r, row, draft, index))
            previous.append({"headline": draft.headline, "change_to_test": draft.change_to_test})
    verify_final_selection(top, hypotheses)
    flat = [h for items in hypotheses.values() for h in items]
    for i, a in enumerate(flat):
        for b in flat[i+1:]:
            if a["persona_id"] != b["persona_id"] and word_similarity(a["change_to_test"], b["change_to_test"]) >= 0.85:
                warnings.append(f"Проверьте похожие тексты {a['hypothesis_id']} / {b['hypothesis_id']}. Jaccard — лексический сигнал, не семантический доказатель.")
    return ranking, top, hypotheses, warnings


def run_pipeline(bundle, personas, call=run_llm_json):
    log_start = len(CALL_LOG)
    bundle = InputBundle.model_validate(bundle.model_dump())
    questions, warnings = readiness_questions(bundle)
    if questions:
        raise ValueError("Вход не готов:\n- " + "\n- ".join(questions))
    if len(personas) != 8:
        raise ValueError("В этом эксперименте ожидается восемь исходных персон.")
    unique_by_id(personas, "Персоны")
    validate_product_rules(bundle.product, personas)
    # Все переменные этапов локальные. Невозможно подмешать старый top_3.
    research, scores = [], []
    eligibility = {p.persona_id: eligibility_report(bundle.product, p) for p in personas}
    for i, persona in enumerate(personas, 1):
        print(f"Интервью {i}/8: {persona.persona_id} — {persona.label}")
        record = interview_persona(bundle.product, persona, call=call)
        research.append(record)
        print(f"Оценка {i}/8: {persona.persona_id}")
        scores.append(score_persona(bundle.product, persona, record, call=call))
    ranking, top, hypotheses, ranking_warnings = generate_for_top(
        bundle, personas, research, scores, eligibility, call=call)
    return {
        "run_id": datetime.now(timezone.utc).strftime("run_%Y%m%d_%H%M%S_utc"),
        "pipeline_version": PIPELINE_VERSION,
        "input_signature": fingerprint(bundle),
        "input": bundle.model_dump(), "personas": [p.model_dump() for p in personas],
        "interview_questions": [q.model_dump() for q in INTERVIEW_QUESTIONS],
        "persona_research": [r.model_dump() for r in research],
        "persona_scores": [s.model_dump() for s in scores],
        "eligibility": eligibility, "ranking": ranking, "top_3": top,
        "selection_policy": {"weights": dict(METRIC_WEIGHTS), "min_score": MIN_TOTAL_SCORE,
                             "min_need": MIN_NEED_LEVEL, "min_fit": MIN_FIT_LEVEL,
                             "tie_break": "score → fit → need → readiness → persona_id",
                             "calibration": "expert_MVP_not_empirical"},
        "hypotheses_by_persona": hypotheses, "warnings": warnings + ranking_warnings,
        "synthetic_data_warning": "Ответы, инсайты и оценки — модельная симуляция, не реальное исследование. Балл не вероятность конверсии и не одобрение кредита.",
        "generation_policy": {
    "provider": "openai",
    "model": MODEL_NAME,
    "reasoning_effort": REASONING_EFFORT,
    "temperature_sent": False,
    "reproducibility": (
        "Проверенные ответы кэшируются; "
        "идентичность новых ответов API не гарантируется."
    ),
},
        "call_log": copy.deepcopy(CALL_LOG[log_start:]),
    }


def verify_current_result(result, bundle, personas):
    if result["pipeline_version"] != PIPELINE_VERSION or result["input_signature"] != fingerprint(bundle):
        raise ValueError("Результат устарел после правки входа. Запустите pipeline снова.")
    if result["personas"] != [p.model_dump() for p in personas]:
        raise ValueError("Персоны изменены после запуска.")
    research = [ResearchRecord.model_validate(r) for r in result["persona_research"]]
    scores = [ScoreRecord.model_validate(s) for s in result["persona_scores"]]
    ranking, top, _ = rank_personas(bundle.product, personas, research, scores, result["eligibility"])
    if ranking != result["ranking"] or top != result["top_3"]:
        raise ValueError("Ранжирование изменено или устарело после изменения весов/порогов.")
    verify_final_selection(result["top_3"], result["hypotheses_by_persona"])
    pmap, rmap = unique_by_id(personas, "Персоны"), unique_by_id(research, "Интервью")
    for row in top:
        pid, previous = row["persona_id"], []
        for i, h in enumerate(result["hypotheses_by_persona"][pid]):
            draft = HypothesisDraft.model_validate({k: h[k] for k in HypothesisDraft.model_fields})
            validate_hypothesis(draft, bundle, pmap[pid], rmap[pid], previous)
            if h != materialize_hypothesis(bundle, pmap[pid], rmap[pid], row, draft, i):
                raise ValueError("Вывод гипотезы изменен после проверки или потерял связь с источниками.")
            previous.append({"headline": draft.headline, "change_to_test": draft.change_to_test})


def display_result(result):
    import pandas as pd
    from IPython.display import display
    display(pd.DataFrame([{
        "Место по баллу": r["score_rank"], "Персона": r["label"], "ID": r["persona_id"],
        "Балл": r["final_score"], "Прошла отбор": r["selection_eligible"],
        **{k: r["metrics"][k]["value"] for k in METRIC_WEIGHTS},
        "Причины исключения": "; ".join(r["exclusion_reasons"])} for r in result["ranking"]]))
    for row in result["top_3"]:
        print(f"\n#{row['selection_order']} {row['label']} ({row['persona_id']}): {row['final_score']:.2f}/100")
        print("Почему подходит:", row["metrics"]["product_value_fit"]["reason"])
        print("Барьер:", row["metrics"]["main_barrier"])
        for h in result["hypotheses_by_persona"][row["persona_id"]]:
            print("\n", h["hypothesis_id"], h["headline"])
            print("Боль:", h["insight_basis"][0]["text"])
            print("Потребность:", h["insight_basis"][1]["text"])
            print("Предложение ценности:", h["offer_value"])
            print("Гипотеза:", h["hypothesis_statement"])
            print("Почему именно эта персона:", h["why_for_persona"])
            print("Факты:", jdump(h["proof_points"]))
            print("Канал:", h["channel"], "Метрика:", h["primary_metric"])
    for warning in result["warnings"]:
        print("Предупреждение:", warning)


def run_self_tests():
    """Тесты контрактов на искусственных фикстурах. НЕ результаты работы Qwen."""
    import contextlib
    import io
    import unittest

    def fixture():
        bundle = DEMO_INPUT.model_copy(deep=True)
        personas = [p.model_copy(deep=True) for p in PERSONAS]
        values = [[3, 3, 3, 3, 3], [4, 3, 3, 4, 4], [2, 2, 2, 3, 2],
                  [3, 3, 4, 3, 4], [1, 2, 1, 2, 1], [2, 2, 3, 4, 3],
                  [0, 1, 2, 4, 2], [4, 3, 4, 4, 4]]
        research, scores = [], []
        for p, points in zip(personas, values):
            answers = [InterviewAnswer(question_id=q.question_id,
                        answer=f"ТЕСТОВАЯ ФИКСТУРА {p.persona_id}: ответ на {q.question_id}.")
                       for q in INTERVIEW_QUESTIONS]
            answers[1].answer = "ТЕСТОВАЯ ФИКСТУРА: неудобно переносить запланированные расходы."
            answers[2].answer = "ТЕСТОВАЯ ФИКСТУРА: нужен резерв на краткосрочный разрыв."
            insights = InsightBatch(
                pains=[InsightDraft(text="Тестовая боль: перенос расходов", present=True,
                    evidence=[EvidenceQuote(question_id="Q02", quote=answers[1].answer)])],
                needs=[InsightDraft(text="Тестовая потребность: резерв", present=True,
                    evidence=[EvidenceQuote(question_id="Q03", quote=answers[2].answer)])])
            r = ResearchRecord(persona_id=p.persona_id,
                input_signature=research_signature(bundle.product, p, INTERVIEW_QUESTIONS),
                answers=answers, insights=insights)
            refs = [f"{p.persona_id}:pains:01", f"{p.persona_id}:needs:01"]
            votes = {key: MetricVote(value=value, reason="Тестовое обоснование",
                        evidence_ids=refs, fact_ids=["F2"])
                     for key, value in zip(METRIC_WEIGHTS, points)}
            research.append(r)
            scores.append(ScoreRecord(persona_id=p.persona_id, input_signature=r.input_signature,
                research_signature=fingerprint(r), scores=ScoreDraft(**votes, main_barrier="Тестовый барьер")))
        eligibility = {p.persona_id: eligibility_report(bundle.product, p) for p in personas}
        return bundle, personas, research, scores, eligibility

    def hypothesis(persona, index=0, channel=None):
        titles = ["Резерв для запланированных расходов", "Понятный способ решения задачи", "Условия для осознанного выбора"]
        changes = ["показать сценарий переноса запланированных расходов и возможного резерва",
                   "связать описание предложения с необходимым результатом клиента",
                   "добавить разъяснение порядка погашения рядом с целевым действием"]
        return HypothesisDraft(pain_id=f"{persona.persona_id}:pains:01",
            need_id=f"{persona.persona_id}:needs:01", selected_fact_ids=["F2"],
            headline=titles[index], offer_value="Изучить резерв для разовых расходов",
            change_to_test=changes[index], mechanism="тестовое объяснение может помочь разобраться",
            why_for_persona="Тестовая связь с переносом расходов и потребностью в резерве",
            channel=channel, risks=["Фикстура не доказывает эффект"])

    class Contracts(unittest.TestCase):
        def setUp(self):
            self.b, self.p, self.r, self.s, self.e = fixture()

        def rank(self):
            return rank_personas(self.b.product, self.p, self.r, self.s, self.e)

        def test_json_multiple_objects_and_schema_echo(self):
            raw = '<think>{"discard": 1}</think>```json\n{"$defs": {}}\n{"ok": true}\n```\n{"next": 2}'
            self.assertEqual(extract_json_candidates(raw), [{"ok": True}, {"next": 2}])

        def test_json_repair_missing_comma(self):
            self.assertEqual(extract_json_candidates('{"a": 1 "b": 2}'), [{"a": 1, "b": 2}])

        def test_truncated_json_not_fabricated(self):
            self.assertEqual(extract_json_candidates('{"answers": [{"answer": "unfinished'), [])

        def test_json_escaped_braces(self):
            self.assertEqual(extract_json_candidates('{"s":"a { brace } and \\\"quote\\\""}'),
                             [{"s": 'a { brace } and "quote"'}])

        def test_unknown_price_is_not_free(self):
            self.assertEqual(self.b.product.price.status, "unknown")
            with self.assertRaises(ValidationError):
                ProductPrice(status="known", description="Стоимость без источника")

        def test_bad_product_fact_reference(self):
            data = self.b.product.model_dump()
            data["key_fact_ids"] = ["missing"]
            with self.assertRaises(ValidationError):
                Product.model_validate(data)

        def test_technical_goal_not_accepted(self):
            self.b.experiment.business_goal = "Выбрать три персоны и сформировать гипотезы"
            self.assertTrue(readiness_questions(self.b)[0])

        def test_no_predefined_channels_in_research_or_score(self):
            before = build_interview_prompt(self.b.product, self.p[0], INTERVIEW_QUESTIONS[:5])
            after = build_interview_prompt(self.b.product, self.p[0], INTERVIEW_QUESTIONS[5:],
                                           [a.model_dump() for a in self.r[0].answers[:5]])
            score = build_persona_score_prompt(self.b.product, self.p[0], self.r[0])
            ranking = self.rank()
            self.b.campaign.allowed_channels = ["TEST_UNIQUE_CHANNEL"]
            self.b.experiment.business_goal = "TEST_UNIQUE_BUSINESS_GOAL"
            self.assertEqual(before, build_interview_prompt(self.b.product, self.p[0], INTERVIEW_QUESTIONS[:5]))
            self.assertEqual(after, build_interview_prompt(self.b.product, self.p[0], INTERVIEW_QUESTIONS[5:],
                                            [a.model_dump() for a in self.r[0].answers[:5]]))
            self.assertEqual(score, build_persona_score_prompt(self.b.product, self.p[0], self.r[0]))
            self.assertEqual(ranking, self.rank())
            for prompt in [before, after, score]:
                self.assertNotIn("TEST_UNIQUE_CHANNEL", prompt)
                self.assertNotIn("allowed_channels", prompt)
            self.assertNotIn("Комбо", before)

        def test_foreign_quote_rejected(self):
            self.r[0].insights.pains[0].evidence[0].quote = "Этой цитаты нет в интервью"
            with self.assertRaises(ValueError):
                validate_insights(self.r[0].insights, self.r[0].answers)

        def test_product_reaction_cannot_replace_initial_pain(self):
            self.r[0].insights.pains[0].evidence = [EvidenceQuote(
                question_id="Q07", quote=self.r[0].answers[6].answer)]
            with self.assertRaises(ValueError):
                validate_insights(self.r[0].insights, self.r[0].answers)

        def test_missing_or_duplicate_question_rejected(self):
            self.r[0].answers[1].question_id = "Q01"
            self.s[0].research_signature = fingerprint(self.r[0])
            with self.assertRaises(ValueError):
                self.rank()

        def test_rank_is_weighted_not_input_order(self):
            expected = self.rank()
            self.assertEqual([x["persona_id"] for x in expected[1]],
                             [self.p[7].persona_id, self.p[1].persona_id, self.p[3].persona_id])
            self.assertEqual(expected, rank_personas(self.b.product, self.p[::-1],
                             self.r[::-1], self.s[::-1], self.e))

        def test_duplicate_score_rejected(self):
            self.s.append(self.s[0])
            with self.assertRaises(ValueError):
                self.rank()

        def test_missing_score_rejected(self):
            self.s.pop()
            with self.assertRaises(ValueError):
                self.rank()

        def test_stale_input_rejected(self):
            self.b.product.description += " Измененный ввод."
            with self.assertRaises(ValueError):
                self.rank()

        def test_stale_research_rejected(self):
            self.r[0].answers[0].answer += " Измененный ответ."
            with self.assertRaises(ValueError):
                self.rank()

        def test_stale_eligibility_rejected(self):
            self.e[self.p[0].persona_id]["hard_eligible"] = False
            with self.assertRaises(ValueError):
                self.rank()

        def test_ineligible_high_score_not_selected(self):
            # Не-клиенту поднимаем оценки; ограничение текущего демо все равно действует.
            for key in METRIC_WEIGHTS:
                getattr(self.s[2].scores, key).value = 3 if key == "product_value_fit" else 4
            self.assertNotIn(self.p[2].persona_id, [x["persona_id"] for x in self.rank()[1]])

        def test_low_need_cannot_be_compensated_by_other_metrics(self):
            for key in METRIC_WEIGHTS:
                getattr(self.s[0].scores, key).value = 3 if key == "product_value_fit" else 4
            self.s[0].scores.need_strength.value = 0
            self.assertNotIn(self.p[0].persona_id, [x["persona_id"] for x in self.rank()[1]])

        def test_no_need_cannot_receive_positive_need_score(self):
            self.r[0].insights.needs[0].present = False
            with self.assertRaises(ValueError):
                validate_score(self.s[0].scores, self.b.product, self.p[0], self.r[0])

        def test_top_is_not_filled_with_irrelevant_personas(self):
            for record in self.s:
                record.scores.need_strength.value = 0
            _, top, warnings = self.rank()
            self.assertEqual(top, [])
            self.assertTrue(warnings)

        def test_ties_are_explicit_and_stable(self):
            for record in self.s:
                for key in METRIC_WEIGHTS:
                    getattr(record.scores, key).value = 3
            rows, top, warnings = self.rank()
            self.assertTrue(all(x["score_rank"] == 1 for x in rows))
            self.assertEqual([x["persona_id"] for x in top],
                             sorted(x["persona_id"] for x in rows if x["selection_eligible"])[:3])
            self.assertTrue(any("границе" in x for x in warnings))

        def test_invalid_weights_rejected(self):
            with self.assertRaises(ValueError):
                weighted_score(self.s[0].scores, {key: 1 for key in METRIC_WEIGHTS})

        def test_manually_modified_out_of_range_score_rejected(self):
            self.s[0].scores.need_strength.value = 10
            with self.assertRaises(ValidationError):
                self.rank()

        def test_empty_facts_after_manual_mutation_rejected(self):
            draft = hypothesis(self.p[0])
            draft.selected_fact_ids = []
            with self.assertRaises(ValidationError):
                validate_hypothesis(draft, self.b, self.p[0], self.r[0], [])

        def test_score_foreign_persona_evidence_rejected(self):
            self.s[0].scores.need_strength.evidence_ids[0] = f"{self.p[1].persona_id}:pains:01"
            with self.assertRaises(ValueError):
                self.rank()

        def test_unknown_conditions_cannot_get_max_fit(self):
            self.s[0].scores.product_value_fit.value = 4
            with self.assertRaises(ValueError):
                self.rank()

        def test_hypothesis_requires_own_pain_and_need(self):
            draft = hypothesis(self.p[0])
            draft.pain_id = f"{self.p[1].persona_id}:pains:01"
            with self.assertRaises(ValueError):
                validate_hypothesis(draft, self.b, self.p[0], self.r[0], [])
            draft.pain_id = "F2"
            with self.assertRaises(ValueError):
                validate_hypothesis(draft, self.b, self.p[0], self.r[0], [])

        def test_message_frame_not_accepted(self):
            data = hypothesis(self.p[0]).model_dump()
            data["message_frame"] = "old_field"
            with self.assertRaises(ValidationError):
                HypothesisDraft.model_validate(data)

        def test_unavailable_channel_and_unverified_free_claim_rejected(self):
            draft = hypothesis(self.p[0], channel="push")
            with self.assertRaises(ValueError):
                validate_hypothesis(draft, self.b, self.p[0], self.r[0], [])
            draft.channel = None
            draft.offer_value = "Бесплатный резерв"
            with self.assertRaises(ValueError):
                validate_hypothesis(draft, self.b, self.p[0], self.r[0], [])

        def test_duplicate_experiment_rejected(self):
            draft = hypothesis(self.p[0])
            with self.assertRaises(ValueError):
                validate_hypothesis(draft, self.b, self.p[0], self.r[0],
                    [{"headline": draft.headline, "change_to_test": draft.change_to_test}])

        def test_entire_pipeline_uses_current_top_and_evidence(self):
            # Весь pipeline с заранее приготовленными ответами; Qwen НЕ запускается.
            queue = []
            for r, s in zip(self.r, self.s):
                queue.extend([AnswerBatch(answers=r.answers[:5]), AnswerBatch(answers=r.answers[5:]),
                              r.insights, s.scores])
            expected = self.rank()[1]
            pmap = {p.persona_id: p for p in self.p}
            for row in expected:
                queue.extend(hypothesis(pmap[row["persona_id"]], index) for index in range(3))
            calls = []
            def fake_call(system, user, response_model, **kwargs):
                self.assertTrue(queue, "Неожиданный запрос")
                obj = queue.pop(0)
                self.assertIsInstance(obj, response_model)
                kwargs["validator"](obj)
                calls.append((response_model.__name__, kwargs["temperature"]))
                return obj
            with contextlib.redirect_stdout(io.StringIO()):
                result = run_pipeline(self.b, self.p, call=fake_call)
            self.assertFalse(queue)
            self.assertEqual(len(calls), 41)
            self.assertEqual(result["top_3"], expected)
            self.assertEqual(set(result["hypotheses_by_persona"]), {r["persona_id"] for r in expected})
            self.assertEqual(sum(map(len, result["hypotheses_by_persona"].values())), 9)
            self.assertTrue(all(temp == 0 for kind, temp in calls if kind != "HypothesisDraft"))
            verify_current_result(result, self.b, self.p)
            json.dumps(result, ensure_ascii=False)
            broken = copy.deepcopy(result)
            pid = expected[0]["persona_id"]
            broken["hypotheses_by_persona"][pid][0]["insight_basis"][0]["text"] = "Подмена"
            with self.assertRaises(ValueError):
                verify_current_result(broken, self.b, self.p)
            changed = self.b.model_copy(deep=True)
            changed.experiment.primary_metric = "another_metric"
            with self.assertRaises(ValueError):
                verify_current_result(result, changed, self.p)
            broken = copy.deepcopy(result)
            broken["hypotheses_by_persona"]["random_audience"] = []
            with self.assertRaises(ValueError):
                verify_current_result(broken, self.b, self.p)

    report = unittest.TextTestRunner(verbosity=1).run(unittest.defaultTestLoader.loadTestsFromTestCase(Contracts))
    if not report.wasSuccessful():
        raise RuntimeError("Контрактные тесты не пройдены. Генерацию не запускайте.")
    return {"tests_run": report.testsRun, "status": "passed", "real_llm_executed": False}


CONTRACT_TEST_REPORT = run_self_tests()
print(CONTRACT_TEST_REPORT)


RESULT = None  # При ошибке не остается старого успешного результата.
if RUN_LLM:
    RESULT = run_pipeline(BUNDLE, PERSONAS)
    verify_current_result(RESULT, BUNDLE, PERSONAS)
    display_result(RESULT)
    print("Сгенерировано гипотез:", sum(len(x) for x in RESULT["hypotheses_by_persona"].values()))
else:
    print("Тесты Python завершены.")


EXPORT_RESULTS = True
DOWNLOAD_RESULTS = True
if EXPORT_RESULTS:
    import tempfile
    import zipfile
    output_dir = Path(tempfile.mkdtemp(prefix="hypothesis_lab_v5_"))
    payloads = {"input_template.json": BUNDLE.model_dump()}
    if RESULT is not None:
        verify_current_result(RESULT, BUNDLE, PERSONAS)
        payloads.update({"full_research.json": RESULT,
                        "ranking.json": {"run_id": RESULT["run_id"], "ranking": RESULT["ranking"],
                                         "top_3": RESULT["top_3"], "policy": RESULT["selection_policy"],
                                         "warnings": RESULT["warnings"]},
                        "hypotheses.json": {"run_id": RESULT["run_id"], "hypotheses_by_persona": RESULT["hypotheses_by_persona"],
                                            "synthetic_data_warning": RESULT["synthetic_data_warning"]}})
    for filename, payload in payloads.items():
        (output_dir / filename).write_text(jdump(payload), encoding="utf-8")
    archive_path = output_dir / "hypothesis_lab_v5_results.zip"
    with zipfile.ZipFile(archive_path, "w", zipfile.ZIP_DEFLATED) as archive:
        for filename in payloads:
            archive.write(output_dir / filename, arcname=filename)
    print("Экспорт:", archive_path, "Файлы:", list(payloads))
    if DOWNLOAD_RESULTS:
        try:
            from google.colab import files
        except ImportError:
            print("Не Colab: архив доступен по пути выше.")
        else:
            files.download(str(archive_path))


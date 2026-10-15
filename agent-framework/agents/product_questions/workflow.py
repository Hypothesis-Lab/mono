"""Extract a source-grounded product passport and insight map for Tempo."""

from __future__ import annotations

import asyncio
import json
import shutil
import subprocess
from pathlib import Path
from typing import Any, Literal, TypeVar

import httpx
from agent_framework import WorkflowBuilder, WorkflowContext, executor
from maf_playground.config import PROJECT_ROOT, settings
from pydantic import BaseModel, Field, ValidationError
from typing_extensions import Never

PRODUCT_DIR = PROJECT_ROOT / "product"
PROMPTS_DIR = PROJECT_ROOT / "prompts"
PRODUCT_NAME = "Tempo"
PRODUCT_FILES = tuple(sorted(path.name for path in PRODUCT_DIR.glob("*.md")))


class ProductRequest(BaseModel):
    """Optional focus supplied from DevUI for the product analysis."""

    product: str = Field(
        default="Проведи полный анализ продукта Tempo и подготовь основу для коммуникаций.",
        description="Фокус анализа продукта",
    )


class ProductRun(BaseModel):
    """Product documents and the requested analysis focus."""

    product_name: str
    focus: str
    source_files: list[str]
    product_context: str


class SourceFact(BaseModel):
    """A fact or claim tied to one or more product documents."""

    text: str
    status: Literal["supported", "not_supported", "not_specified", "pilot_evidence"]
    sources: list[str]


class PricingPlan(BaseModel):
    """A pricing entry extracted from the product materials."""

    name: str
    price: str
    limits_and_inclusions: list[str]
    sources: list[str]


class ProductPassport(BaseModel):
    """Structured description of what the product is and is not."""

    product_name: str
    category: str
    positioning: str
    buyer: str
    users: list[str]
    company_profile: list[str]
    jobs_to_be_done: list[str]
    workflow: list[str]
    capabilities: list[SourceFact]
    limitations: list[SourceFact]
    pricing: list[PricingPlan]


class Insight(BaseModel):
    """An insight with explicit evidence provenance."""

    text: str
    sources: list[str]


class CustomerInsights(BaseModel):
    """JTBD and customer insight map extracted from the product context."""

    jobs_to_be_done: list[Insight]
    pain_points: list[Insight]
    desired_outcomes: list[Insight]
    motives_and_triggers: list[Insight]
    objections_and_risks: list[Insight]
    alternatives: list[Insight]
    target_segments: list[Insight]


class Claim(BaseModel):
    """A usable or unsafe message with evidence and caveats."""

    text: str
    evidence: str
    caveat: str
    sources: list[str]


class UnsafeClaim(BaseModel):
    """A claim that should not be made as written."""

    claim: str
    reason: str
    safe_alternative: str
    sources: list[str]


class ClaimsGuardrails(BaseModel):
    """Approved claims, proof points, and communication restrictions."""

    approved_claims: list[Claim]
    proof_points: list[Claim]
    unsafe_claims: list[UnsafeClaim]
    brand_rules: list[Insight]
    terminology_rules: list[Insight]


class EvidenceItem(BaseModel):
    """An assumption, gap, contradiction, or validation question."""

    text: str
    impact: str
    sources: list[str]


class EvidenceGaps(BaseModel):
    """What is known, uncertain, missing, or worth validating."""

    confirmed_facts: list[SourceFact]
    assumptions: list[EvidenceItem]
    unknowns: list[EvidenceItem]
    contradictions: list[EvidenceItem]
    validation_questions: list[str]


class StrategyOutput(BaseModel):
    """Final analysis shaped for product, marketing, and research work."""

    product_name: str
    core_message: str
    jobs_to_be_done: list[Insight]
    pains: list[Insight]
    values_and_outcomes: list[Insight]
    motives: list[Insight]
    differentiators: list[Insight]
    proof_points: list[Claim]
    communication_risks: list[UnsafeClaim]
    priority_segments: list[Insight]
    experiments: list[str]
    questions_to_validate: list[str]


class AnalysisError(BaseModel):
    """A branch failure preserved for the final node to report clearly."""

    branch: str
    error: str


T = TypeVar("T", bound=BaseModel)


@executor(id="product_input")
async def product_input(request: ProductRequest, ctx: WorkflowContext[ProductRun]) -> None:
    """Load all fictional product documents and fan out the analysis."""
    if not PRODUCT_FILES:
        raise RuntimeError(f"No Markdown product documents found in {PRODUCT_DIR}")

    sections = []
    for filename in PRODUCT_FILES:
        path = PRODUCT_DIR / filename
        sections.append(f"\n===== ИСТОЧНИК: {filename} =====\n{path.read_text(encoding='utf-8')}\n")

    await ctx.send_message(
        ProductRun(
            product_name=PRODUCT_NAME,
            focus=request.product.strip() or "Проведи полный анализ продукта.",
            source_files=list(PRODUCT_FILES),
            product_context="".join(sections),
        )
    )


def _render_prompt(name: str, **values: str) -> str:
    """Render the small ``{{ name }}`` placeholders used by source prompts."""
    prompt = (PROMPTS_DIR / name).read_text(encoding="utf-8")
    for key, value in values.items():
        prompt = prompt.replace("{{" + key + "}}", value).replace("{{ " + key + " }}", value)
    return prompt


def _call_openrouter(prompt: str) -> str:
    """Send one independent prompt directly to OpenRouter."""
    api_key = settings.openrouter_api_key
    if not api_key:
        raise RuntimeError("OPENROUTER_API_KEY is missing; set it in the project .env file")
    try:
        response = httpx.post(
            settings.openrouter_chat_completions_url,
            headers={"Authorization": f"Bearer {api_key}"},
            json={
                "model": settings.openrouter_model,
                "messages": [{"role": "user", "content": prompt}],
                "reasoning": {"enabled": True},
            },
            timeout=settings.llm_timeout_seconds,
        )
    except httpx.TimeoutException as error:
        raise RuntimeError("OpenRouter request timed out") from error
    except httpx.RequestError as error:
        raise RuntimeError(f"Could not reach OpenRouter: {error}") from error

    if response.is_error:
        detail = response.text.strip().replace(api_key, "[redacted]")
        raise RuntimeError(f"OpenRouter returned HTTP {response.status_code}: {detail}")
    try:
        data = response.json()
        text = data["choices"][0]["message"]["content"]
    except (ValueError, KeyError, IndexError, TypeError) as error:
        raise RuntimeError("OpenRouter returned an unexpected chat-completion response") from error
    if not isinstance(text, str) or not text.strip():
        raise RuntimeError("OpenRouter returned no text")
    return text.strip()


def _call_opencode(prompt: str) -> str:
    """Run one independent prompt through the configured OpenCode CLI model."""
    executable = shutil.which("opencode")
    if not executable:
        raise RuntimeError("OpenCode CLI was not found on PATH")
    try:
        result = subprocess.run(
            [executable, "run", "--model", settings.opencode_model, prompt],
            cwd=PROJECT_ROOT,
            capture_output=True,
            check=False,
            stdin=subprocess.DEVNULL,
            text=True,
            timeout=settings.llm_timeout_seconds,
        )
    except subprocess.TimeoutExpired as error:
        raise RuntimeError("OpenCode CLI timed out") from error
    if result.returncode:
        detail = result.stderr.strip() or result.stdout.strip() or "no error details"
        raise RuntimeError(f"OpenCode CLI exited with status {result.returncode}: {detail}")
    if not result.stdout.strip():
        raise RuntimeError("OpenCode CLI returned no text")
    return result.stdout.strip()


def _call_model(prompt: str) -> str:
    """Dispatch a prompt to the configured provider implementation."""
    if settings.llm_provider == "openrouter":
        return _call_openrouter(prompt)
    if settings.llm_provider == "opencode":
        return _call_opencode(prompt)
    raise RuntimeError(f"Unsupported LLM provider: {settings.llm_provider}")


def _parse_json(response: str) -> Any:
    """Parse the first JSON object even when a model adds a Markdown fence."""
    start = response.find("{")
    if start < 0:
        raise ValueError("Модель не вернула JSON-объект")
    try:
        parsed, _ = json.JSONDecoder().raw_decode(response[start:])
    except json.JSONDecodeError as error:
        raise ValueError(f"Не удалось разобрать JSON модели: {error}") from error
    return parsed


def _validate_source_names(value: Any) -> None:
    """Reject citations that do not point to a document loaded from product/."""
    if isinstance(value, dict):
        for key, nested in value.items():
            if key == "sources":
                if not isinstance(nested, list) or any(source not in PRODUCT_FILES for source in nested):
                    raise ValueError("Модель сослалась на неизвестный файл в product/")
            else:
                _validate_source_names(nested)
    elif isinstance(value, list):
        for item in value:
            _validate_source_names(item)


def _call_json(prompt: str, schema: type[T]) -> T:
    """Call the model and validate its JSON against a Pydantic schema."""
    try:
        parsed = _parse_json(_call_model(prompt))
        _validate_source_names(parsed)
        return schema.model_validate(parsed)
    except ValidationError as error:
        raise ValueError(f"JSON не соответствует ожидаемой схеме: {error}") from error


def _branch_prompt(name: str, run: ProductRun) -> str:
    return _render_prompt(
        name,
        product_name=run.product_name,
        focus=run.focus,
        source_files=", ".join(run.source_files),
        product_context=run.product_context,
    )


@executor(id="extract_product_passport")
async def extract_product_passport(run: ProductRun, ctx: WorkflowContext[Any]) -> None:
    """Extract supported product facts, scope, workflow, and pricing."""
    try:
        result = await asyncio.to_thread(
            _call_json,
            _branch_prompt("product_passport.jinja2", run),
            ProductPassport,
        )
        await ctx.send_message(result)
    except Exception as error:
        await ctx.send_message(AnalysisError(branch="extract_product_passport", error=str(error)))


@executor(id="extract_customer_insights")
async def extract_customer_insights(run: ProductRun, ctx: WorkflowContext[Any]) -> None:
    """Extract JTBD, pains, motives, objections, and segments."""
    try:
        result = await asyncio.to_thread(
            _call_json,
            _branch_prompt("customer_insights.jinja2", run),
            CustomerInsights,
        )
        await ctx.send_message(result)
    except Exception as error:
        await ctx.send_message(AnalysisError(branch="extract_customer_insights", error=str(error)))


@executor(id="extract_claims_guardrails")
async def extract_claims_guardrails(run: ProductRun, ctx: WorkflowContext[Any]) -> None:
    """Extract usable claims and brand/legal boundaries."""
    try:
        result = await asyncio.to_thread(
            _call_json,
            _branch_prompt("claims_guardrails.jinja2", run),
            ClaimsGuardrails,
        )
        await ctx.send_message(result)
    except Exception as error:
        await ctx.send_message(AnalysisError(branch="extract_claims_guardrails", error=str(error)))


@executor(id="extract_evidence_gaps")
async def extract_evidence_gaps(run: ProductRun, ctx: WorkflowContext[Any]) -> None:
    """Separate documented facts from assumptions and missing evidence."""
    try:
        result = await asyncio.to_thread(
            _call_json,
            _branch_prompt("evidence_gaps.jinja2", run),
            EvidenceGaps,
        )
        await ctx.send_message(result)
    except Exception as error:
        await ctx.send_message(AnalysisError(branch="extract_evidence_gaps", error=str(error)))


def _find_one(results: list[Any], schema: type[T]) -> T:
    matches = [item for item in results if isinstance(item, schema)]
    if len(matches) != 1:
        raise ValueError(f"Expected one {schema.__name__}, got {len(matches)}")
    return matches[0]


@executor(id="synthesize_strategy")
async def synthesize_strategy(results: list[Any], ctx: WorkflowContext[Never, str]) -> None:
    """Join the extraction branches and produce the final product analysis."""
    failures = [item for item in results if isinstance(item, AnalysisError)]
    if failures:
        details = "; ".join(f"{item.branch}: {item.error}" for item in failures)
        raise RuntimeError(f"Не удалось завершить извлечение: {details}")

    passport = _find_one(results, ProductPassport)
    insights = _find_one(results, CustomerInsights)
    guardrails = _find_one(results, ClaimsGuardrails)
    gaps = _find_one(results, EvidenceGaps)

    prompt = _render_prompt(
        "synthesize_strategy.jinja2",
        product_name=passport.product_name,
        passport=json.dumps(passport.model_dump(), ensure_ascii=False, indent=2),
        customer_insights=json.dumps(insights.model_dump(), ensure_ascii=False, indent=2),
        claims_guardrails=json.dumps(guardrails.model_dump(), ensure_ascii=False, indent=2),
        evidence_gaps=json.dumps(gaps.model_dump(), ensure_ascii=False, indent=2),
    )
    strategy = await asyncio.to_thread(_call_json, prompt, StrategyOutput)
    await ctx.yield_output(json.dumps(strategy.model_dump(), ensure_ascii=False, indent=2))


workflow = (
    WorkflowBuilder(start_executor=product_input)
    .add_fan_out_edges(
        product_input,
        [
            extract_product_passport,
            extract_customer_insights,
            extract_claims_guardrails,
            extract_evidence_gaps,
        ],
    )
    .add_fan_in_edges(
        [
            extract_product_passport,
            extract_customer_insights,
            extract_claims_guardrails,
            extract_evidence_gaps,
        ],
        synthesize_strategy,
    )
    .build()
)

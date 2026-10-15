"""Ask three product questions in parallel, then rank matching audiences."""

from __future__ import annotations

import asyncio
import json
import shutil
import subprocess
from pathlib import Path
from typing import Any

from agent_framework import WorkflowBuilder, WorkflowContext, executor
import httpx
from maf_playground.config import PROJECT_ROOT, settings
from pydantic import BaseModel, Field
from typing_extensions import Never

PROMPTS_DIR = PROJECT_ROOT / "prompts"
AUDIENCES_PATH = PROJECT_ROOT / "data" / "audiences.json"

DEFAULT_PRODUCT = (
    "Дебетовая и кредитная карта в одной: кредитный лимит подключается к дебетовой "
    "или зарплатной карте Цифра банка через мобильное приложение."
)


class ProductRequest(BaseModel):
    """Product description submitted as the workflow input."""

    product: str = Field(default=DEFAULT_PRODUCT, description="Описание продукта")


class ProductRun(BaseModel):
    """Product description passed through the workflow."""

    product: str


class QuestionAnswer(BaseModel):
    """Answer from one of the parallel product-question nodes."""

    key: str
    question: str
    answer: str
    product: str


class Audience(BaseModel):
    """Audience profile from the local catalog."""

    id: str
    label: str
    description: str


class AudienceCatalog(BaseModel):
    """The complete set of audience profiles used during ranking."""

    audiences: list[Audience]
    product: str


@executor(id="product_input")
async def product_input(request: ProductRequest, ctx: WorkflowContext[ProductRun]) -> None:
    """Fan out the product description to the question nodes."""
    await ctx.send_message(ProductRun(product=request.product))


def _render_prompt(name: str, **values: str) -> str:
    """Render the small ``{{ name }}`` placeholders used by the source prompts."""
    prompt = (PROMPTS_DIR / name).read_text(encoding="utf-8")
    for key, value in values.items():
        prompt = prompt.replace("{{" + key + "}}", value).replace("{{ " + key + " }}", value)
    return prompt


def _call_openrouter(prompt: str) -> str:
    """Send one independent prompt directly to OpenRouter's OpenAI-compatible API."""
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
        raise RuntimeError("OpenRouter request timed out after 300 seconds") from error
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
    """Run one independent prompt through the signed-in OpenCode CLI."""
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
        raise RuntimeError("OpenCode CLI timed out after 300 seconds") from error
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


@executor(id="answer_problem")
async def answer_problem(request: ProductRun, ctx: WorkflowContext[QuestionAnswer]) -> None:
    """Answer what problem the product solves."""
    question = "Какую основную проблему решает этот продукт? Ответь одним кратким предложением на русском языке."
    try:
        answer = await asyncio.to_thread(
            _call_model,
            _render_prompt("problem.jinja2", product=request.product),
        )
    except Exception as error:
        answer = f"ERROR: {error}"
    await ctx.send_message(
        QuestionAnswer(key="problem_answer", question=question, answer=answer, product=request.product)
    )


@executor(id="answer_primary_customer")
async def answer_primary_customer(request: ProductRun, ctx: WorkflowContext[QuestionAnswer]) -> None:
    """Identify the primary customer and their need."""
    question = "Кто основной клиент этого продукта и что ему нужно? Ответь кратко на русском языке."
    try:
        answer = await asyncio.to_thread(
            _call_model,
            _render_prompt("primary_customer.jinja2", product=request.product),
        )
    except Exception as error:
        answer = f"ERROR: {error}"
    await ctx.send_message(
        QuestionAnswer(key="primary_customer_answer", question=question, answer=answer, product=request.product)
    )


@executor(id="answer_product_value")
async def answer_product_value(request: ProductRun, ctx: WorkflowContext[QuestionAnswer]) -> None:
    """Describe the product's main customer value."""
    question = "В чём главная ценность этого продукта для клиента? Ответь одним кратким предложением на русском языке."
    try:
        answer = await asyncio.to_thread(
            _call_model,
            _render_prompt("product_value.jinja2", product=request.product),
        )
    except Exception as error:
        answer = f"ERROR: {error}"
    await ctx.send_message(
        QuestionAnswer(key="product_value_answer", question=question, answer=answer, product=request.product)
    )


@executor(id="audience_catalog")
async def audience_catalog(request: ProductRun, ctx: WorkflowContext[AudienceCatalog]) -> None:
    """Load the local audience catalog as a visible workflow branch."""
    data = json.loads(AUDIENCES_PATH.read_text(encoding="utf-8"))
    catalog = AudienceCatalog(
        audiences=[Audience.model_validate(item) for item in data],
        product=request.product,
    )
    await ctx.send_message(catalog)


def _validate_ranking(ranking: str, catalog: AudienceCatalog) -> list[dict[str, str]]:
    """Require exactly three distinct IDs that exist in the audience catalog."""
    start = ranking.find("{")
    if start < 0:
        raise ValueError("The model did not return a JSON object")

    try:
        parsed, _ = json.JSONDecoder().raw_decode(ranking[start:])
    except json.JSONDecodeError as error:
        raise ValueError(f"Could not parse ranking JSON: {error}") from error

    ranked = parsed.get("top_audiences")
    if not isinstance(ranked, list) or len(ranked) != 3:
        raise ValueError("The model must return exactly three audiences")

    allowed = {audience.id: audience for audience in catalog.audiences}
    seen: set[str] = set()
    result: list[dict[str, str]] = []
    for item in ranked:
        audience_id = item.get("audience_id")
        if audience_id not in allowed or audience_id in seen:
            raise ValueError("The model returned an unknown or duplicate audience")
        seen.add(audience_id)
        result.append(
            {
                "audience_id": audience_id,
                "label": allowed[audience_id].label,
                "reason": (item.get("reason") or "Обоснование не указано.").strip(),
            }
        )
    return result


@executor(id="rank_audiences")
async def rank_audiences(results: list[Any], ctx: WorkflowContext[Never, str]) -> None:
    """Join question answers and the catalog, then select the top three audiences."""
    answers = {item.key: item for item in results if isinstance(item, QuestionAnswer)}
    catalogs = [item for item in results if isinstance(item, AudienceCatalog)]
    if len(answers) != 3 or len(catalogs) != 1:
        raise ValueError("Expected three question answers and one audience catalog")

    catalog = catalogs[0]
    product = catalog.product
    catalog_json = json.dumps(
        [audience.model_dump() for audience in catalog.audiences],
        ensure_ascii=False,
        indent=2,
    )
    failed_answers = [item for item in answers.values() if item.answer.startswith("ERROR: ")]
    if failed_answers:
        failures = "; ".join(
            f"{item.key}: {item.answer.removeprefix('ERROR: ').strip()}" for item in failed_answers
        )
        raise RuntimeError("Product-question request(s) failed: " + failures)

    prompt = _render_prompt(
        "rank_audiences.jinja2",
        product=product,
        problem_answer=answers["problem_answer"].answer,
        primary_customer_answer=answers["primary_customer_answer"].answer,
        product_value_answer=answers["product_value_answer"].answer,
        audience_catalog=catalog_json,
    )
    ranking = await asyncio.to_thread(_call_model, prompt)
    ranked_audiences = _validate_ranking(ranking, catalog)

    output = {
        "product": product,
        "problem_answer": answers["problem_answer"].answer,
        "primary_customer_answer": answers["primary_customer_answer"].answer,
        "product_value_answer": answers["product_value_answer"].answer,
        "top_audiences": ranked_audiences,
    }
    await ctx.yield_output(json.dumps(output, ensure_ascii=False, indent=2))


workflow = (
    WorkflowBuilder(start_executor=product_input)
    .add_fan_out_edges(
        product_input,
        [answer_problem, answer_primary_customer, answer_product_value, audience_catalog],
    )
    .add_fan_in_edges(
        [answer_problem, answer_primary_customer, answer_product_value, audience_catalog],
        rank_audiences,
    )
    .build()
)

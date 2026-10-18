"""Generate an inspection-oriented draw.io map of the product evaluation pipeline."""

from __future__ import annotations

import ast
import html
import xml.etree.ElementTree as ET
from datetime import UTC, datetime
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
EVAL = ROOT / "eval"
OUT = EVAL / "reports" / "product-pipeline-prompts.drawio"


def read(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def source_functions(path: Path, names: list[str], class_name: str | None = None) -> str:
    """Return exact source for selected functions/methods, preserving current code."""

    source = read(path)
    tree = ast.parse(source, filename=str(path))
    definitions: list[ast.AST] = []
    owners = [tree]
    if class_name:
        owners = [
            node
            for node in ast.walk(tree)
            if isinstance(node, ast.ClassDef) and node.name == class_name
        ]
    for owner in owners:
        definitions.extend(
            node
            for node in getattr(owner, "body", [])
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and node.name in names
        )
    by_name = {node.name: node for node in definitions}  # type: ignore[attr-defined]
    chunks: list[str] = []
    for name in names:
        node = by_name.get(name)
        if node is None:
            continue
        segment = ast.get_source_segment(source, node)
        if segment:
            chunks.append(f"# {path.relative_to(ROOT)} :: {name}\n{segment}")
    return "\n\n".join(chunks)


def add_cell(root: ET.Element, cell_id: str, label: str, style: str, *, x: int, y: int, width: int, height: int, parent: str = "1", link: str | None = None) -> None:
    attributes = {"id": cell_id, "value": label, "style": style, "vertex": "1", "parent": parent}
    if link:
        attributes["link"] = link
    cell = ET.SubElement(root, "mxCell", attributes)
    ET.SubElement(cell, "mxGeometry", {"x": str(x), "y": str(y), "width": str(width), "height": str(height), "as": "geometry"})


def add_edge(root: ET.Element, cell_id: str, source: str, target: str, *, label: str = "") -> None:
    cell = ET.SubElement(root, "mxCell", {"id": cell_id, "value": label, "edge": "1", "parent": "1", "source": source, "target": target, "style": "edgeStyle=orthogonalEdgeStyle;rounded=0;orthogonalLoop=1;jettySize=auto;html=1;"})
    ET.SubElement(cell, "mxGeometry", {"relative": "1", "as": "geometry"})


def model() -> ET.Element:
    return ET.Element("mxGraphModel", {"dx": "1600", "dy": "900", "grid": "1", "page": "1", "pageScale": "1", "background": "#ffffff", "math": "0", "shadow": "0"})


def page(title: str) -> tuple[ET.Element, ET.Element]:
    graph = model()
    root = ET.SubElement(graph, "root")
    ET.SubElement(root, "mxCell", {"id": "0"})
    ET.SubElement(root, "mxCell", {"id": "1", "parent": "0"})
    add_cell(root, "title", f"<b>{html.escape(title)}</b>", "text;html=1;strokeColor=none;fillColor=none;align=left;verticalAlign=middle;fontSize=24;fontStyle=1;", x=30, y=20, width=1500, height=45)
    return graph, root


def box_label(title: str, body: str, *, color: str = "#dae8fc") -> str:
    return (
        f"<div style='font-size:16px'><b>{html.escape(title)}</b></div>"
        f"<div style='font-size:12px;margin-top:8px'>{html.escape(body).replace(chr(10), '<br>')}</div>"
    )


def add_overview() -> ET.Element:
    graph, root = page("Карта product-пайплайна Tempo — полная логика")
    add_cell(root, "note", box_label("Как читать схему", "Синие узлы — вызовы модели; жёлтые — детерминированная логика; зелёные — сохранённые результаты. Нажмите узел или выберите страницу prompt в списке страниц слева: там полный текст разбит на читаемые части."), "rounded=1;whiteSpace=wrap;html=1;fillColor=#fff2cc;strokeColor=#d6b656;", x=30, y=80, width=1500, height=75)
    nodes = [
        ("sources", "ВХОДНЫЕ ДАННЫЕ\nфайлы источников", "n8n/product/<product>/*.md\n--product cafe|bank выбирает набор; load_sources() сортирует файлы. README.md исключается.", 40, 220, 220, 100, "fillColor=#d5e8d4;strokeColor=#82b366;"),
        ("fanout", "START → fan_out_sources", "LangGraph Send создаёт отдельный extract_source для каждого источника.\nЗадачи выполняются параллельно, drafts объединяются через operator.add.", 310, 220, 250, 100, "fillColor=#fff2cc;strokeColor=#d6b656;"),
        ("extract", "МОДЕЛЬ: extract_source", "Один structured-вызов Passport на источник.\nPrompt: tempo-product-extraction.md\nСхема ответа: Passport", 610, 220, 250, 100, "fillColor=#dae8fc;strokeColor=#6c8ebf;"),
        ("drafts", "ЧЕРНОВИКИ", "Draft(source, result) для каждого источника.\nКаждый draft сохраняет локальный паспорт и evidence.", 910, 220, 220, 100, "fillColor=#d5e8d4;strokeColor=#82b366;"),
        ("consolidate", "МОДЕЛЬ: consolidate", "Объединяет только подтверждённые факты; сохраняет evidence, conflicts, gaps и условия.\nПеренумеровывает records r1… и сортирует по category.", 1180, 220, 270, 100, "fillColor=#dae8fc;strokeColor=#6c8ebf;"),
        ("validate", "ДЕТЕРМИНИРОВАННО: validate", "Проверяет последовательные ID записей, statements, evidence, source/quote и ссылки конфликтов.", 1180, 390, 270, 95, "fillColor=#fff2cc;strokeColor=#d6b656;"),
        ("decision", "ВАЛИДНО?", "valid и repair ещё не выполнялся → finalize\ninvalid → repair\nrepair уже был → finalize", 900, 390, 220, 95, "rhombus;whiteSpace=wrap;html=1;fillColor=#fff2cc;strokeColor=#d6b656;"),
        ("repair", "МОДЕЛЬ: repair (условно)", "Получает только passport и validation.errors.\nМожет исправить структуру, но не добавляет факты и не удаляет evidence.", 610, 390, 250, 95, "fillColor=#dae8fc;strokeColor=#6c8ebf;"),
        ("finalize", "ДЕТЕРМИНИРОВАННО: finalize", "Повторно запускает validate_passport().\nФормирует финальные Passport + Validation.", 310, 390, 250, 95, "fillColor=#fff2cc;strokeColor=#d6b656;"),
        ("evaluate", "МОДЕЛЬ: evaluate", "Аудит источников по полному evaluation prompt.\nПолучает passport, validation и исходные материалы.\nРезультат: Evaluation.", 40, 390, 220, 95, "fillColor=#dae8fc;strokeColor=#6c8ebf;"),
        ("result", "СОХРАНЁННЫЙ РЕЗУЛЬТАТ", "runner инкрементально пишет JSON: status, duration, calls, token counts, checkpoint, passport, validation и evaluation.", 40, 570, 300, 105, "fillColor=#d5e8d4;strokeColor=#82b366;"),
        ("benchmark", "BENCHMARK × 3 ЗАПУСКА", "Run 1: обычный порядок источников\nRun 2: обратный порядок\nRun 3: детерминированный shuffle Random(42)\nПосле 3 паспортов: C9 stability evaluator.", 430, 570, 340, 105, "fillColor=#f8cecc;strokeColor=#b85450;"),
        ("stability", "МОДЕЛЬ: C9 stability evaluator", "Сравнивает паспорта, validation, условия, числа, evidence, conflicts, gaps и границы продукта.\nРезультат: StabilityEvaluation.", 860, 570, 310, 105, "fillColor=#dae8fc;strokeColor=#6c8ebf;"),
        ("handoff", "ДЕТЕРМИНИРОВАННАЯ ПЕРЕДАЧА", "load_product_passport() выбирает минимальный run_number среди завершённых.\ncompact_product_passport() передаёт records и product_jtbd в audience pipeline.", 1260, 570, 250, 105, "fillColor=#d5e8d4;strokeColor=#82b366;"),
    ]
    for cell_id, title, body, x, y, width, height, style in nodes:
        add_cell(root, cell_id, box_label(title, body), f"rounded=1;whiteSpace=wrap;html=1;{style}", x=x, y=y, width=width, height=height)
    page_links = {
        "extract": "data:page/id,extraction-prompt-1",
        "consolidate": "data:page/id,consolidation-1",
        "validate": "data:page/id,validation-repair-1",
        "repair": "data:page/id,validation-repair-1",
        "evaluate": "data:page/id,evaluation-prompt-1",
        "benchmark": "data:page/id,benchmark-1",
        "stability": "data:page/id,benchmark-1",
        "handoff": "data:page/id,orchestration-1",
    }
    for cell in root:
        if cell.attrib.get("id") in page_links:
            cell.set("link", page_links[cell.attrib["id"]])
    edges = [
        ("sources", "fanout", ""), ("fanout", "extract", "parallel"), ("extract", "drafts", ""), ("drafts", "consolidate", ""),
        ("consolidate", "validate", ""), ("validate", "decision", ""), ("decision", "repair", "invalid"), ("decision", "finalize", "valid / repaired"),
        ("repair", "finalize", ""), ("finalize", "evaluate", ""), ("evaluate", "result", ""), ("result", "benchmark", "run slots"),
        ("benchmark", "stability", "3 completed"), ("stability", "handoff", "selected run 1 when used downstream"),
    ]
    for index, (source, target, label) in enumerate(edges, start=1):
        add_edge(root, f"edge{index}", source, target, label=label)
    return graph


def add_text_pages(page_prefix: str, title: str, subtitle: str, content: str, *, chunk_lines: int = 40) -> list[tuple[str, str, ET.Element]]:
    lines = content.splitlines() or [""]
    chunks = [lines[index:index + chunk_lines] for index in range(0, len(lines), chunk_lines)]
    pages: list[tuple[str, str, ET.Element]] = []
    for index, chunk in enumerate(chunks, start=1):
        graph, root = page(f"{title} — часть {index}/{len(chunks)}")
        add_cell(root, "subtitle", html.escape(subtitle).replace("\n", "<br>"), "text;html=1;strokeColor=none;fillColor=none;align=left;verticalAlign=middle;fontSize=13;fontColor=#666666;", x=30, y=70, width=1500, height=45)
        body = "\n".join(chunk)
        label = f"<div style='font-size:16px'><b>Часть {index}/{len(chunks)}</b></div><pre style='font-size:14px;white-space:pre-wrap;margin-top:8px'>{html.escape(body)}</pre>"
        add_cell(root, f"part{index}", label, "rounded=1;whiteSpace=wrap;html=1;fillColor=#f5f5f5;strokeColor=#999999;align=left;verticalAlign=top;spacing=10;overflow=hidden;", x=30, y=125, width=1500, height=760)
        pages.append((f"{page_prefix}-{index}", f"{title} — часть {index}/{len(chunks)}", graph))
    return pages


def add_page(root: ET.Element, graph: ET.Element, page_id: str, name: str) -> None:
    diagram = ET.SubElement(root, "diagram", {"id": page_id, "name": name})
    diagram.append(graph)


def main() -> None:
    root = ET.Element("mxfile", {"host": "app.diagrams.net", "modified": datetime.now(UTC).isoformat(), "agent": "OpenCode", "version": "24.7.17", "type": "device"})
    pages: list[tuple[str, str, ET.Element]] = [("overview", "00 Обзор пайплайна", add_overview())]
    prompt_dir = EVAL / "tempo_eval" / "prompts"
    pages.extend(add_text_pages("extraction-prompt", "01 Полный prompt экстракции", "Точное содержимое eval/tempo_eval/prompts/tempo-product-extraction.md. Placeholder подставляется отдельно для каждого источника.", read(prompt_dir / "tempo-product-extraction.md")))
    pages.extend(add_text_pages("consolidation", "02 Consolidation: логика и prompt", "Точный код Workflow, который объединяет drafts и вызывает модель consolidation.", source_functions(EVAL / "tempo_eval" / "workflow.py", ["compile", "fan_out_sources", "extract_source", "consolidate"], "Workflow")))
    pages.extend(add_text_pages("validation-repair", "03 Validation, repair, finalize", "Детерминированные проверки структуры, условный repair prompt и финальная проверка.", source_functions(EVAL / "tempo_eval" / "workflow.py", ["validate", "route_validation", "repair", "finalize"], "Workflow") + "\n\n" + source_functions(EVAL / "tempo_eval" / "workflow.py", ["validate_passport"])))
    pages.extend(add_text_pages("evaluation-prompt", "04 Полный prompt аудита результата", "Точное содержимое eval/tempo_eval/prompts/tempo-product-extraction-evaluation.md.", read(prompt_dir / "tempo-product-extraction-evaluation.md")))
    pages.extend(add_text_pages("benchmark", "05 Benchmark × 3 и C9 prompt", "Точная оркестрация benchmark, политика порядка источников и template prompt stability evaluator.", source_functions(EVAL / "benchmark.py", ["run_benchmark", "run_model_slots", "evaluate_stability", "product_benchmark_prompt", "sources_for_run"]) + "\n\n# Политика benchmark\n" + read(EVAL / "README.md").split("## Run the C9 benchmark", 1)[1].split("##", 1)[0]))
    pages.extend(add_text_pages("runtime", "06 Runtime, retries, checkpoints", "Точная логика runner и model client: structured JSON, correction retries, heartbeat, metrics, persistence и выбор паспорта.", source_functions(EVAL / "tempo_eval" / "client.py", ["complete_structured", "correction_prompt", "strict_schema"]) + "\n\n" + source_functions(EVAL / "tempo_eval" / "runner.py", ["run_models", "run_one_model", "load_product_passport", "compact_product_passport"])))
    pages.extend(add_text_pages("orchestration", "07 Оркестрация и передача результата", "Product-фаза one-model pipeline и детерминированная передача product passport в audience.", source_functions(EVAL / "one_model_pipeline.py", ["run_pipeline"]) + "\n\n# Контракт product → audience\n" + "Product run завершается первым. load_product_passport() выбирает минимальный завершённый run_number. compact_product_passport() передаёт product_name, extraction_status, records, product_jtbd, product_jtbd_status и warnings в audience workflow."))
    for page_id, name, graph in pages:
        add_page(root, graph, page_id, name)
    OUT.parent.mkdir(parents=True, exist_ok=True)
    ET.ElementTree(root).write(OUT, encoding="utf-8", xml_declaration=True)
    print(OUT)


if __name__ == "__main__":
    main()

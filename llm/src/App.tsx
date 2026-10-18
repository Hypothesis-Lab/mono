import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Background,
  Controls,
  Handle,
  MarkerType,
  MiniMap,
  Position,
  ReactFlow,
  applyNodeChanges,
  type Edge,
  type Node,
  type NodeChange,
  type NodeProps,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import {
  Answer,
  AppShell,
  Brand,
  BrandMark,
  Eyebrow,
  GlobalStyle,
  GraphActions,
  GraphHandle,
  GraphLegend,
  GraphNode as StyledGraphNode,
  GraphPanel,
  GraphZoomIndicator,
  LegendDot,
  NodeEyebrow,
  NodeStatus,
  NodeTitle,
  PipelineButton,
  PipelineDot,
  ProductDocsButton,
  ProductDocsHeader,
  ProductDocsPanel,
  ProductDocument,
  ProductDocumentBody,
  ProductDocumentSummary,
  ProductInput,
  PromptEditor,
  PromptEditorActions,
  PromptSaveButton,
  PromptSaveStatus,
  QuestionCopy,
  QuestionEdgeStyles,
  QuestionHeader,
  RunButton,
  Sidebar,
  SidebarFooter,
  SidebarLabel,
  StatusDot,
  Topbar,
  Workspace,
} from './styles'
import { DEFAULT_PROMPTS, type PromptId } from './prompts'

const MODEL = 'cat-jam-free'
const PRODUCT_NAME = 'Tempo'
const DEFAULT_FOCUS = 'Проведи полный анализ продукта Tempo и подготовь основу для коммуникаций.'
let promptTemplates: Record<PromptId, string> = { ...DEFAULT_PROMPTS }

type NodeStatus = 'idle' | 'running' | 'complete' | 'error'
type StageKind = 'questions' | 'extractor' | 'critic' | 'validator' | 'output'
type StageId = 'questions' | 'parser' | 'brand' | 'insights' | 'critic' | 'validator' | 'output'

type ProductNodeData = {
  label: string
  value: string
  onChange: (value: string) => void
}

type ProductDataNodeData = {
  label: string
  files: string[]
  loaded: boolean
  onOpenDocuments: () => void
}

type AnalysisNodeData = {
  label: string
  description: string
  status: NodeStatus
  kind: StageKind
  promptId: PromptId
  prompt: string
  promptStatus: 'saved' | 'dirty' | 'saving' | 'error'
  onPromptChange: (value: string) => void
  onPromptSave: () => void
  answer?: string
  error?: string
}

type ProductNode = Node<ProductNodeData, 'product'>
type ProductDataNode = Node<ProductDataNodeData, 'product-data'>
type AnalysisNode = Node<AnalysisNodeData, 'analysis'>
type GraphNode = ProductNode | ProductDataNode | AnalysisNode
type GraphEdge = Edge<{ stage?: StageId }>

type ChatResponse = {
  choices?: Array<{ message?: { content?: string | null } }>
  error?: { message?: string }
}

type ProductContext = {
  productName: string
  files: Array<{ name: string; content: string }>
  context: string
}

type PromptRecord = {
  id: PromptId
  content: string
  updatedAt: string
}

type StageDefinition = {
  id: StageId
  nodeId: string
  label: string
  description: string
  kind: StageKind
  position: { x: number; y: number }
}

const stageDefinitions: StageDefinition[] = [
  {
    id: 'questions',
    nodeId: 'product-questions',
    label: 'Вопросы к продукту',
    description: 'Что за продукт, для кого, какие задачи, выгоды, ограничения, экономика и каналы.',
    kind: 'questions',
    position: { x: 650, y: 430 },
  },
  {
    id: 'parser',
    nodeId: 'extract-product-facts',
    label: 'Извлечение фактов продукта',
    description: 'Извлекает факты, функции, условия, экономику и обязательные поля.',
    kind: 'extractor',
    position: { x: 0, y: 1050 },
  },
  {
    id: 'brand',
    nodeId: 'extract-brand-constraints',
    label: 'Извлечение бренда и ограничений',
    description: 'Разбирает допустимые claims, терминологию, брендовые и юридические границы.',
    kind: 'extractor',
    position: { x: 600, y: 1050 },
  },
  {
    id: 'insights',
    nodeId: 'extract-product-insights',
    label: 'Извлечение продуктовых инсайтов',
    description: 'Извлекает JTBD, боли, ценности, отличия, сегменты и измеримые события.',
    kind: 'extractor',
    position: { x: 1200, y: 1050 },
  },
  {
    id: 'critic',
    nodeId: 'product-critics',
    label: 'Критики',
    description: 'Проверяют достоверность фактов, полноту, противоречия и соответствие бренду.',
    kind: 'critic',
    position: { x: 600, y: 1700 },
  },
  {
    id: 'validator',
    nodeId: 'product-validators',
    label: 'Валидаторы',
    description: 'Проверяют источники, обязательные поля, legal/brand rules, sensitive data и ссылки.',
    kind: 'validator',
    position: { x: 600, y: 2350 },
  },
  {
    id: 'output',
    nodeId: 'product-passport',
    label: 'Паспорт продукта',
    description: 'Факты, выводы, ограничения, экономика, claims, неизвестные поля и источники.',
    kind: 'output',
    position: { x: 545, y: 3000 },
  },
]

const outputSchemas: Record<StageId, string> = {
  questions: `{
  "product_and_audience": {"answer":"...","sources":["product-brief.md"]},
  "user_problem": {"answer":"...","sources":["product-brief.md"]},
  "confirmed_benefits": [{"text":"...","sources":["product-brief.md"]}],
  "differentiating_properties": [{"text":"...","sources":["product-brief.md"]}],
  "mandatory_conditions_and_limits": [{"text":"...","sources":["policies.md"]}],
  "allowed_promises": [{"text":"...","sources":["brand-book.md"]}],
  "forbidden_promises": [{"text":"...","sources":["brand-book.md"]}],
  "economics": {"cpa":"UNKNOWN","cac":"UNKNOWN","target_conversion":"UNKNOWN","sources":[]},
  "measurable_business_events": [{"text":"...","sources":["api-reference.md"]}],
  "allowed_channels": [{"text":"UNKNOWN","sources":[]}],
  "unknown_fields": [{"field":"...","why_it_is_unknown":"..."}]
}`,
  parser: `{
  "facts": [{"text":"...","status":"FACT","sources":["product-brief.md"]}],
  "capabilities": [{"text":"...","sources":["features-and-pricing.md"]}],
  "limitations": [{"text":"...","sources":["product-brief.md"]}],
  "economics": [{"field":"CPA","value":"UNKNOWN","sources":[]}],
  "mandatory_fields": [{"field":"...","value":"...","sources":["product-brief.md"]}],
  "unknown_fields": [{"field":"...","reason":"..."}],
  "source_links": [{"claim":"...","sources":["product-brief.md"]}]
}`,
  brand: `{
  "approved_claims": [{"text":"...","caveat":"...","sources":["brand-book.md"]}],
  "forbidden_claims": [{"text":"...","reason":"...","safe_alternative":"...","sources":["brand-book.md"]}],
  "brand_rules": [{"text":"...","sources":["brand-book.md"]}],
  "legal_rules": [{"text":"...","sources":["policies.md"]}],
  "terminology_rules": [{"text":"...","sources":["glossary.md"]}],
  "sensitive_data_rules": [{"text":"...","sources":["policies.md"]}]
}`,
  insights: `{
  "jobs_to_be_done": [{"text":"...","sources":["product-brief.md"]}],
  "pain_points": [{"text":"...","sources":["product-brief.md"]}],
  "values_and_outcomes": [{"text":"...","sources":["product-brief.md"]}],
  "differentiators": [{"text":"...","sources":["product-brief.md"]}],
  "target_segments": [{"text":"...","sources":["product-brief.md"]}],
  "measurable_events": [{"text":"...","sources":["api-reference.md"]}],
  "hypotheses": [{"text":"...","sources":["product-brief.md"]}]
}`,
  critic: `{
  "verdict":"PASS или NEEDS_REVIEW",
  "factuality_issues": [{"issue":"...","severity":"...","sources":["product-brief.md"]}],
  "completeness_issues": [{"issue":"...","missing_field":"..."}],
  "contradictions": [{"issue":"...","sources":["product-brief.md","website-copy.md"]}],
  "brand_fit_issues": [{"issue":"...","sources":["brand-book.md"]}],
  "required_fixes": ["..."]
}`,
  validator: `{
  "status":"VALID или INVALID",
  "source_issues": [{"issue":"...","fix":"..."}],
  "mandatory_field_issues": [{"field":"...","issue":"..."}],
  "brand_legal_issues": [{"issue":"...","sources":["policies.md"]}],
  "sensitive_data_issues": [{"issue":"...","sources":["policies.md"]}],
  "source_link_issues": [{"claim":"...","issue":"..."}],
  "required_fixes": ["..."]
}`,
  output: `{
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

function initialNodes(onFocusChange: (value: string) => void): GraphNode[] {
  return [
    {
      id: 'product-data',
      type: 'product-data',
      position: { x: 90, y: 30 },
      data: { label: 'Документы продукта', files: [], loaded: false, onOpenDocuments: () => undefined },
    },
    {
      id: 'analysis-focus',
      type: 'product',
      position: { x: 760, y: 30 },
      data: { label: 'Фокус анализа', value: DEFAULT_FOCUS, onChange: onFocusChange },
    },
    ...stageDefinitions.map((definition) => ({
      id: definition.nodeId,
      type: 'analysis' as const,
      position: definition.position,
      data: {
        label: definition.label,
        description: definition.description,
        status: 'idle' as const,
        kind: definition.kind,
        promptId: definition.id,
        prompt: '',
        promptStatus: 'saved' as const,
        onPromptChange: () => undefined,
        onPromptSave: () => undefined,
      },
    })),
  ]
}

const initialEdges: GraphEdge[] = [
  { id: 'product-to-questions', source: 'product-data', target: 'product-questions', type: 'smoothstep', markerEnd: { type: MarkerType.ArrowClosed, color: '#74815a' } },
  { id: 'focus-to-questions', source: 'analysis-focus', target: 'product-questions', type: 'smoothstep', markerEnd: { type: MarkerType.ArrowClosed, color: '#74815a' } },
  ...['extract-product-facts', 'extract-brand-constraints', 'extract-product-insights'].map((target) => ({
    id: `questions-to-${target}`,
    source: 'product-questions',
    target,
    type: 'smoothstep',
    markerEnd: { type: MarkerType.ArrowClosed, color: '#74815a' },
  })),
  ...['extract-product-facts', 'extract-brand-constraints', 'extract-product-insights'].map((source) => ({
    id: `${source}-to-critic`,
    source,
    target: 'product-critics',
    type: 'smoothstep',
    markerEnd: { type: MarkerType.ArrowClosed, color: '#74815a' },
  })),
  { id: 'critic-to-validator', source: 'product-critics', target: 'product-validators', type: 'smoothstep', markerEnd: { type: MarkerType.ArrowClosed, color: '#74815a' } },
  { id: 'validator-to-output', source: 'product-validators', target: 'product-passport', type: 'smoothstep', markerEnd: { type: MarkerType.ArrowClosed, color: '#74815a' } },
]

async function loadProductContext(): Promise<ProductContext> {
  const product = new URLSearchParams(window.location.search).get('product') ?? 'cafe'
  const response = await fetch(`/api/product-context?product=${encodeURIComponent(product)}`)
  const body = (await response.json()) as ProductContext & { error?: { message?: string } }
  if (!response.ok) throw new Error(body.error?.message ?? `Не удалось загрузить документы продукта (${response.status})`)
  return body
}

async function loadPrompts(): Promise<Record<PromptId, string>> {
  const response = await fetch('/api/prompts')
  const body = (await response.json()) as { prompts?: PromptRecord[]; error?: { message?: string } }
  if (!response.ok || !body.prompts) throw new Error(body.error?.message ?? `Не удалось загрузить промпты (${response.status})`)
  return body.prompts.reduce((result, prompt) => {
    result[prompt.id] = prompt.content
    return result
  }, { ...DEFAULT_PROMPTS })
}

async function savePrompt(id: PromptId, content: string): Promise<void> {
  const response = await fetch('/api/prompts', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id, content }),
  })
  const body = (await response.json()) as { error?: { message?: string } }
  if (!response.ok) throw new Error(body.error?.message ?? `Не удалось сохранить промпт (${response.status})`)
}

async function callOpenCode(prompt: string): Promise<string> {
  const response = await fetch('/api/opencode', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ model: MODEL, messages: [{ role: 'user', content: prompt }] }),
  })
  const body = (await response.json()) as ChatResponse
  if (!response.ok) throw new Error(body.error?.message ?? `OpenCode returned HTTP ${response.status}`)
  const content = body.choices?.[0]?.message?.content?.trim()
  if (!content) throw new Error('OpenCode returned an empty response')
  return content
}

function parseJsonObject(content: string): Record<string, unknown> {
  const start = content.indexOf('{')
  const end = content.lastIndexOf('}')
  if (start < 0 || end <= start) throw new Error('Модель не вернула JSON-объект')
  try {
    return JSON.parse(content.slice(start, end + 1)) as Record<string, unknown>
  } catch (error) {
    throw new Error(`Не удалось разобрать JSON модели: ${error instanceof Error ? error.message : 'unknown error'}`)
  }
}

function validateSourceNames(value: unknown, sourceNames: Set<string>): void {
  if (Array.isArray(value)) {
    value.forEach((item) => validateSourceNames(item, sourceNames))
    return
  }
  if (!value || typeof value !== 'object') return
  Object.entries(value).forEach(([key, nested]) => {
    if (key === 'sources') {
      if (!Array.isArray(nested) || nested.some((source) => typeof source !== 'string' || !sourceNames.has(source))) {
        throw new Error('Модель сослалась на неизвестный файл в product/')
      }
      return
    }
    validateSourceNames(nested, sourceNames)
  })
}

function parseAndValidate(content: string, sourceNames: Set<string>): Record<string, unknown> {
  const parsed = parseJsonObject(content)
  validateSourceNames(parsed, sourceNames)
  return parsed
}

function sourceRules(sourceFiles: string): string {
  return `
Общие правила:
1. Отвечай по-русски, но ключи JSON оставь ровно как в схеме.
2. Используй только материалы продукта. Не додумывай факты, функции, экономику, каналы или результаты.
3. Если данных нет, используй строку "UNKNOWN" и добавь поле в unknown_fields. Не подставляй примерные значения.
4. Каждая запись с sources должна содержать точное имя файла из этого списка: ${sourceFiles}
5. Разделяй FACT, ВЫВОД ИИ и UNKNOWN.
6. Не превращай маркетинговый текст в гарантию.
`
}

function renderPrompt(template: string, values: Record<string, string>): string {
  return Object.entries(values).reduce((prompt, [key, value]) => prompt.split(`{{${key}}}`).join(value), template)
}

function questionsPrompt(focus: string, context: ProductContext): string {
  return renderPrompt(promptTemplates.questions, {
    FOCUS: focus,
    PRODUCT_CONTEXT: context.context,
    SOURCE_FILES: context.files.map((file) => file.name).join(', '),
  })
}

function extractorPrompt(agent: 'parser' | 'brand' | 'insights', focus: string, context: ProductContext, questions: Record<string, unknown>): string {
  if (promptTemplates[agent]) return renderPrompt(promptTemplates[agent], {
    FOCUS: focus,
    QUESTIONS: JSON.stringify(questions, null, 2),
    PRODUCT_CONTEXT: context.context,
    SOURCE_FILES: context.files.map((file) => file.name).join(', '),
  })
  const instructions = {
    parser: 'Ты экстрактор фактов продукта. Извлеки паспортные факты, функции, ограничения, экономику, обязательные поля и ссылки на источники.',
    brand: 'Ты экстрактор бренда и ограничений. Извлеки разрешённые и запрещённые claims, брендовые, юридические, терминологические и sensitive-data правила.',
    insights: 'Ты экстрактор продуктовых инсайтов. Извлеки JTBD, боли, ценности, реальные отличия, подтверждённые сегменты, измеримые события и гипотезы.',
  }[agent]
  return `${instructions}

Фокус запроса:
${focus}

Ответы агента вопросов:
${JSON.stringify(questions, null, 2)}

Материалы продукта:
${context.context}
${sourceRules(context.files.map((file) => file.name).join(', '))}
Верни только JSON такого вида:
${outputSchemas[agent]}`
}

function criticPrompt(focus: string, context: ProductContext, questions: Record<string, unknown>, extractors: Record<string, unknown>): string {
  if (promptTemplates.critic) return renderPrompt(promptTemplates.critic, {
    FOCUS: focus,
    QUESTIONS: JSON.stringify(questions, null, 2),
    EXTRACTORS: JSON.stringify(extractors, null, 2),
    PRODUCT_CONTEXT: context.context,
    SOURCE_FILES: context.files.map((file) => file.name).join(', '),
  })
  return `Ты независимый критик продуктового паспорта ${PRODUCT_NAME}. Проверь результаты экстракторов.

Фокус запроса:
${focus}

Ответы на вопросы:
${JSON.stringify(questions, null, 2)}

Извлечения агентов:
${JSON.stringify(extractors, null, 2)}

Материалы продукта:
${context.context}
${sourceRules(context.files.map((file) => file.name).join(', '))}
Проверь отдельно: достоверность фактов, полноту и обязательные поля, противоречия между источниками, соответствие бренду и отсутствие выдуманных обещаний. Верни только JSON:
${outputSchemas.critic}`
}

function validatorPrompt(focus: string, context: ProductContext, extractors: Record<string, unknown>, critic: Record<string, unknown>): string {
  if (promptTemplates.validator) return renderPrompt(promptTemplates.validator, {
    FOCUS: focus,
    EXTRACTORS: JSON.stringify(extractors, null, 2),
    CRITIC: JSON.stringify(critic, null, 2),
    PRODUCT_CONTEXT: context.context,
    SOURCE_FILES: context.files.map((file) => file.name).join(', '),
  })
  return `Ты строгий валидатор результата анализа продукта ${PRODUCT_NAME}.

Фокус запроса:
${focus}

Результаты агентов:
${JSON.stringify(extractors, null, 2)}

Отчёт критика:
${JSON.stringify(critic, null, 2)}

Материалы продукта:
${context.context}
${sourceRules(context.files.map((file) => file.name).join(', '))}
Проверь наличие источника у каждого факта, обязательные поля, brand/legal rules, sensitive data, корректность source links и явную маркировку UNKNOWN. Ничего не исправляй молча — перечисли проблемы и required_fixes. Верни только JSON:
${outputSchemas.validator}`
}

function outputPrompt(focus: string, context: ProductContext, questions: Record<string, unknown>, extractors: Record<string, unknown>, critic: Record<string, unknown>, validator: Record<string, unknown>): string {
  if (promptTemplates.output) return renderPrompt(promptTemplates.output, {
    FOCUS: focus,
    QUESTIONS: JSON.stringify(questions, null, 2),
    EXTRACTORS: JSON.stringify(extractors, null, 2),
    CRITIC: JSON.stringify(critic, null, 2),
    VALIDATOR: JSON.stringify(validator, null, 2),
    PRODUCT_CONTEXT: context.context,
    SOURCE_FILES: context.files.map((file) => file.name).join(', '),
  })
  return `Ты ведущий продуктовый стратег. Собери финальный паспорт продукта ${PRODUCT_NAME} только из проверенных материалов.

Фокус запроса:
${focus}

Ответы на вопросы:
${JSON.stringify(questions, null, 2)}

Извлечения агентов:
${JSON.stringify(extractors, null, 2)}

Критика:
${JSON.stringify(critic, null, 2)}

Валидация:
${JSON.stringify(validator, null, 2)}

Материалы продукта:
${context.context}
${sourceRules(context.files.map((file) => file.name).join(', '))}
Если валидатор нашёл проблему, не скрывай её: отрази её в critic_and_validator_summary или unknown_fields. Статус каждого элемента должен быть FACT, ВЫВОД ИИ или UNKNOWN. Верни только JSON:
${outputSchemas.output}`
}

function AnalysisNode({ data }: NodeProps<AnalysisNode>) {
  return (
    <StyledGraphNode $kind={data.kind} $status={data.status}>
      <Handle type="target" position={Position.Top}><GraphHandle /></Handle>
      <Handle type="source" position={Position.Bottom}><GraphHandle /></Handle>
      <QuestionHeader>
        <NodeEyebrow>{data.kind === 'critic' ? 'ПРОВЕРКА' : data.kind === 'validator' ? 'ВАЛИДАЦИЯ' : data.kind === 'extractor' ? 'ИЗВЛЕЧЕНИЕ' : 'ВОПРОСЫ'}</NodeEyebrow>
        <NodeStatus $status={data.status}>{data.status}</NodeStatus>
      </QuestionHeader>
      <NodeTitle>{data.label}</NodeTitle>
      <QuestionCopy>{data.description}</QuestionCopy>
      <PromptEditor
        className="nodrag"
        value={data.prompt}
        onChange={(event) => data.onPromptChange(event.target.value)}
        onBlur={data.onPromptSave}
        spellCheck="false"
        aria-label={`Промпт: ${data.label}`}
      />
      <PromptEditorActions>
        <PromptSaveStatus $status={data.promptStatus}>
          {data.promptStatus === 'saving' ? 'Сохраняется…' : data.promptStatus === 'dirty' ? 'Есть изменения' : data.promptStatus === 'error' ? 'Ошибка сохранения' : 'Сохранено в SQLite'}
        </PromptSaveStatus>
        <PromptSaveButton className="nodrag" type="button" onClick={(event) => { event.stopPropagation(); data.onPromptSave() }} disabled={data.promptStatus !== 'dirty'}>
          Сохранить
        </PromptSaveButton>
      </PromptEditorActions>
      <Answer $visible={Boolean(data.answer || data.error)} $error={Boolean(data.error)} aria-live="polite">
        {data.answer ?? data.error ?? (data.status === 'running' ? 'Выполняется…' : 'Ожидает запуска')}
      </Answer>
    </StyledGraphNode>
  )
}

function ProductDataNode({ data }: NodeProps<ProductDataNode>) {
  return (
    <StyledGraphNode $kind="product-data">
      <Handle type="source" position={Position.Bottom}><GraphHandle /></Handle>
      <NodeEyebrow $kind="product">ИСТОЧНИК ДАННЫХ</NodeEyebrow>
      <NodeTitle>{data.label}</NodeTitle>
      <QuestionCopy>{data.loaded ? `${data.files.length} файлов Tempo загружено в контекст.` : 'Загрузка файлов продукта…'}</QuestionCopy>
      <Answer $visible={data.loaded}>{data.loaded ? data.files.join('\n') : 'Ожидает загрузки'}</Answer>
      <ProductDocsButton className="nodrag" type="button" onClick={(event) => { event.stopPropagation(); data.onOpenDocuments() }} disabled={!data.loaded}>
        Открыть документы
      </ProductDocsButton>
    </StyledGraphNode>
  )
}

function FocusNode({ data }: NodeProps<ProductNode>) {
  return (
    <StyledGraphNode $kind="focus">
      <Handle type="source" position={Position.Bottom}><GraphHandle /></Handle>
      <NodeEyebrow $kind="product">ИНСТРУКЦИЯ</NodeEyebrow>
      <NodeTitle>{data.label}</NodeTitle>
      <ProductInput value={data.value} onChange={(event) => data.onChange(event.target.value)} onKeyDown={(event) => event.stopPropagation()} spellCheck="false" aria-label="Фокус анализа продукта" />
    </StyledGraphNode>
  )
}

const nodeTypes = { product: FocusNode, 'product-data': ProductDataNode, analysis: AnalysisNode }

function App() {
  const [focus, setFocus] = useState(DEFAULT_FOCUS)
  const [nodes, setNodes] = useState<GraphNode[]>(() => initialNodes(setFocus))
  const [isRunning, setIsRunning] = useState(false)
  const [productContext, setProductContext] = useState<ProductContext | null>(null)
  const [productContextError, setProductContextError] = useState<string | null>(null)
  const [showProductContext, setShowProductContext] = useState(false)
  const [graphZoom, setGraphZoom] = useState(1)
  const [prompts, setPrompts] = useState<Record<PromptId, string>>({ ...DEFAULT_PROMPTS })
  const [promptStatuses, setPromptStatuses] = useState<Record<PromptId, 'saved' | 'dirty' | 'saving' | 'error'>>({
    questions: 'saved',
    parser: 'saved',
    brand: 'saved',
    insights: 'saved',
    critic: 'saved',
    validator: 'saved',
    output: 'saved',
  })

  useEffect(() => {
    void loadProductContext()
      .then(setProductContext)
      .catch((reason: unknown) => setProductContextError(reason instanceof Error ? reason.message : 'Не удалось загрузить документы продукта'))
  }, [])

  useEffect(() => {
    void loadPrompts()
      .then((loaded) => {
        promptTemplates = loaded
        setPrompts(loaded)
      })
      .catch(() => {
        // The in-memory defaults remain usable if the prompt store is unavailable.
      })
  }, [])

  const handlePromptChange = useCallback((id: PromptId, content: string) => {
    setPrompts((current) => ({ ...current, [id]: content }))
    setPromptStatuses((current) => ({ ...current, [id]: 'dirty' }))
  }, [])

  const handlePromptSave = useCallback((id: PromptId) => {
    const content = prompts[id]
    if (!content.trim() || promptStatuses[id] !== 'dirty') return
    setPromptStatuses((current) => ({ ...current, [id]: 'saving' }))
    void savePrompt(id, content)
      .then(() => {
        promptTemplates = { ...promptTemplates, [id]: content }
        setPromptStatuses((current) => ({ ...current, [id]: 'saved' }))
      })
      .catch(() => setPromptStatuses((current) => ({ ...current, [id]: 'error' })))
  }, [prompts, promptStatuses])

  function updateNode(nodeId: string, patch: Partial<AnalysisNodeData>) {
    setNodes((current) => current.map((node) => node.id === nodeId && node.type === 'analysis'
      ? { ...node, data: { ...node.data, ...patch } }
      : node))
  }

  async function runStage(stage: StageDefinition, prompt: string, sourceNames: Set<string>): Promise<Record<string, unknown>> {
    updateNode(stage.nodeId, { status: 'running', error: undefined })
    try {
      const parsed = parseAndValidate(await callOpenCode(prompt), sourceNames)
      updateNode(stage.nodeId, { status: 'complete', answer: JSON.stringify(parsed, null, 2) })
      return parsed
    } catch (reason) {
      const error = reason instanceof Error ? reason.message : 'Этап не выполнен'
      updateNode(stage.nodeId, { status: 'error', error })
      throw new Error(`${stage.label}: ${error}`)
    }
  }

  async function runGraph() {
    if (!focus.trim() || isRunning) return
    setIsRunning(true)
    promptTemplates = prompts
    setNodes((current) => current.map((node) => node.type === 'analysis'
      ? { ...node, data: { ...node.data, status: 'idle', answer: undefined, error: undefined } }
      : node))
    try {
      const context = productContext ?? await loadProductContext()
      setProductContext(context)
      setProductContextError(null)
      const sourceNames = new Set(context.files.map((file) => file.name))
      const sourceFiles = stageDefinitions.find((stage) => stage.id === 'questions')!
      const questions = await runStage(sourceFiles, questionsPrompt(focus, context), sourceNames)
      const extractorStages = stageDefinitions.filter((stage): stage is StageDefinition & { id: 'parser' | 'brand' | 'insights' } => stage.id === 'parser' || stage.id === 'brand' || stage.id === 'insights')
      const extractorResults = await Promise.all(extractorStages.map(async (stage) => {
        const result = await runStage(stage, extractorPrompt(stage.id, focus, context, questions), sourceNames)
        return [stage.id, result] as const
      }))
      const extractors = Object.fromEntries(extractorResults)
      const critic = await runStage(stageDefinitions.find((stage) => stage.id === 'critic')!, criticPrompt(focus, context, questions, extractors), sourceNames)
      const validator = await runStage(stageDefinitions.find((stage) => stage.id === 'validator')!, validatorPrompt(focus, context, extractors, critic), sourceNames)
      await runStage(stageDefinitions.find((stage) => stage.id === 'output')!, outputPrompt(focus, context, questions, extractors, critic, validator), sourceNames)
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : 'Граф не выполнен'
      const output = stageDefinitions.find((stage) => stage.id === 'output')!
      updateNode(output.nodeId, { status: 'error', error: message })
    } finally {
      setIsRunning(false)
    }
  }

  const graphNodes = useMemo(() => nodes.map((node) => {
    if (node.id === 'analysis-focus' && node.type === 'product') {
      return { ...node, data: { ...node.data, value: focus } }
    }
    if (node.id === 'product-data' && node.type === 'product-data') {
      return {
        ...node,
        data: {
          ...node.data,
          files: productContext?.files.map((file) => file.name) ?? [],
          loaded: Boolean(productContext),
          onOpenDocuments: () => setShowProductContext(true),
        },
      }
    }
    if (node.type === 'analysis') {
      return {
        ...node,
        data: {
          ...node.data,
          prompt: prompts[node.data.promptId],
          promptStatus: promptStatuses[node.data.promptId],
          onPromptChange: (value: string) => handlePromptChange(node.data.promptId, value),
          onPromptSave: () => handlePromptSave(node.data.promptId),
        },
      }
    }
    return node
  }), [nodes, focus, productContext, prompts, promptStatuses, handlePromptChange, handlePromptSave])

  const edges = useMemo(() => initialEdges.map((edge) => {
    const source = graphNodes.find((node) => node.id === edge.source)
    const target = graphNodes.find((node) => node.id === edge.target)
    const status = source?.type === 'analysis'
      ? source.data.status
      : target?.type === 'analysis' ? target.data.status : 'idle'
    return {
      ...edge,
      animated: status === 'running',
      className: `question-edge ${status}`,
      markerEnd: { type: MarkerType.ArrowClosed, color: status === 'complete' ? '#9ed36d' : '#74815a' },
    }
  }), [graphNodes])

  function handleNodesChange(changes: NodeChange[]) {
    setNodes((current) => applyNodeChanges(changes, current) as GraphNode[])
  }

  return (
    <AppShell>
      <GlobalStyle />
      <QuestionEdgeStyles />
      <Sidebar>
        <Brand><BrandMark>λ</BrandMark><span>llm lab</span></Brand>
        <SidebarLabel>Pipelines</SidebarLabel>
        <PipelineButton $active>
          <PipelineDot />
          <span><strong>Product passport</strong><small>Данные → вопросы → извлечение → проверки</small></span>
        </PipelineButton>
        <SidebarFooter><StatusDot />Local OpenCode</SidebarFooter>
      </Sidebar>
      <Workspace>
        <Topbar>
          <div><Eyebrow>Graph / product-passport</Eyebrow></div>
        </Topbar>
        {showProductContext && (
          <ProductDocsPanel aria-label="Документы продукта">
            <ProductDocsHeader>
              <div><strong>{productContext?.productName ?? PRODUCT_NAME}</strong><span>{productContext?.files.length ?? 0} документов · только просмотр</span></div>
              <ProductDocsButton type="button" onClick={() => setShowProductContext(false)}>Закрыть</ProductDocsButton>
            </ProductDocsHeader>
            {productContextError && <ProductDocumentBody>{productContextError}</ProductDocumentBody>}
            {productContext?.files.map((file) => (
              <ProductDocument key={file.name} open><ProductDocumentSummary>{file.name}</ProductDocumentSummary><ProductDocumentBody>{file.content}</ProductDocumentBody></ProductDocument>
            ))}
          </ProductDocsPanel>
        )}
        <GraphPanel aria-label="Product passport graph">
          <GraphLegend>
            <span><LegendDot $kind="input" />Данные</span>
            <span><LegendDot $kind="question" />Этапы</span>
            <span><LegendDot $kind="complete" />Проверено</span>
          </GraphLegend>
          <GraphZoomIndicator>Масштаб: {Math.round(graphZoom * 100)}%</GraphZoomIndicator>
          <ReactFlow nodes={graphNodes} edges={edges} nodeTypes={nodeTypes} onNodesChange={handleNodesChange} onMove={(_, viewport) => setGraphZoom(viewport.zoom)} onInit={(instance) => setGraphZoom(instance.getZoom())} fitView fitViewOptions={{ padding: 0.16 }} nodesConnectable={false} nodesDraggable zoomOnScroll panOnScroll minZoom={0.25} maxZoom={1.5} proOptions={{ hideAttribution: false }}>
            <Background color="#353b31" gap={24} size={1} />
            <Controls showInteractive={false} />
            <MiniMap pannable zoomable nodeColor={(node) => node.type === 'product' || node.type === 'product-data' ? '#d5f36b' : node.data.status === 'complete' ? '#9ed36d' : '#657054'} />
          </ReactFlow>
        </GraphPanel>
        <GraphActions><RunButton type="button" onClick={() => void runGraph()} disabled={isRunning}>{isRunning ? 'Running…' : 'Run graph'}<span aria-hidden="true">↗</span></RunButton></GraphActions>
      </Workspace>
    </AppShell>
  )
}

export default App

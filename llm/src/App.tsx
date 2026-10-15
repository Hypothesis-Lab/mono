import { useMemo, useState } from 'react'
import {
  Background,
  Controls,
  Handle,
  MarkerType,
  MiniMap,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import {
  Answer,
  AppShell,
  AudienceEntry,
  AudienceList,
  AudienceToggle,
  Brand,
  BrandMark,
  Eyebrow,
  GlobalStyle,
  GraphActions,
  GraphHandle,
  GraphLegend,
  GraphNode as StyledGraphNode,
  GraphPanel,
  LegendDot,
  NodeEyebrow,
  NodeStatus,
  NodeTitle,
  PipelineButton,
  PipelineDot,
  ProductInput,
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

const MODEL = 'cat-jam-free'
const DEFAULT_PRODUCT = 'Дебетовая и кредитная карта в одной: кредитный лимит подключается к дебетовой или зарплатной карте Цифра банка через мобильное приложение.'

type QuestionStatus = 'idle' | 'running' | 'complete' | 'error'

type ProductNodeData = {
  label: string
  value: string
  onChange: (value: string) => void
}

type QuestionNodeData = {
  label: string
  question: string
  status: QuestionStatus
  answer?: string
  error?: string
}

type AudienceNodeData = {
  label: string
  status: QuestionStatus
  answer?: string
  error?: string
}

type AudienceCatalogNodeData = {
  label: string
}

type ProductNode = Node<ProductNodeData, 'product'>
type QuestionNode = Node<QuestionNodeData, 'question'>
type AudienceNode = Node<AudienceNodeData, 'audience-ranking'>
type AudienceCatalogNodeType = Node<AudienceCatalogNodeData, 'audience-catalog'>
type GraphNode = ProductNode | QuestionNode | AudienceNode | AudienceCatalogNodeType
type GraphEdge = Edge<{ branchId?: string }>

type ChatResponse = {
  choices?: Array<{
    message?: {
      content?: string | null
    }
  }>
  error?: {
    message?: string
  }
}

const questionDefinitions = [
  {
    id: 'question-problem',
    label: 'Проблема',
    question: 'Какую основную проблему решает этот продукт? Ответь одним кратким предложением на русском языке.',
    position: { x: 40, y: 330 },
  },
  {
    id: 'question-user',
    label: 'Основной клиент',
    question: 'Кто основной клиент этого продукта и что ему нужно? Ответь кратко на русском языке.',
    position: { x: 340, y: 330 },
  },
  {
    id: 'question-value',
    label: 'Ценность продукта',
    question: 'В чём главная ценность этого продукта для клиента? Ответь одним кратким предложением на русском языке.',
    position: { x: 640, y: 330 },
  },
] as const

const audiences = [
  {
    id: 'persona_001',
    label: 'Мобильная специалистка с регулярными расходами',
    description: 'Женщина, 27 лет, маркетолог в Москве; постоянная работа, гражданство и постоянная регистрация в России, доход 95 000 ₽. iPhone 14, iOS; почти все финансовые операции делает в приложениях. Интересы: фитнес, сериалы, путешествия. Есть дебетовая карта Цифра банка. Ведёт бюджет, но иногда переносит крупные покупки до зарплаты. Готова к короткому льготному кредитованию, боится скрытых условий.',
  },
  {
    id: 'persona_002',
    label: 'Автомобилист — IT-аналитик',
    description: 'Мужчина, 35 лет, системный аналитик в Казани; постоянная работа, гражданство и постоянная регистрация в России, доход 180 000 ₽. Samsung Galaxy S24, Android; активно использует банковские приложения и уведомления. Интересы: автомобиль, технологии, инвестиции, фитнес. Есть зарплатная карта Цифра банка. Доход стабильный, но расходы на автомобиль возникают неравномерно. Использует кредит только при понятном сроке и полном контроле долга.',
  },
  {
    id: 'persona_003',
    label: 'Семейная бухгалтер с консервативным бюджетом',
    description: 'Женщина, 42 года, бухгалтер в Екатеринбурге; постоянная работа, гражданство и постоянная регистрация в России, доход 130 000 ₽. Xiaomi Redmi Note 13, Android; пользуется приложениями, но перепроверяет тарифы и документы. Интересы: семья, дом, фитнес, телесериалы. Не является клиентом Цифра банка. Планирует семейные расходы и держит небольшой резерв. Осторожна, опасается комиссий и сложных правил льготного периода.',
  },
  {
    id: 'persona_004',
    label: 'Региональный менеджер в разъездах',
    description: 'Мужчина, 51 год, региональный менеджер по продажам в Нижнем Новгороде; постоянная работа, гражданство и постоянная регистрация в России, доход 155 000 ₽. iPhone 13, iOS; часто платит телефоном и решает банковские вопросы дистанционно. Интересы: автомобиль, телевизор, футбол, поездки. Есть дебетовая карта Цифра банка. Бывают кассовые разрывы между командировочными расходами и компенсацией. Рассматривает лимит как резерв и не хочет носить вторую карту.',
  },
  {
    id: 'persona_005',
    label: 'Работающая преподавательница старшего возраста',
    description: 'Женщина, 63 года, преподаватель колледжа в Самаре; постоянная работа, гражданство и постоянная регистрация в России, доход 75 000 ₽. Realme C55, Android; использует базовые функции банка, сложные настройки вызывают вопросы. Интересы: телевизор, сад, здоровье, пешие прогулки. Есть зарплатная карта Цифра банка. Предпочитает копить заранее, но иногда помогает семье до зарплаты. Настороженно относится к кредиту и нуждается в простом объяснении.',
  },
  {
    id: 'persona_006',
    label: 'Молодой специалист и геймер',
    description: 'Мужчина, 23 года, специалист технической поддержки в Новосибирске; постоянная работа, гражданство и постоянная регистрация в России, доход 85 000 ₽. Google Pixel 8, Android; быстро осваивает приложения, предпочитает короткие интерфейсы. Интересы: игры, гаджеты, фитнес, каршеринг. Не является клиентом Цифра банка. Регулярные расходы контролирует, крупные покупки планирует не всегда. Интересуется гибкостью, но не хочет привыкать жить в долг.',
  },
  {
    id: 'persona_007',
    label: 'Руководительница digital-команды',
    description: 'Женщина, 34 года, руководитель digital-команды в Санкт-Петербурге; постоянная работа, гражданство и постоянная регистрация в России, доход 290 000 ₽. iPhone 16 Pro, iOS; ожидает полностью цифровой путь и прозрачные уведомления. Интересы: фитнес, путешествия, инвестиции, технологии. Есть дебетовая карта Цифра банка. Имеет финансовый резерв и использует кредитные продукты для управления ликвидностью. Готова пользоваться при высокой прозрачности, но небольшой лимит может быть нерелевантен.',
  },
  {
    id: 'persona_008',
    label: 'Инженер с семейным бюджетом',
    description: 'Мужчина, 46 лет, инженер производственного предприятия в Челябинске; постоянная работа, гражданство и постоянная регистрация в России, доход 145 000 ₽. Honor 200, Android; регулярно использует зарплатное банковское приложение. Интересы: автомобиль, ремонт дома, хоккей, телевизор. Есть зарплатная карта Цифра банка. Доход предсказуемый, но ремонт автомобиля или дома создаёт разовые пики расходов. Готов держать кредитный лимит как запас, если правила погашения понятны.',
  },
] as const

const rankingQuestion = `На основе описания продукта и ответов на три вопроса выбери три наиболее подходящие аудитории из каталога. Это гипотеза о соответствии продукта, а не утверждение о реальных клиентах. Сопоставь конкретную потребность, сценарий использования, отношение к кредиту и наличие карты Цифра банка. Не используй стереотипы о возрасте или поле. Не добавляй факты, которых нет во входных данных. Выбери ровно три разных audience_id из каталога, расположи по убыванию соответствия и кратко обоснуй каждый выбор на русском языке. Верни только JSON вида {"top_audiences":[{"audience_id":"persona_001","reason":"..."},{"audience_id":"persona_002","reason":"..."},{"audience_id":"persona_003","reason":"..."}]}.`

function initialNodes(onProductChange: (value: string) => void): GraphNode[] {
  return [
    {
      id: 'product-input',
      type: 'product',
      position: { x: 340, y: 54 },
      data: { label: 'Product input', value: DEFAULT_PRODUCT, onChange: onProductChange },
    },
    ...questionDefinitions.map((definition) => ({
      id: definition.id,
      type: 'question' as const,
      position: definition.position,
      data: {
        label: definition.label,
        question: definition.question,
        status: 'idle' as const,
      },
    })),
    {
      id: 'audience-ranking',
      type: 'audience-ranking',
      position: { x: 700, y: 760 },
      data: { label: 'Топ-3 целевые аудитории', status: 'idle' },
    },
    {
      id: 'audience-catalog',
      type: 'audience-catalog',
      position: { x: 30, y: 760 },
      data: { label: 'Каталог аудиторий' },
    },
  ]
}

const initialEdges: GraphEdge[] = [
  ...questionDefinitions.map((definition) => ({
    id: `product-to-${definition.id}`,
    source: 'product-input',
    target: definition.id,
    type: 'smoothstep',
    data: { branchId: definition.id },
    markerEnd: { type: MarkerType.ArrowClosed, color: '#74815a' },
  })),
  ...questionDefinitions.map((definition) => ({
    id: `${definition.id}-to-audience-ranking`,
    source: definition.id,
    target: 'audience-ranking',
    type: 'smoothstep',
    markerEnd: { type: MarkerType.ArrowClosed, color: '#74815a' },
  })),
  {
    id: 'catalog-to-audience-ranking',
    source: 'audience-catalog',
    sourceHandle: 'catalog-output',
    target: 'audience-ranking',
    targetHandle: 'catalog-input',
    type: 'smoothstep',
    markerEnd: { type: MarkerType.ArrowClosed, color: '#74815a' },
  },
]

async function callOpenCode(product: string, question: string): Promise<string> {
  const response = await fetch('/api/opencode', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      model: MODEL,
      messages: [
        {
          role: 'user',
          content: `You are answering a product research question.\n\n<product>\n${product}\n</product>\n\nQuestion: ${question}`,
        },
      ],
    }),
  })

  const body = (await response.json()) as ChatResponse
  if (!response.ok) {
    throw new Error(body.error?.message ?? `OpenCode returned HTTP ${response.status}`)
  }

  const content = body.choices?.[0]?.message?.content?.trim()
  if (!content) {
    throw new Error('OpenCode returned an empty response')
  }

  return content
}

function parseAudienceRanking(content: string) {
  const start = content.indexOf('{')
  const parsed = JSON.parse(start >= 0 ? content.slice(start) : content) as {
    top_audiences?: Array<{ audience_id?: string; reason?: string }>
  }
  const ranked = parsed.top_audiences
  if (!Array.isArray(ranked) || ranked.length !== 3) {
    throw new Error('Модель должна вернуть ровно три аудитории')
  }

  const allowedIDs = new Set<string>(audiences.map((audience) => audience.id))
  const seenIDs = new Set<string>()
  return ranked.map((item) => {
    if (!item.audience_id || !allowedIDs.has(item.audience_id) || seenIDs.has(item.audience_id)) {
      throw new Error('Модель вернула неизвестную или повторяющуюся аудиторию')
    }
    seenIDs.add(item.audience_id)
    const audience = audiences.find((candidate) => candidate.id === item.audience_id)!
    return { ...audience, reason: item.reason?.trim() || 'Обоснование не указано.' }
  })
}

function formatAudienceRanking(result: ReturnType<typeof parseAudienceRanking>) {
  return result.map((audience, index) => `${index + 1}. ${audience.label}\n${audience.reason}`).join('\n\n')
}

function ProductInputNode({ data }: NodeProps<ProductNode>) {
  return (
    <StyledGraphNode $kind="product">
      <Handle type="source" position={Position.Bottom}><GraphHandle /></Handle>
      <NodeEyebrow $kind="product">Input node</NodeEyebrow>
      <NodeTitle>{data.label}</NodeTitle>
      <ProductInput
        value={data.value}
        onChange={(event) => data.onChange(event.target.value)}
        onKeyDown={(event) => event.stopPropagation()}
        spellCheck="false"
        aria-label="Product input"
      />
    </StyledGraphNode>
  )
}

function QuestionNode({ data }: NodeProps<QuestionNode>) {
  return (
    <StyledGraphNode $status={data.status}>
      <Handle type="target" position={Position.Top}><GraphHandle /></Handle>
      <Handle type="source" position={Position.Bottom}><GraphHandle /></Handle>
      <QuestionHeader>
        <NodeEyebrow>Question node</NodeEyebrow>
        <NodeStatus $status={data.status}>{data.status}</NodeStatus>
      </QuestionHeader>
      <NodeTitle>{data.label}</NodeTitle>
      <QuestionCopy>{data.question}</QuestionCopy>
      <Answer $visible={Boolean(data.answer || data.error)} $error={Boolean(data.error)} aria-live="polite">
        {data.answer ?? data.error ?? (data.status === 'running' ? 'Thinking…' : 'Waiting for input')}
      </Answer>
    </StyledGraphNode>
  )
}

function AudienceRankingNode({ data }: NodeProps<AudienceNode>) {
  return (
    <StyledGraphNode $kind="audience" $status={data.status}>
      <Handle type="target" position={Position.Top}><GraphHandle /></Handle>
      <Handle id="catalog-input" type="target" position={Position.Left}><GraphHandle /></Handle>
      <QuestionHeader>
        <NodeEyebrow>Следующий этап · 8 аудиторий</NodeEyebrow>
        <NodeStatus $status={data.status}>{data.status}</NodeStatus>
      </QuestionHeader>
      <NodeTitle>{data.label}</NodeTitle>
      <QuestionCopy>{rankingQuestion}</QuestionCopy>
      <Answer $ranking $visible={Boolean(data.answer || data.error)} $error={Boolean(data.error)} aria-live="polite">
        {data.answer ?? data.error ?? (data.status === 'running' ? 'Сравниваем аудитории…' : 'Ожидает ответов на все вопросы')}
      </Answer>
    </StyledGraphNode>
  )
}

function AudienceCatalogNode() {
  const [isExpanded, setIsExpanded] = useState(false)

  return (
    <StyledGraphNode $kind="catalog">
      <Handle id="catalog-output" type="source" position={Position.Right}><GraphHandle /></Handle>
      <QuestionHeader>
        <NodeEyebrow>Задано в коде</NodeEyebrow>
        <NodeStatus $status="idle">{audiences.length} профилей</NodeStatus>
      </QuestionHeader>
      <NodeTitle>Каталог аудиторий</NodeTitle>
      <AudienceToggle
        className="nodrag"
        type="button"
        aria-expanded={isExpanded}
        onClick={(event) => {
          event.stopPropagation()
          setIsExpanded((expanded) => !expanded)
        }}
      >
        {isExpanded ? 'Свернуть каталог' : 'Показать аудитории'}
        <span aria-hidden="true">{isExpanded ? '−' : '+'}</span>
      </AudienceToggle>
      {isExpanded && (
        <AudienceList>
          {audiences.map((audience) => (
            <AudienceEntry key={audience.id}>
              <strong>{audience.label}</strong>
              <p>{audience.description}</p>
            </AudienceEntry>
          ))}
        </AudienceList>
      )}
    </StyledGraphNode>
  )
}

const nodeTypes = {
  product: ProductInputNode,
  question: QuestionNode,
  'audience-ranking': AudienceRankingNode,
  'audience-catalog': AudienceCatalogNode,
}

function App() {
  const [product, setProduct] = useState(DEFAULT_PRODUCT)
  const [nodes, setNodes] = useState<GraphNode[]>(() => initialNodes(setProduct))
  const [isRunning, setIsRunning] = useState(false)

  function updateQuestion(id: string, patch: Partial<QuestionNodeData>) {
    setNodes((current) => current.map((node) => node.id === id && node.type === 'question'
      ? { ...node, data: { ...node.data, ...patch } }
      : node))
  }

  function updateAudienceRanking(patch: Partial<AudienceNodeData>) {
    setNodes((current) => current.map((node) => node.id === 'audience-ranking' && node.type === 'audience-ranking'
      ? { ...node, data: { ...node.data, ...patch } }
      : node))
  }

  async function runGraph(input = product) {
    if (!input.trim() || isRunning) return
    setIsRunning(true)
    setNodes((current) => current.map((node) => node.type === 'question'
      ? { ...node, data: { ...node.data, status: 'running', answer: undefined, error: undefined } }
      : node.type === 'audience-ranking'
        ? { ...node, data: { ...node.data, status: 'idle', answer: undefined, error: undefined } }
        : node))

    const results = await Promise.all(questionDefinitions.map(async (definition) => {
      try {
        const answer = await callOpenCode(input, definition.question)
        updateQuestion(definition.id, { status: 'complete', answer })
        return { id: definition.id, answer }
      } catch (reason) {
        const error = reason instanceof Error ? reason.message : 'Question failed'
        updateQuestion(definition.id, {
          status: 'error',
          error,
        })
        return { id: definition.id, error }
      }
    }))

    if (results.some((result) => 'error' in result)) {
      updateAudienceRanking({
        status: 'error',
        error: 'Не удалось ответить на все вопросы; выбор аудитории не запускался.',
      })
      setIsRunning(false)
      return
    }

    updateAudienceRanking({ status: 'running', answer: undefined, error: undefined })
    try {
      const questionAnswers = results.map((result) => {
        const definition = questionDefinitions.find((item) => item.id === result.id)!
        return { question: definition.question, answer: 'answer' in result ? result.answer : '' }
      })
      const rankingResponse = await callOpenCode(input, `${rankingQuestion}\n\nОтветы на вопросы:\n${JSON.stringify(questionAnswers, null, 2)}\n\nКаталог аудиторий:\n${JSON.stringify(audiences, null, 2)}`)
      const ranking = parseAudienceRanking(rankingResponse)
      updateAudienceRanking({ status: 'complete', answer: formatAudienceRanking(ranking) })
    } catch (reason) {
      updateAudienceRanking({
        status: 'error',
        error: reason instanceof Error ? reason.message : 'Не удалось выбрать аудитории',
      })
    }

    setIsRunning(false)
  }

  const graphNodes = useMemo(() => nodes.map((node) => node.id === 'product-input' && node.type === 'product'
    ? { ...node, data: { ...node.data, value: product } }
    : node), [nodes, product])

  const edges = useMemo(() => initialEdges.map((edge) => {
    const target = graphNodes.find((node) => node.id === edge.target)
    const status = edge.source === 'product-input'
      ? target?.type === 'question' ? target.data.status : 'idle'
      : target?.type === 'audience-ranking' ? target.data.status : 'idle'
    return {
      ...edge,
      animated: status === 'running',
      className: `question-edge ${status}`,
      markerEnd: { type: MarkerType.ArrowClosed, color: status === 'complete' ? '#9ed36d' : '#74815a' },
    }
  }), [graphNodes])

  return (
    <AppShell>
      <GlobalStyle />
      <QuestionEdgeStyles />
      <Sidebar>
        <Brand>
          <BrandMark>λ</BrandMark>
          <span>llm lab</span>
        </Brand>

        <SidebarLabel>Pipelines</SidebarLabel>
        <PipelineButton $active>
          <PipelineDot />
          <span>
            <strong>Product questions</strong>
            <small>3 вопроса → топ-3 аудитории</small>
          </span>
        </PipelineButton>

        <SidebarFooter>
          <StatusDot />
          Local OpenCode
        </SidebarFooter>
      </Sidebar>

      <Workspace>
        <Topbar>
          <div>
            <Eyebrow>Graph / product-questions</Eyebrow>
          </div>
        </Topbar>

        <GraphPanel aria-label="Product question graph">
          <GraphLegend>
            <span><LegendDot $kind="input" />Ввод</span>
            <span><LegendDot $kind="question" />Вопросы</span>
            <span><LegendDot $kind="complete" />Ответ получен</span>
          </GraphLegend>
          <ReactFlow
            nodes={graphNodes}
            edges={edges}
            nodeTypes={nodeTypes}
            fitView
            fitViewOptions={{ padding: 0.18 }}
            nodesConnectable={false}
            nodesDraggable
            zoomOnScroll
            panOnScroll
            minZoom={0.5}
            maxZoom={1.5}
            proOptions={{ hideAttribution: false }}
          >
            <Background color="#353b31" gap={24} size={1} />
            <Controls showInteractive={false} />
            <MiniMap
              pannable
              zoomable
              nodeColor={(node) => node.type === 'product' ? '#d5f36b' : node.data.status === 'complete' ? '#9ed36d' : '#657054'}
            />
          </ReactFlow>
        </GraphPanel>
        <GraphActions>
          <RunButton type="button" onClick={() => void runGraph()} disabled={isRunning}>
            {isRunning ? 'Running…' : 'Run graph'}
            <span aria-hidden="true">↗</span>
          </RunButton>
        </GraphActions>
      </Workspace>
    </AppShell>
  )
}

export default App

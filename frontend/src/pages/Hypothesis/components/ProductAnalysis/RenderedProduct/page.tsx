import * as S from './styles'

type RenderedProductProps = {
  value: Record<string, unknown>
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null
}

function records(value: unknown): Record<string, unknown>[] {
  return Array.isArray(value) ? value.filter((item): item is Record<string, unknown> => asRecord(item) !== null) : []
}

function strings(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
}

function text(value: unknown, fallback = 'Не указано'): string {
  return typeof value === 'string' && value.trim() !== '' ? value : fallback
}

const categoryLabels: Record<string, string> = {
  identity: 'О продукте',
  purpose: 'Назначение',
  audience: 'Аудитория',
  jtbd: 'Задачи пользователей',
  features: 'Возможности',
  benefits: 'Польза',
  pricing: 'Тарифы',
  terms: 'Условия',
  limitations: 'Ограничения',
  economics: 'Результаты',
  funnel: 'Целевое действие',
  offers_channels: 'Оффер и каналы',
  communication_rules: 'Правила коммуникации',
}

function categoryLabel(category: unknown): string {
  if (typeof category !== 'string') return 'Факты'
  return categoryLabels[category] ?? category.replaceAll('_', ' ')
}

function factStatusLabel(status: unknown): string {
  if (status === 'supported') return 'Подтверждено'
  if (status === 'conflicted') return 'Есть противоречие'
  if (status === 'not_specified') return 'Не указано'
  return text(status, '')
}

function resultStatusLabel(status: unknown): string {
  if (status === 'complete') return 'Готово'
  if (status === 'partial') return 'Частично'
  if (status === 'failed') return 'Ошибка'
  return text(status)
}

function FactCard({ fact }: { fact: Record<string, unknown> }) {
  const evidence = records(fact.evidence)
  const conditions = strings(fact.conditions)
  const status = factStatusLabel(fact.evidence_status)

  return (
    <S.FactCard>
      <S.FactHeader>
        <S.FactSubject>{text(fact.subject)}</S.FactSubject>
        {status && <S.StatusBadge $status={fact.evidence_status === 'conflicted' ? 'warning' : 'success'}>{status}</S.StatusBadge>}
      </S.FactHeader>
      <S.FactStatement>{text(fact.statement)}</S.FactStatement>
      {(conditions.length > 0 || typeof fact.time_scope === 'string' || evidence.length > 0) && (
        <S.EvidenceDetails>
          <S.EvidenceSummary>
            Источники и условия{evidence.length > 0 ? ` · ${evidence.length}` : ''}
          </S.EvidenceSummary>
          {typeof fact.time_scope === 'string' && <S.EvidenceNote>Период: {text(fact.time_scope)}</S.EvidenceNote>}
          {conditions.length > 0 && (
            <S.TagList>
              {conditions.map((condition) => <S.Tag key={condition}>{condition}</S.Tag>)}
            </S.TagList>
          )}
          {evidence.length > 0 && (
            <S.EvidenceList>
              {evidence.map((item, index) => (
                <S.EvidenceItem key={`${text(item.source, String(index))}-${index}`}>
                  <S.EvidenceQuote>{text(item.quote)}</S.EvidenceQuote>
                  <S.EvidenceSource>{text(item.source)}{item.locator ? ` · ${text(item.locator)}` : ''}</S.EvidenceSource>
                </S.EvidenceItem>
              ))}
            </S.EvidenceList>
          )}
        </S.EvidenceDetails>
      )}
    </S.FactCard>
  )
}

function NoticeList({ title, items, tone }: { title: string; items: string[]; tone: 'warning' | 'neutral' }) {
  if (items.length === 0) return null

  return (
    <S.Notice $tone={tone}>
      <S.NoticeTitle>{title}</S.NoticeTitle>
      <S.BulletList>
        {items.map((item) => <li key={item}>{item}</li>)}
      </S.BulletList>
    </S.Notice>
  )
}

export function RenderedProduct({ value }: RenderedProductProps) {
  const passport = asRecord(value.passport) ?? value
  const factList = records(passport.records)
  const groupedFacts = factList.reduce<Record<string, Record<string, unknown>[]>>((groups, fact) => {
    const category = typeof fact.category === 'string' ? fact.category : 'other'
    groups[category] = [...(groups[category] ?? []), fact]
    return groups
  }, {})
  const conflicts = records(passport.conflicts)
  const gaps = records(passport.gaps)
  const warnings = strings(passport.warnings)
  const evaluation = asRecord(value.evaluation)
  const validation = asRecord(value.validation)

  return (
    <S.Root>
      <S.ProductHero>
        <S.ProductHeading>
          <S.ProductName>{text(passport.product_name, 'Паспорт продукта')}</S.ProductName>
          <S.StatusBadge $status={passport.extraction_status === 'complete' ? 'success' : 'warning'}>
            {resultStatusLabel(passport.extraction_status)}
          </S.StatusBadge>
        </S.ProductHeading>
        <S.Metrics>
          <S.Metric><S.MetricValue>{factList.length}</S.MetricValue><S.MetricLabel>фактов</S.MetricLabel></S.Metric>
          <S.Metric><S.MetricValue>{conflicts.length}</S.MetricValue><S.MetricLabel>противоречий</S.MetricLabel></S.Metric>
          <S.Metric><S.MetricValue>{gaps.length}</S.MetricValue><S.MetricLabel>пробелов</S.MetricLabel></S.Metric>
        </S.Metrics>
      </S.ProductHero>

      {Object.entries(groupedFacts).map(([category, facts]) => (
        <S.FactSection key={category}>
          <S.SectionHeading>{categoryLabel(category)} <S.SectionCount>{facts.length}</S.SectionCount></S.SectionHeading>
          <S.FactList>{facts.map((fact) => <FactCard key={text(fact.id)} fact={fact} />)}</S.FactList>
        </S.FactSection>
      ))}

      {(conflicts.length > 0 || gaps.length > 0 || warnings.length > 0) && (
        <S.NoticeGrid>
          <NoticeList
            title={`Противоречия${conflicts.length > 0 ? ` · ${conflicts.length}` : ''}`}
            tone="warning"
            items={conflicts.map((conflict) => `${text(conflict.description)}${conflict.needed_to_resolve ? ` Решить: ${text(conflict.needed_to_resolve)}` : ''}`)}
          />
          <NoticeList title={`Пробелы${gaps.length > 0 ? ` · ${gaps.length}` : ''}`} tone="neutral" items={gaps.map((gap) => `${text(gap.category)}: ${text(gap.missing)}`)} />
          <NoticeList title={`Предупреждения${warnings.length > 0 ? ` · ${warnings.length}` : ''}`} tone="warning" items={warnings} />
        </S.NoticeGrid>
      )}

      {(evaluation || validation) && (
        <S.QualityPanel>
          <S.SectionHeading>Проверка результата</S.SectionHeading>
          <S.QualityItems>
            <S.QualityItem><S.QualityLabel>Оценка</S.QualityLabel><S.QualityValue>{typeof evaluation?.overall_score === 'number' ? `${evaluation.overall_score} / 100` : 'Не рассчитана'}</S.QualityValue></S.QualityItem>
            {evaluation?.evaluation_status !== undefined && <S.QualityItem><S.QualityLabel>Статус оценки</S.QualityLabel><S.QualityValue>{resultStatusLabel(evaluation.evaluation_status)}</S.QualityValue></S.QualityItem>}
            {validation?.valid !== undefined && <S.QualityItem><S.QualityLabel>Структура</S.QualityLabel><S.QualityValue>{validation.valid ? 'Корректна' : 'Есть ошибки'}</S.QualityValue></S.QualityItem>}
          </S.QualityItems>
          {evaluation?.summary !== undefined && <S.QualitySummary>{text(evaluation.summary)}</S.QualitySummary>}
        </S.QualityPanel>
      )}
    </S.Root>
  )
}

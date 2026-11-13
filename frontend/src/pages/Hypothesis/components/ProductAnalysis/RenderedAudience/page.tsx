import * as S from './styles'

type RenderedAudienceProps = {
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

function resultStatusLabel(status: unknown): string {
  if (status === 'complete') return 'Готово'
  if (status === 'partial') return 'Частично'
  if (status === 'pass') return 'Проверка пройдена'
  if (status === 'failed') return 'Ошибка'
  return text(status)
}

function formatNumber(value: unknown): string {
  return typeof value === 'number'
    ? new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 2 }).format(value)
    : text(value)
}

function formatFeatureName(value: unknown): string {
  if (typeof value !== 'string') return 'Признак'
  const labels: Record<string, string> = {
    expenses: 'расходы',
    total: 'всего',
    account: 'счёт',
    accounts: 'счета',
    category: 'категория',
    amount: 'сумма',
    count: 'количество',
    most: 'частый',
    common: 'вариант',
  }
  const parts = value.split('__').filter((part) => part && part !== 't01' && !/^value_\d+$/.test(part) && part !== 'all')
  return parts.map((part) => labels[part] ?? part.replaceAll('_', ' ')).join(' · ')
}

function featureMetric(feature: Record<string, unknown>, side: 'segment' | 'overall'): string {
  const share = feature[`${side}_share`]
  if (typeof share === 'number') return `${formatNumber(share * 100)}%`

  const mean = feature[`${side}_mean`]
  const median = feature[`${side}_median`]
  if (mean !== undefined) {
    return median === undefined
      ? formatNumber(mean)
      : `${formatNumber(mean)} · медиана ${formatNumber(median)}`
  }

  return text(feature.value)
}

function FeatureTable({ features }: { features: Record<string, unknown>[] }) {
  if (features.length === 0) return null

  return (
    <S.TableWrapper>
      <S.FeatureTable>
        <thead>
          <tr><th>Признак</th><th>В сегменте</th><th>В целом</th><th>Направление</th></tr>
        </thead>
        <tbody>
          {features.map((feature, index) => (
            <tr key={`${text(feature.feature, 'feature')}-${index}`}>
              <td><S.FeatureName>{formatFeatureName(feature.feature)}</S.FeatureName>{feature.value !== undefined && <S.FeatureValue>{text(feature.value)}</S.FeatureValue>}</td>
              <td>{featureMetric(feature, 'segment')}</td>
              <td>{featureMetric(feature, 'overall')}</td>
              <td><S.Direction $higher={feature.direction === 'higher'}>{feature.direction === 'higher' ? 'Выше' : feature.direction === 'lower' ? 'Ниже' : text(feature.direction)}</S.Direction><S.Effect>эффект {formatNumber(feature.effect_size)}</S.Effect></td>
            </tr>
          ))}
        </tbody>
      </S.FeatureTable>
    </S.TableWrapper>
  )
}

function SegmentCard({ segment, selected, summary }: { segment: Record<string, unknown>; selected: boolean; summary?: string }) {
  const size = typeof segment.size === 'number' ? formatNumber(segment.size) : null
  const signals = strings(segment.supporting_signals)
  const unknowns = strings(segment.unknowns)
  const rationale = typeof segment.rationale === 'string' ? segment.rationale : undefined

  return (
    <S.SegmentCard>
      <S.SegmentSummary>
        <S.SegmentHeader>
          <div>
            <S.SegmentName>{text(segment.segment_id, 'Сегмент')}</S.SegmentName>
            <S.SegmentId>{text(segment.candidate_id)}</S.SegmentId>
          </div>
          <S.SegmentBadges>
            {selected && <S.SelectionBadge>Выбран</S.SelectionBadge>}
            {size && <S.SizeBadge>{size} участников</S.SizeBadge>}
          </S.SegmentBadges>
        </S.SegmentHeader>
      </S.SegmentSummary>
      <S.SegmentBody>
        {rationale !== undefined && rationale !== summary && <S.SegmentText>{rationale}</S.SegmentText>}
        {signals.length > 0 && (
          <S.SegmentBlock>
            <S.BlockTitle>Поддерживающие сигналы</S.BlockTitle>
            <S.BulletList>{signals.map((signal) => <li key={signal}>{signal}</li>)}</S.BulletList>
          </S.SegmentBlock>
        )}
        <FeatureTable features={records(segment.key_features)} />
        {unknowns.length > 0 && (
          <S.Unknowns>
            <S.BlockTitle>Что пока неизвестно</S.BlockTitle>
            <S.BulletList>{unknowns.map((unknown) => <li key={unknown}>{unknown}</li>)}</S.BulletList>
          </S.Unknowns>
        )}
      </S.SegmentBody>
    </S.SegmentCard>
  )
}

export function RenderedAudience({ value }: RenderedAudienceProps) {
  const audience = asRecord(value.audience) ?? value
  const segments = records(audience.segments)
  const selectedIds = strings(audience.selected_candidate_ids)
  const selection = asRecord(audience.selection)
  const review = asRecord(audience.review)
  const limitations = strings(audience.limitations)
  const selectionRationale = typeof selection?.rationale === 'string' ? selection.rationale : undefined

  return (
    <S.Root>
      <S.AudienceHero>
        <S.HeroStatus>
          <S.StatusBadge $status={audience.status === 'complete' ? 'success' : 'warning'}>{resultStatusLabel(audience.status)}</S.StatusBadge>
          <S.HeroCaption>{segments.length} сегм. · {selectedIds.length} выбрано</S.HeroCaption>
        </S.HeroStatus>
        {selection?.rationale !== undefined && <S.HeroText>{text(selection.rationale)}</S.HeroText>}
      </S.AudienceHero>

      {segments.length > 0 ? (
        <S.SegmentList>
          {segments.map((segment, index) => (
            <SegmentCard
              key={text(segment.segment_id, String(index))}
              segment={segment}
              summary={selectionRationale}
              selected={selectedIds.includes(text(segment.candidate_id, '')) || selection?.segment_id === segment.segment_id}
            />
          ))}
        </S.SegmentList>
      ) : (
        <S.EmptyState>Сегменты пока не сформированы.</S.EmptyState>
      )}

      {review?.rationale !== undefined && (
        <S.ReviewPanel>
          <S.BlockTitle>Проверка аудитории · {resultStatusLabel(review.status)}</S.BlockTitle>
          <S.SegmentText>{text(review.rationale)}</S.SegmentText>
          {strings(review.limitations).length > 0 && <S.BulletList>{strings(review.limitations).map((item) => <li key={item}>{item}</li>)}</S.BulletList>}
        </S.ReviewPanel>
      )}

      {limitations.length > 0 && (
        <S.LimitationPanel>
          <S.BlockTitle>Ограничения результата</S.BlockTitle>
          <S.BulletList>{limitations.map((item) => <li key={item}>{item}</li>)}</S.BulletList>
        </S.LimitationPanel>
      )}
    </S.Root>
  )
}

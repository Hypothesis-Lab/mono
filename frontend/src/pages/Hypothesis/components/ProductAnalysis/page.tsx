import { Fragment, useEffect, useRef, useState } from 'react'
import { skipToken } from '@reduxjs/toolkit/query'
import {
  isApiNotFound,
  rtkErrorMessage,
} from '../../../../api/structs'
import {
  useContinueProductAnalysisMutation,
  useGetProductAnalysisQuery,
  useGetProductAnalysisStatusQuery,
  useSaveProductAnalysisReviewMutation,
  useStartProductAnalysisMutation,
} from '../../../../store/generatedApi'
import type {
  HypothesisRating,
  ProductAnalysisResponse,
  ProductAnalysisStatusResponse,
} from '../../../../store/generatedApi'
import { JsonTree } from './JsonTree/page'
import * as S from './styles'

type ProductAnalysisProps = {
  productId: string
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return isRecord(value) ? value : null
}

function audienceCandidates(value: unknown): Record<string, unknown>[] {
  const result = asRecord(value)
  const audience = asRecord(result?.audience) ?? result
  const candidates = Array.isArray(audience?.candidates) ? audience.candidates : audience?.segments
  return Array.isArray(candidates) ? candidates.filter(isRecord) : []
}

function candidateId(candidate: Record<string, unknown>, index: number): string {
  if (typeof candidate.candidate_id === 'string' && candidate.candidate_id !== '') return candidate.candidate_id
  if (typeof candidate.segment_id === 'string' && candidate.segment_id !== '') return candidate.segment_id
  return `audience_${index + 1}`
}

function hypothesisCandidateId(candidate: Record<string, unknown>, index: number): string {
  if (typeof candidate.candidate_id === 'string' && candidate.candidate_id !== '') return candidate.candidate_id
  return `hypothesis_${index + 1}`
}

export function ProductAnalysis({ productId }: ProductAnalysisProps) {
  const {
    data: analysisStatus,
    error: analysisStatusError,
    refetch: refetchAnalysisStatus,
  } = useGetProductAnalysisStatusQuery(
    { id: productId },
    { pollingInterval: 10000 },
  )
  const {
    data: analysis,
    error: analysisError,
    refetch: refetchAnalysis,
  } = useGetProductAnalysisQuery(analysisStatus ? { id: productId } : skipToken)

  const [startAnalysis, { isLoading: startingAnalysis }] = useStartProductAnalysisMutation()
  const [saveAnalysisReview, { isLoading: savingAnalysisReview }] = useSaveProductAnalysisReviewMutation()
  const [continueAnalysis, { isLoading: continuingAnalysis }] = useContinueProductAnalysisMutation()

  const [analysisStartError, setAnalysisStartError] = useState<string | null>(null)
  const [analysisReviewError, setAnalysisReviewError] = useState<string | null>(null)
  const [audienceSelectionRunId, setAudienceSelectionRunId] = useState<string | null>(null)
  const [selectedAudienceIds, setSelectedAudienceIds] = useState<string[]>([])
  const [customAudiences, setCustomAudiences] = useState<Record<string, unknown>[]>([])
  const [hypothesisRatingRunId, setHypothesisRatingRunId] = useState<string | null>(null)
  const [hypothesisRatings, setHypothesisRatings] = useState<Record<string, HypothesisRating>>({})
  const [customAudienceName, setCustomAudienceName] = useState('')
  const [customAudienceDescription, setCustomAudienceDescription] = useState('')
  const [showCustomAudienceForm, setShowCustomAudienceForm] = useState(false)
  const [hypothesisRatingError, setHypothesisRatingError] = useState<string | null>(null)
  const analysisStatusSignature = analysisStatus
    ? `${analysisStatus.id}:${analysisStatus.status}:${analysisStatus.current_stage}:${analysisStatus.product_result_available}:${analysisStatus.audience_result_available}:${analysisStatus.hypothesis_result_available}:${analysisStatus.audience_review_available}:${analysisStatus.hypothesis_review_available}`
    : null
  const previousAnalysisStatusSignature = useRef<string | null>(null)

  useEffect(() => {
    if (!analysis || analysis.id === audienceSelectionRunId) return

    const candidates = audienceCandidates(analysis.audience_result)
    const review = asRecord(analysis.audience_review)
    const reviewedIds = Array.isArray(review?.selected_candidate_ids)
      ? review.selected_candidate_ids.filter((id): id is string => typeof id === 'string')
      : []
    setSelectedAudienceIds(
      reviewedIds.length > 0 ? reviewedIds : candidates.map((item, index) => candidateId(item, index)),
    )
    setCustomAudiences(
      Array.isArray(review?.custom_candidates) ? review.custom_candidates.filter(isRecord) : [],
    )
    setAudienceSelectionRunId(analysis.id)
  }, [analysis, audienceSelectionRunId])

  useEffect(() => {
    if (!analysis || analysis.id === hypothesisRatingRunId) return

    const review = asRecord(analysis.hypothesis_review)
    const ratings = asRecord(review?.ratings)
    const nextRatings: Record<string, HypothesisRating> = {}
    for (const [id, rating] of Object.entries(ratings ?? {})) {
      if (rating === 'good' || rating === 'bad') {
        nextRatings[id] = rating
      }
    }

    setHypothesisRatings(nextRatings)
    setHypothesisRatingRunId(analysis.id)
  }, [analysis, hypothesisRatingRunId])

  useEffect(() => {
    if (!analysisStatusSignature) return

    if (previousAnalysisStatusSignature.current && previousAnalysisStatusSignature.current !== analysisStatusSignature) {
      void refetchAnalysis()
    }

    previousAnalysisStatusSignature.current = analysisStatusSignature
  }, [analysisStatusSignature, refetchAnalysis])

  async function handleStartAnalysis() {
    setAnalysisStartError(null)

    try {
      await startAnalysis({ id: productId }).unwrap()
      await refetchAnalysisStatus()
    } catch (err) {
      setAnalysisStartError(rtkErrorMessage(err, 'Не удалось начать анализ'))
    }
  }

  function toggleAudienceCandidate(id: string) {
    setSelectedAudienceIds((current) =>
      current.includes(id) ? current.filter((item) => item !== id) : [...current, id],
    )
  }

  function handleAddCustomAudience() {
    const nameValue = customAudienceName.trim()
    const descriptionValue = customAudienceDescription.trim()
    if (!nameValue) {
      setAnalysisReviewError('Укажите название собственной аудитории')
      return
    }

    const id = `custom_audience_${String(customAudiences.length + 1).padStart(3, '0')}`
    const customCandidate = {
      candidate_id: id,
      segment_id: id,
      name: nameValue,
      description: descriptionValue,
    }
    setCustomAudiences((current) => [...current, customCandidate])
    setSelectedAudienceIds((current) => [...current, id])
    setCustomAudienceName('')
    setCustomAudienceDescription('')
    setShowCustomAudienceForm(false)
    setAnalysisReviewError(null)
  }

  async function handleSaveAudienceReview() {
    if (!analysis) return

    if (selectedAudienceIds.length === 0) {
      setAnalysisReviewError('Выберите хотя бы одну аудиторию или добавьте собственную')
      return
    }

    setAnalysisReviewError(null)
    try {
      await saveAnalysisReview({
        id: productId,
        productAnalysisReviewRequest: {
          stage: 'audience',
          selected_candidate_ids: selectedAudienceIds,
          custom_candidates: customAudiences,
        },
      }).unwrap()
      await continueAnalysis({ id: productId }).unwrap()
      await Promise.all([refetchAnalysisStatus(), refetchAnalysis()])
    } catch (err) {
      setAnalysisReviewError(rtkErrorMessage(err, 'Не удалось сохранить выбор аудитории'))
    }
  }

  async function handleSaveHypothesisRatings() {
    if (Object.keys(hypothesisRatings).length === 0) {
      setHypothesisRatingError('Оцените хотя бы одну гипотезу')
      return
    }

    setHypothesisRatingError(null)
    try {
      await saveAnalysisReview({
        id: productId,
        productAnalysisReviewRequest: {
          stage: 'hypotheses',
          selected_candidate_ids: [],
          custom_candidates: [],
          ratings: hypothesisRatings,
        },
      }).unwrap()
      await refetchAnalysis()
    } catch (err) {
      setHypothesisRatingError(rtkErrorMessage(err, 'Не удалось сохранить оценки гипотез'))
    }
  }

  const audienceCandidateList = audienceCandidates(analysis?.audience_result)
  const hypothesisResult = asRecord(analysis?.hypothesis_result)
  const hypotheses = Array.isArray(hypothesisResult?.hypotheses)
    ? hypothesisResult.hypotheses.filter(isRecord)
    : []

  return (
    <S.AnalysisSection>
      <S.SectionTitle>Анализ и генерация</S.SectionTitle>
      <S.Message>Используются сохранённые данные продукта и загруженные документы.</S.Message>
      {analysisStatus && (
        <>
          <S.AnalysisStatusTitle>Статус</S.AnalysisStatusTitle>
          <S.AnalysisPipeline aria-label="Этапы анализа">
            {getAnalysisPipelineStages(analysisStatus, analysis).map((stage, index) => (
              <Fragment key={stage.id}>
                <S.AnalysisPipelineItem
                  $state={stage.state}
                  aria-current={stage.state === 'active' || stage.state === 'waiting' ? 'step' : undefined}
                >
                  <S.AnalysisPipelineMarker $state={stage.state}>
                    {stage.state === 'completed' ? '✓' : stage.state === 'error' ? '!' : index + 1}
                  </S.AnalysisPipelineMarker>
                  <S.AnalysisPipelineInfo>
                    <S.AnalysisPipelineName>{stage.name}</S.AnalysisPipelineName>
                    <S.AnalysisPipelineStatus>{stage.label}</S.AnalysisPipelineStatus>
                  </S.AnalysisPipelineInfo>
                </S.AnalysisPipelineItem>
                {index < 2 && <S.AnalysisPipelineArrow aria-hidden="true">→</S.AnalysisPipelineArrow>}
              </Fragment>
            ))}
          </S.AnalysisPipeline>
        </>
      )}
      {(analysisStatusError || analysisError) && !isApiNotFound(analysisStatusError ?? analysisError) && (
        <S.ErrorMessage>{rtkErrorMessage(analysisStatusError ?? analysisError, 'Не удалось загрузить результаты анализа')}</S.ErrorMessage>
      )}
      {(isRecord(analysis?.product_result) || isRecord(analysis?.audience_result) || hypotheses.length > 0) && (
        <S.ResultsTitle>Результаты</S.ResultsTitle>
      )}
      {isRecord(analysis?.product_result) && (
        <S.AudienceDetails>
          <S.AudienceSummary>Паспорт продукта</S.AudienceSummary>
          <JsonTree value={analysis.product_result} />
        </S.AudienceDetails>
      )}
      {isRecord(analysis?.audience_result) && (
        <S.AudienceDetails>
          <S.AudienceSummary>Аудитории</S.AudienceSummary>
          <JsonTree value={analysis.audience_result} />
        </S.AudienceDetails>
      )}
      {analysisStatus?.status === 'waiting_for_user_input' && analysisStatus.current_stage === 'audience' && (
        <S.ReviewForm>
          <S.ReviewHeading>Проверьте предложенные аудитории</S.ReviewHeading>
          <S.Message>Оставьте подходящие варианты или добавьте собственную аудиторию.</S.Message>
          {audienceCandidateList.length > 0 && (
            <S.AudienceList>
              {audienceCandidateList.map((candidate, index) => {
                const id = candidateId(candidate, index)
                const title = typeof candidate.name === 'string'
                  ? candidate.name
                  : typeof candidate.segment_id === 'string'
                    ? candidate.segment_id
                    : `Аудитория ${index + 1}`
                const description = typeof candidate.description === 'string'
                  ? candidate.description
                  : typeof candidate.rationale === 'string'
                    ? candidate.rationale
                    : 'Описание не указано'
                return (
                  <S.ReviewItem key={id}>
                    <S.ReviewCheckbox
                      type="checkbox"
                      checked={selectedAudienceIds.includes(id)}
                      onChange={() => toggleAudienceCandidate(id)}
                    />
                    <S.AudienceInfo>
                      <S.AudienceDescription>{title}</S.AudienceDescription>
                      <S.AudienceMeta>{description}</S.AudienceMeta>
                    </S.AudienceInfo>
                  </S.ReviewItem>
                )
              })}
            </S.AudienceList>
          )}
          {customAudiences.length > 0 && (
            <S.AudienceList>
              {customAudiences.map((candidate) => {
                const id = candidateId(candidate, 0)
                return (
                  <S.ReviewItem key={id}>
                    <S.ReviewCheckbox
                      type="checkbox"
                      checked={selectedAudienceIds.includes(id)}
                      onChange={() => toggleAudienceCandidate(id)}
                    />
                    <S.AudienceInfo>
                      <S.AudienceDescription>
                        {typeof candidate.name === 'string' ? candidate.name : id}
                      </S.AudienceDescription>
                      <S.AudienceMeta>Собственная аудитория</S.AudienceMeta>
                    </S.AudienceInfo>
                  </S.ReviewItem>
                )
              })}
            </S.AudienceList>
          )}
          {!showCustomAudienceForm && (
            <S.Button
              type="button"
              onClick={() => {
                setShowCustomAudienceForm(true)
                setAnalysisReviewError(null)
              }}
              disabled={savingAnalysisReview || continuingAnalysis}
            >
              + Добавить свою аудиторию
            </S.Button>
          )}
          {showCustomAudienceForm && (
            <>
              <S.Field>
                Название собственной аудитории
                <S.Input
                  value={customAudienceName}
                  onChange={(event) => setCustomAudienceName(event.target.value)}
                  placeholder="Например, новые пользователи"
                />
              </S.Field>
              <S.Field>
                Описание аудитории
                <S.TextArea
                  rows={2}
                  value={customAudienceDescription}
                  onChange={(event) => setCustomAudienceDescription(event.target.value)}
                  placeholder="Кого нужно включить в аудиторию"
                />
              </S.Field>
              <S.ButtonGroup>
                <S.Button type="button" onClick={handleAddCustomAudience} disabled={savingAnalysisReview || continuingAnalysis}>
                  Добавить аудиторию
                </S.Button>
                <S.Button
                  type="button"
                  onClick={() => {
                    setShowCustomAudienceForm(false)
                    setCustomAudienceName('')
                    setCustomAudienceDescription('')
                    setAnalysisReviewError(null)
                  }}
                  disabled={savingAnalysisReview || continuingAnalysis}
                >
                  Отмена
                </S.Button>
              </S.ButtonGroup>
            </>
          )}
          <S.Button
            type="button"
            $success
            onClick={handleSaveAudienceReview}
            disabled={savingAnalysisReview || continuingAnalysis}
          >
            {savingAnalysisReview || continuingAnalysis ? 'Сохраняем…' : 'Сохранить и продолжить'}
          </S.Button>
          {analysisReviewError && <S.ErrorMessage>{analysisReviewError}</S.ErrorMessage>}
        </S.ReviewForm>
      )}
      {hypotheses.length > 0 && (
        <S.AudienceDetails>
          <S.AudienceSummary>Гипотезы</S.AudienceSummary>
          <S.Message>Это черновики для проверки человеком; данные аудиторий синтетические.</S.Message>
          <S.AudienceList>
            {hypotheses.map((hypothesis, index) => (
              <S.AudienceItem key={typeof hypothesis.candidate_id === 'string' ? hypothesis.candidate_id : index}>
                <S.AudienceDetails>
                    <S.AudienceSummary>
                      <S.AudienceInfo>
                        {typeof hypothesis.statement === 'string' && hypothesis.statement.trim() !== '' && (
                          <S.HypothesisStatement>{hypothesis.statement}</S.HypothesisStatement>
                        )}
                        <S.AudienceMeta>
                        {hypothesis.selection_status === 'selected'
                          ? 'Выбрана для рассмотрения'
                          : hypothesis.selection_status === 'rejected_validation'
                            ? 'Не прошла проверку'
                            : 'Запасной кандидат'}
                      </S.AudienceMeta>
                    </S.AudienceInfo>
                  </S.AudienceSummary>
                  <JsonTree value={hypothesis} />
                </S.AudienceDetails>
                {(() => {
                  const id = hypothesisCandidateId(hypothesis, index)
                  const rating = hypothesisRatings[id]
                  return (
                    <S.HypothesisRatingForm>
                      <S.HypothesisRatingLabel>Оценка гипотезы</S.HypothesisRatingLabel>
                      <S.HypothesisRatingButtons>
                        <S.RatingButton
                          type="button"
                          $selected={rating === 'good'}
                          $good
                          aria-pressed={rating === 'good'}
                          onClick={() => {
                            setHypothesisRatings((current) => ({ ...current, [id]: 'good' }))
                            setHypothesisRatingError(null)
                          }}
                        >
                          Хорошо
                        </S.RatingButton>
                        <S.RatingButton
                          type="button"
                          $selected={rating === 'bad'}
                          aria-pressed={rating === 'bad'}
                          onClick={() => {
                            setHypothesisRatings((current) => ({ ...current, [id]: 'bad' }))
                            setHypothesisRatingError(null)
                          }}
                        >
                          Плохо
                        </S.RatingButton>
                      </S.HypothesisRatingButtons>
                    </S.HypothesisRatingForm>
                  )
                })()}
              </S.AudienceItem>
            ))}
          </S.AudienceList>
          <S.HypothesisRatingSummary>
            Оценено: {hypotheses.filter((hypothesis, index) => hypothesisRatings[hypothesisCandidateId(hypothesis, index)]).length} из {hypotheses.length}
          </S.HypothesisRatingSummary>
          <S.Button
            type="button"
            $success
            onClick={handleSaveHypothesisRatings}
            disabled={savingAnalysisReview}
          >
            {savingAnalysisReview ? 'Сохраняем…' : 'Сохранить оценки'}
          </S.Button>
          {hypothesisRatingError && <S.ErrorMessage>{hypothesisRatingError}</S.ErrorMessage>}
        </S.AudienceDetails>
      )}
      <S.Button
        type="button"
        $success
        onClick={handleStartAnalysis}
        disabled={
          startingAnalysis ||
          analysisStatus?.status === 'queued' ||
          analysisStatus?.status === 'running' ||
          analysisStatus?.status === 'waiting_for_user_input'
        }
      >
        {startingAnalysis ? 'Запускаем…' : 'Проанализировать и создать гипотезы'}
      </S.Button>
      {analysisStartError && <S.ErrorMessage>{analysisStartError}</S.ErrorMessage>}
    </S.AnalysisSection>
  )
}

type AnalysisPipelineState = 'completed' | 'active' | 'waiting' | 'pending' | 'error'

type AnalysisPipelineStage = {
  id: ProductAnalysisResponse['current_stage']
  name: string
  state: AnalysisPipelineState
  label: string
}

function getAnalysisPipelineStages(
  status: ProductAnalysisResponse | ProductAnalysisStatusResponse,
  results?: ProductAnalysisResponse,
): AnalysisPipelineStage[] {
  const stageOrder: AnalysisPipelineStage['id'][] = ['product', 'audience', 'hypotheses']
  const currentIndex = stageOrder.indexOf(status.current_stage)
  const terminal = status.status === 'failed' || status.status === 'blocked'
  const resultByStage = {
    product: 'product_result_available' in status ? status.product_result_available : isRecord(results?.product_result),
    audience: 'audience_result_available' in status ? status.audience_result_available : isRecord(results?.audience_result),
    hypotheses: 'hypothesis_result_available' in status
      ? status.hypothesis_result_available
      : isRecord(results?.hypothesis_result),
  }
  const names = {
    product: 'Продукт',
    audience: 'Аудитория',
    hypotheses: 'Гипотезы',
  }

  return stageOrder.map((id, index) => {
    let state: AnalysisPipelineState = 'pending'
    if (terminal && currentIndex === index) {
      state = 'error'
    } else if (id === 'audience' && status.status === 'waiting_for_user_input' && status.current_stage === id) {
      state = 'waiting'
    } else if (resultByStage[id]) {
      state = 'completed'
    } else if (currentIndex === index) {
      state = 'active'
    }

    return {
      id,
      name: names[id],
      state,
      label: getAnalysisPipelineStateLabel(state, status.status),
    }
  })
}

function getAnalysisPipelineStateLabel(state: AnalysisPipelineState, analysisStatus: string): string {
  if (state === 'completed') return 'Завершено'
  if (state === 'active') return analysisStatus === 'queued' ? 'В очереди' : 'В работе'
  if (state === 'waiting') return 'Ждёт выбора'
  if (state === 'error') return analysisStatus === 'blocked' ? 'Остановлено' : 'Ошибка'
  return 'Не запущено'
}

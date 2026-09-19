import { Fragment, useState } from 'react'
import type {
  ProductFeedbackSurveyPersona,
  ProductFeedbackSurveyQuestionnaire,
  ProductFeedbackSurveyResultResponse,
} from '../../../../store/generatedApi'
import * as S from '../../styles'

type JsonObject = Record<string, unknown>

type SurveyResultProps = {
  questionnaire: ProductFeedbackSurveyQuestionnaire
  result: ProductFeedbackSurveyResultResponse
}

function asObject(value: unknown): JsonObject {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as JsonObject : {}
}

function formatJson(value: unknown): string {
  return JSON.stringify(value, null, 2) ?? String(value)
}

function formatPercent(value: unknown): string {
  return typeof value === 'number' ? `${Math.round(value * 100)}%` : 'n/a'
}

function formatScore(value: unknown, answer: JsonObject): string {
  if (typeof value !== 'number') {
    return 'n/a'
  }

  const legend = asObject(answer.legend)
  const isZeroBased = Object.prototype.hasOwnProperty.call(legend, '0')
  const displayedScore = isZeroBased ? value + 1 : value
  return `${displayedScore.toFixed(2)} / 5`
}

function questionLabel(questionnaire: ProductFeedbackSurveyQuestionnaire, key: string): string {
  return questionnaire[key]?.instructions ?? key.replaceAll('_', ' ')
}

function answerLabel(questionnaire: ProductFeedbackSurveyQuestionnaire, questionKey: string, value: unknown): string {
  if (typeof value !== 'string') {
    return formatJson(value)
  }

  const criteria = questionnaire[questionKey]?.criteria
  if (criteria !== null && typeof criteria === 'object' && !Array.isArray(criteria)) {
    const label = (criteria as Record<string, unknown>)[value]
    if (typeof label === 'string') {
      return label
    }
  }

  return value.replaceAll('_', ' ')
}

function confidenceNote(answer: JsonObject): string | null {
  return typeof answer.confidence === 'number' ? `${formatPercent(answer.confidence)} confidence` : null
}

function Distribution({ answer, questionnaire, questionKey }: {
  answer: JsonObject
  questionnaire: ProductFeedbackSurveyQuestionnaire
  questionKey: string
}) {
  const probabilities = asObject(answer.probabilities)
  const legend = asObject(answer.legend)
  const selected = typeof answer.choice === 'string' ? answer.choice : null
  const entries = Object.entries(probabilities).sort(([, left], [, right]) => {
    const leftNumber = typeof left === 'number' ? left : 0
    const rightNumber = typeof right === 'number' ? right : 0
    return rightNumber - leftNumber
  })

  if (entries.length === 0) {
    return null
  }

  return (
    <S.Distribution aria-label="Probability distribution">
      {entries.map(([key, value]) => {
        const probability = typeof value === 'number' ? value : 0
        const label = legend[key] ?? answerLabel(questionnaire, questionKey, key)

        return (
          <S.DistributionRow key={key}>
            <S.DistributionLabel title={String(label)}>
              {selected === key ? `Selected: ${label}` : String(label)}
            </S.DistributionLabel>
            <S.DistributionTrack>
              <S.DistributionBar $value={probability * 100} />
            </S.DistributionTrack>
            <S.DistributionValue>{formatPercent(probability)}</S.DistributionValue>
          </S.DistributionRow>
        )
      })}
    </S.Distribution>
  )
}

function AnswerCard({ answer, questionnaire, questionKey }: {
  answer: unknown
  questionnaire: ProductFeedbackSurveyQuestionnaire
  questionKey: string
}) {
  const answerObject = asObject(answer)
  const type = answerObject.type
  const confidence = confidenceNote(answerObject)

  let selectedAnswer: string | null = null
  if (type === 'score') {
    selectedAnswer = formatScore(answerObject.score, answerObject)
  } else if (type === 'choice') {
    selectedAnswer = answerLabel(questionnaire, questionKey, answerObject.choice)
  } else if (type === 'noul') {
    selectedAnswer = `${formatPercent(answerObject.noul)} estimated likelihood`
  }

  return (
    <S.AnswerCard>
      <S.AnswerHeader>
        <S.AnswerTitle>{questionLabel(questionnaire, questionKey)}</S.AnswerTitle>
        <S.AnswerKey>{questionKey}</S.AnswerKey>
      </S.AnswerHeader>
      {selectedAnswer !== null ? <S.SelectedAnswer>{selectedAnswer}</S.SelectedAnswer> : null}
      {confidence !== null ? <S.AnswerNote>{confidence}</S.AnswerNote> : null}
      <Distribution answer={answerObject} questionnaire={questionnaire} questionKey={questionKey} />
      {selectedAnswer === null ? <S.UnknownValue>{formatJson(answer)}</S.UnknownValue> : null}
    </S.AnswerCard>
  )
}

function RenderedResponse({ questionnaire, response }: {
  questionnaire: ProductFeedbackSurveyQuestionnaire
  response: unknown
}) {
  const responseObject = asObject(response)
  const answers = asObject(responseObject.answers)
  const usage = asObject(responseObject.usage)

  return (
    <S.Panel role="tabpanel">
      <S.ResponseMeta>
        {typeof responseObject.model === 'string' ? <span>Model: {responseObject.model}</span> : null}
        {typeof responseObject.provider === 'string' ? <span>Provider: {responseObject.provider}</span> : null}
        {typeof usage.input_tokens === 'number' ? <span>Input: {usage.input_tokens.toLocaleString()}</span> : null}
        {typeof usage.output_tokens === 'number' ? <span>Output: {usage.output_tokens.toLocaleString()}</span> : null}
      </S.ResponseMeta>
      {Object.keys(answers).length > 0 ? (
        <S.AnswerList>
          {Object.entries(answers).map(([questionKey, answer]) => (
            <AnswerCard key={questionKey} questionnaire={questionnaire} questionKey={questionKey} answer={answer} />
          ))}
        </S.AnswerList>
      ) : (
        <S.EmptyState>No answer fields were returned.</S.EmptyState>
      )}
    </S.Panel>
  )
}

function PersonaProfile({ persona }: { persona: ProductFeedbackSurveyPersona }) {
  const dimensions = Object.entries(persona.dimensions ?? {})

  return (
    <S.PersonaCard>
      <S.PersonaTitle>Persona profile</S.PersonaTitle>
      <S.PersonaFacts>
        <span>{persona.display_name}</span>
        {persona.source ? <span>Source: {persona.source}</span> : null}
        {persona.version ? <span>Version: {persona.version}</span> : null}
        <span>ID: {persona.id}</span>
      </S.PersonaFacts>
      {dimensions.length > 0 ? (
        <S.PersonaDimensions>
          <summary>{dimensions.length} dimensions</summary>
          <S.PersonaDimensionList>
            {dimensions.map(([key, value]) => (
              <Fragment key={key}>
                <S.PersonaDimensionKey>{key}</S.PersonaDimensionKey>
                <S.PersonaDimensionValue>{value}</S.PersonaDimensionValue>
              </Fragment>
            ))}
          </S.PersonaDimensionList>
        </S.PersonaDimensions>
      ) : null}
    </S.PersonaCard>
  )
}

export function SurveyResult({ questionnaire, result }: SurveyResultProps) {
  const [activeTab, setActiveTab] = useState<'rendered' | 'raw'>('rendered')
  const response = result.response as unknown
  const responseObject = asObject(response)
  const answers = asObject(responseObject.answers)
  const interest = asObject(answers.overall_interest)
  const score = formatScore(interest.score, interest)

  return (
    <S.ResultItem>
      <S.ResultDetails>
        <S.ResultSummary>
          <S.PersonaInfo>
            <S.PersonaName>{result.persona_name}</S.PersonaName>
            <S.PersonaMeta>Persona {result.persona_id}</S.PersonaMeta>
          </S.PersonaInfo>
          {score !== 'n/a' ? <S.SummaryScore>Interest {score}</S.SummaryScore> : null}
        </S.ResultSummary>
        <S.ResultBody>
          <PersonaProfile persona={result.persona} />
          <S.Tabs role="tablist" aria-label={`${result.persona_name} response views`}>
            <S.Tab
              type="button"
              role="tab"
              aria-selected={activeTab === 'raw'}
              $active={activeTab === 'raw'}
              onClick={() => setActiveTab('raw')}
            >
              Raw JSON
            </S.Tab>
            <S.Tab
              type="button"
              role="tab"
              aria-selected={activeTab === 'rendered'}
              $active={activeTab === 'rendered'}
              onClick={() => setActiveTab('rendered')}
            >
              Rendered
            </S.Tab>
          </S.Tabs>
          {activeTab === 'raw' ? (
            <S.Panel role="tabpanel">
              <S.RawOutput>{formatJson(response)}</S.RawOutput>
            </S.Panel>
          ) : (
            <RenderedResponse questionnaire={questionnaire} response={response} />
          )}
        </S.ResultBody>
      </S.ResultDetails>
    </S.ResultItem>
  )
}

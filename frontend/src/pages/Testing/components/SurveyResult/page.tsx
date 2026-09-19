import { useState } from 'react'
import type { ProductFeedbackSurveyResultResponse } from '../../../../store/generatedApi'
import * as S from '../../styles'

type JsonObject = Record<string, unknown>

type SurveyResultProps = {
  result: ProductFeedbackSurveyResultResponse
}

const questionLabels: Record<string, string> = {
  overall_interest: 'Overall interest in the product',
  q0: 'Plan after trying the free version',
  q1: 'Plus vs Pro preference',
  q2: 'Annual vs monthly billing',
  q3: 'Response to the $1 first-month promo',
  q4: 'Reaction to a paid organizer app',
  q5: 'Value of removing ads',
  q6: 'Perception of the product pricing',
  would_try_beta: 'Likelihood of trying a beta',
}

const choiceLabels: Record<string, Record<string, string>> = {
  q0: {
    q0_use_free_wont_pay: 'Keep using free and only upgrade if necessary',
    q0_pay_when_roi_clear: 'Upgrade once time savings justify the price',
    q0_subscribe_paid_launch: 'Subscribe to Plus or better at launch',
    q0_free_never_decide_tier: 'Use free and never think much about tiers',
    q0_not_interested: 'Skip the product because it is not relevant',
  },
  q1: {
    q1_reject_both_tiers: 'Would not pay for either tier',
    q1_plus_after_sustained_use: 'Choose Plus after sustained use proves value',
    q1_happy_plus_or_pro: 'Happy with Plus or Pro',
    q1_wont_compare_tiers: 'Would not spend effort comparing tiers',
  },
  q2: {
    q2_monthly_cancel_anytime: 'Monthly only, with the ability to cancel anytime',
    q2_annual_after_long_use: 'Prepay annually after a long trial',
    q2_prepay_annual_plus: 'Prepay annually on Plus immediately',
    q2_billing_no_preference: 'No preference between monthly and annual billing',
  },
  q3: {
    q3_skip_even_one_dollar: 'Skip it because it is not worth $1 or the hassle',
    q3_one_dollar_try_cancel: 'Try it for $1 and cancel unless value is clear',
    q3_grab_dollar_promo: 'Grab the $1 Plus month immediately',
    q3_ignore_promo: 'Ignore it unless already planning to upgrade',
  },
  q4: {
    q4_seek_free_alternative: 'Look for a free alternative first',
    q4_compare_pay_if_wins: 'Compare options and pay only if one clearly wins',
    q4_pay_best_no_hunt: 'Pay for the best app without hunting for free options',
    q4_switch_only_effortless: 'Switch only if setup is effortless',
  },
  q5: {
    q5_ads_not_worth_paying: 'Ads are annoying but not worth paying to remove',
    q5_ads_pay_if_plus_useful: 'Pay only if Plus is useful beyond being ad-free',
    q5_pay_primarily_adfree: 'Removing ads alone is enough value to pay',
    q5_ads_irrelevant_to_tier: 'Ads do not affect the upgrade decision',
  },
  q6: {
    q6_too_expensive_stay_free: 'Too expensive, so stay free',
    q6_fair_if_use_justifies: 'Fair only if daily use clearly justifies it',
    q6_premium_price_ok: 'Reasonable price for premium convenience',
    q6_pricing_unnoticed: 'Does not really notice or care about pricing',
  },
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

function questionLabel(key: string): string {
  return questionLabels[key] ?? key.replaceAll('_', ' ')
}

function answerLabel(questionKey: string, value: unknown): string {
  if (typeof value !== 'string') {
    return formatJson(value)
  }

  return choiceLabels[questionKey]?.[value] ?? value.replaceAll('_', ' ')
}

function confidenceNote(answer: JsonObject): string | null {
  return typeof answer.confidence === 'number' ? `${formatPercent(answer.confidence)} confidence` : null
}

function Distribution({ answer, questionKey }: { answer: JsonObject; questionKey: string }) {
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
        const label = legend[key] ?? answerLabel(questionKey, key)

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

function AnswerCard({ questionKey, answer }: { questionKey: string; answer: unknown }) {
  const answerObject = asObject(answer)
  const type = answerObject.type
  const confidence = confidenceNote(answerObject)

  let selectedAnswer: string | null = null
  if (type === 'score') {
    selectedAnswer = formatScore(answerObject.score, answerObject)
  } else if (type === 'choice') {
    selectedAnswer = answerLabel(questionKey, answerObject.choice)
  } else if (type === 'noul') {
    selectedAnswer = `${formatPercent(answerObject.noul)} estimated likelihood`
  }

  return (
    <S.AnswerCard>
      <S.AnswerHeader>
        <S.AnswerTitle>{questionLabel(questionKey)}</S.AnswerTitle>
        <S.AnswerKey>{questionKey}</S.AnswerKey>
      </S.AnswerHeader>
      {selectedAnswer !== null ? <S.SelectedAnswer>{selectedAnswer}</S.SelectedAnswer> : null}
      {confidence !== null ? <S.AnswerNote>{confidence}</S.AnswerNote> : null}
      <Distribution answer={answerObject} questionKey={questionKey} />
      {selectedAnswer === null ? <S.UnknownValue>{formatJson(answer)}</S.UnknownValue> : null}
    </S.AnswerCard>
  )
}

function RenderedResponse({ response }: { response: unknown }) {
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
            <AnswerCard key={questionKey} questionKey={questionKey} answer={answer} />
          ))}
        </S.AnswerList>
      ) : (
        <S.EmptyState>No answer fields were returned.</S.EmptyState>
      )}
    </S.Panel>
  )
}

export function SurveyResult({ result }: SurveyResultProps) {
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
            <RenderedResponse response={response} />
          )}
        </S.ResultBody>
      </S.ResultDetails>
    </S.ResultItem>
  )
}

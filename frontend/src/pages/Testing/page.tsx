import { useState } from 'react'
import { rtkErrorMessage } from '../../api/structs'
import {
  type ProductFeedbackSurveyResultResponse,
  useListProductFeedbackSurveyResultsQuery,
  useRunProductFeedbackSurveyMutation,
} from '../../store/generatedApi'
import { SurveyResult } from './components/SurveyResult/page'
import * as S from './styles'

type JsonObject = Record<string, unknown>

function asObject(value: unknown): JsonObject {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as JsonObject : {}
}

function interestScore(result: ProductFeedbackSurveyResultResponse): number {
  const response = asObject(result.response)
  const answers = asObject(response.answers)
  const interest = asObject(answers.overall_interest)

  if (typeof interest.score !== 'number') {
    return Number.NEGATIVE_INFINITY
  }

  const legend = asObject(interest.legend)
  return Object.prototype.hasOwnProperty.call(legend, '0') ? interest.score + 1 : interest.score
}

export function TestingPage() {
  const [runSurvey, { isLoading }] = useRunProductFeedbackSurveyMutation()
  const { data: survey, isFetching, refetch } = useListProductFeedbackSurveyResultsQuery()
  const [error, setError] = useState<string | null>(null)
  const results = survey?.results
  const questionnaire = survey?.questionnaire ?? {}
  const orderedResults = results
    ?.map((result, index) => ({ result, index }))
    .sort((left, right) => interestScore(right.result) - interestScore(left.result) || left.index - right.index)
    .map(({ result }) => result)

  async function handleRun() {
    setError(null)

    try {
      await runSurvey().unwrap()
    } catch (err) {
      setError(rtkErrorMessage(err, 'Failed to run product feedback survey'))
    } finally {
      await refetch()
    }
  }

  const output = error === null ? results : { error }

  return (
    <S.Page>
      <S.Header>
        <S.Title>Product feedback survey</S.Title>
        <S.Description>
          Run the survey for all personas and inspect each model response in a readable view or as the original JSON.
        </S.Description>
      </S.Header>
      <S.RunButton type="button" onClick={handleRun} disabled={isLoading || isFetching}>
        {isLoading ? 'Running 200 personas...' : 'Run survey'}
      </S.RunButton>
      {error !== null ? <S.ErrorMessage>{error}</S.ErrorMessage> : null}
      {output !== undefined && output !== null && error === null ? (
        <S.ResultsSection>
          <S.ResultsHeader>
            <S.ResultsTitle>Responses</S.ResultsTitle>
            <S.ResultsCount>{results?.length ?? 0} personas</S.ResultsCount>
          </S.ResultsHeader>
          {isFetching && !results ? <S.Message>Loading responses...</S.Message> : null}
          {results && results.length > 0 ? (
            <S.ResultList>
              {orderedResults?.map((result) => (
                <SurveyResult key={result.persona_id} questionnaire={questionnaire} result={result} />
              ))}
            </S.ResultList>
          ) : null}
          {!isFetching && results && results.length === 0 ? <S.Message>No responses yet.</S.Message> : null}
        </S.ResultsSection>
      ) : null}
    </S.Page>
  )
}

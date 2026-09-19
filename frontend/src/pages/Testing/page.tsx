import { useState } from 'react'
import { rtkErrorMessage } from '../../api/structs'
import {
  useListProductFeedbackSurveyResultsQuery,
  useRunProductFeedbackSurveyMutation,
} from '../../store/generatedApi'
import { SurveyResult } from './components/SurveyResult/page'
import * as S from './styles'

export function TestingPage() {
  const [runSurvey, { isLoading }] = useRunProductFeedbackSurveyMutation()
  const { data: results, isFetching, refetch } = useListProductFeedbackSurveyResultsQuery()
  const [error, setError] = useState<string | null>(null)

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
              {results.map((result) => <SurveyResult key={result.persona_id} result={result} />)}
            </S.ResultList>
          ) : null}
          {!isFetching && results && results.length === 0 ? <S.Message>No responses yet.</S.Message> : null}
        </S.ResultsSection>
      ) : null}
    </S.Page>
  )
}

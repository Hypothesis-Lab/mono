import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { rtkErrorMessage } from '../../api/structs'
import { useCreateHypothesisMutation } from '../../store/generatedApi'
import * as S from './styles'

export function CreateHypothesisPage() {
  const navigate = useNavigate()
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [createHypothesis, { isLoading: submitting }] = useCreateHypothesisMutation()

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()

    if (description.trim() === '') {
      setError('Описание обязательно')
      return
    }

    setError(null)

    try {
      const created = await createHypothesis({
        createHypothesisRequest: { description: description.trim() },
      }).unwrap()
      navigate(`/edit-hypothesis/${created.id}`)
    } catch (err) {
      setError(rtkErrorMessage(err, 'Что-то пошло не так'))
    }
  }

  return (
    <S.Page>
      <S.Title>Создать гипотезу</S.Title>

      <S.Form onSubmit={handleSubmit}>
        <S.Field>
          Описание
          <S.TextArea
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Что вы хотите проверить?"
            disabled={submitting}
          />
        </S.Field>

        {error && <S.ErrorMessage>{error}</S.ErrorMessage>}

        <S.Button type="submit" $primary disabled={submitting}>
          {submitting ? 'Создаём…' : 'Создать'}
        </S.Button>
      </S.Form>
    </S.Page>
  )
}

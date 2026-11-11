import { Link, useNavigate } from 'react-router-dom'
import { getStatusLabel, rtkErrorMessage } from '../../api/structs'
import { useListHypothesesQuery } from '../../store/generatedApi'
import * as S from './styles'

export function DashboardMainPage() {
  const navigate = useNavigate()
  const { data: hypotheses, isLoading: loading, error } = useListHypothesesQuery()

  return (
    <S.Page>
      <S.Title>Гипотезы</S.Title>

      <S.CreateButton
        type="button"
        onClick={() => navigate('/create-hypothesis')}
      >
        Создать гипотезу
      </S.CreateButton>

      <S.Section>
        {loading && <S.Message>Загрузка гипотез…</S.Message>}

        {error && (
          <S.ErrorMessage>
            {rtkErrorMessage(error, 'Не удалось загрузить гипотезы')}
          </S.ErrorMessage>
        )}

        {!loading && !error && hypotheses?.length === 0 && (
          <S.Message>Пока нет гипотез.</S.Message>
        )}

        {!loading && !error && hypotheses && hypotheses.length > 0 && (
          <S.List>
            {hypotheses.map((hypothesis) => (
              <S.Item key={hypothesis.id}>
                <S.ItemContent>
                  <S.ItemName as={Link} to={`/hypothesis/${hypothesis.id}`}>
                    {hypothesis.description}
                  </S.ItemName>
                  <S.ItemMeta>Статус: {getStatusLabel(hypothesis.status)}</S.ItemMeta>
                </S.ItemContent>
              </S.Item>
            ))}
          </S.List>
        )}
      </S.Section>
    </S.Page>
  )
}

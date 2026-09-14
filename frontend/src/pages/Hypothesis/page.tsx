import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { skipToken } from '@reduxjs/toolkit/query'
import { getStatusLabel, rtkErrorMessage } from '../../api/structs'
import {
  useGetHypothesisProductQuery,
  useGetHypothesisQuery,
  useLaunchHypothesisMutation,
  useStopHypothesisMutation,
} from '../../store/generatedApi'
import * as S from './styles'

export function HypothesisPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()

  const { data: hypothesis, isLoading: loading, error } = useGetHypothesisQuery(
    id ? { id } : skipToken,
  )
  const { data: product, isLoading: productLoading } = useGetHypothesisProductQuery(
    id ? { id } : skipToken,
  )
  const [launchHypothesis, { isLoading: launching }] = useLaunchHypothesisMutation()
  const [stopHypothesis, { isLoading: stopping }] = useStopHypothesisMutation()

  const [launchError, setLaunchError] = useState<string | null>(null)
  const [stopError, setStopError] = useState<string | null>(null)

  if (!id) {
    return (
      <S.Page>
        <S.ErrorMessage>Отсутствует идентификатор гипотезы</S.ErrorMessage>
      </S.Page>
    )
  }

  async function handleLaunch() {
    if (!id) {
      return
    }

    setLaunchError(null)

    try {
      await launchHypothesis({ id }).unwrap()
    } catch (err) {
      setLaunchError(rtkErrorMessage(err, 'Не удалось запустить'))
    }
  }

  async function handleStop() {
    if (!id) {
      return
    }

    setStopError(null)

    try {
      await stopHypothesis({ id }).unwrap()
    } catch (err) {
      setStopError(rtkErrorMessage(err, 'Не удалось остановить'))
    }
  }

  return (
    <S.Page>
      {loading && <S.Message>Загрузка гипотезы…</S.Message>}

      {error && (
        <S.ErrorMessage>
          {rtkErrorMessage(error, 'Не удалось загрузить гипотезу')}
        </S.ErrorMessage>
      )}

      {!loading && !error && hypothesis && (
        <>
          <S.Title>{hypothesis.description}</S.Title>
          <S.Meta>Статус: {getStatusLabel(hypothesis.status)}</S.Meta>

          <S.ProductSection>
            {productLoading && <S.Message>Загрузка продукта…</S.Message>}
            {product && (
              <>
                <S.SectionTitle>Продукт</S.SectionTitle>
                <S.ProductName>{product.name}</S.ProductName>
                {product.description && (
                  <S.Description>{product.description}</S.Description>
                )}
              </>
            )}
            {!productLoading && !product && (
              <>
                <S.SectionTitle>Продукт</S.SectionTitle>
                <S.Message>Нет продукта</S.Message>
              </>
            )}
          </S.ProductSection>

          <S.ButtonGroup>
            <S.EditButton
              type="button"
              disabled={hypothesis.status !== 'idle'}
              onClick={() => navigate(`/edit-hypothesis/${id}`)}
            >
              Редактировать
            </S.EditButton>
            {hypothesis.status === 'idle' && (
              <S.LaunchButton
                type="button"
                onClick={handleLaunch}
                disabled={launching}
              >
                {launching ? 'Запускаем…' : 'Запустить'}
              </S.LaunchButton>
            )}
            {hypothesis.status === 'in_progress' && (
              <S.StopButton
                type="button"
                onClick={handleStop}
                disabled={stopping}
              >
                {stopping ? 'Останавливаем…' : 'Остановить'}
              </S.StopButton>
            )}
          </S.ButtonGroup>

          {launchError && <S.ErrorMessage>{launchError}</S.ErrorMessage>}
          {stopError && <S.ErrorMessage>{stopError}</S.ErrorMessage>}
        </>
      )}
    </S.Page>
  )
}

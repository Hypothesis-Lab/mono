import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { skipToken } from '@reduxjs/toolkit/query'
import { getCategoryLabel, getStatusLabel, rtkErrorMessage } from '../../api/structs'
import {
  useDeleteHypothesisMutation,
  useGetHypothesisProductQuery,
  useGetHypothesisQuery,
  useUpdateHypothesisMutation,
} from '../../store/generatedApi'
import * as S from './styles'

export function EditHypothesisPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()

  const { data: hypothesis, isLoading: loading, error } = useGetHypothesisQuery(
    id ? { id } : skipToken,
  )
  const { data: product, isLoading: productLoading } = useGetHypothesisProductQuery(
    id ? { id } : skipToken,
  )

  const [updateHypothesis, { isLoading: saving }] = useUpdateHypothesisMutation()
  const [deleteHypothesis, { isLoading: deleting }] = useDeleteHypothesisMutation()

  const [description, setDescription] = useState('')
  const [saveError, setSaveError] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  useEffect(() => {
    if (hypothesis) {
      setDescription(hypothesis.description ?? '')
    }
  }, [hypothesis, hypothesis?.id, hypothesis?.description])

  if (!id) {
    return (
      <S.Page>
        <S.ErrorMessage>Отсутствует идентификатор гипотезы</S.ErrorMessage>
      </S.Page>
    )
  }

  if (loading) {
    return (
      <S.Page>
        <S.Message>Загрузка…</S.Message>
      </S.Page>
    )
  }

  if (error) {
    return (
      <S.Page>
        <S.ErrorMessage>
          {rtkErrorMessage(error, 'Не удалось загрузить гипотезу')}
        </S.ErrorMessage>
      </S.Page>
    )
  }

  if (!hypothesis) {
    return (
      <S.Page>
        <S.ErrorMessage>Гипотеза не найдена</S.ErrorMessage>
      </S.Page>
    )
  }

  const hypothesisId = id

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()

    const trimmedDescription = description.trim()

    if (trimmedDescription === '') {
      setSaveError('Описание обязательно')
      return
    }

    setSaveError(null)

    const body: { description?: string } = {}
    if (trimmedDescription !== hypothesis?.description) {
      body.description = trimmedDescription
    }

    try {
      await updateHypothesis({ id: hypothesisId, updateHypothesisRequest: body }).unwrap()
    } catch (err) {
      setSaveError(rtkErrorMessage(err, 'Не удалось сохранить'))
    }
  }

  async function handleDelete() {
    setDeleteError(null)

    try {
      await deleteHypothesis({ id: hypothesisId }).unwrap()
      navigate('/')
    } catch (err) {
      setDeleteError(rtkErrorMessage(err, 'Не удалось удалить'))
    }
  }

  return (
    <S.Page>
      <S.Button
        type="button"
        onClick={() => navigate(`/hypothesis/${id}`)}
      >
        Назад к гипотезе
      </S.Button>

      <S.Title>Редактировать гипотезу</S.Title>
      <S.Meta>Статус: {getStatusLabel(hypothesis.status)}</S.Meta>

      <S.Form onSubmit={handleSubmit}>
        <S.HypothesisFields>
          <S.Field>
            Описание
            <S.TextArea
              rows={3}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Что вы хотите проверить?"
              disabled={saving}
            />
          </S.Field>
        </S.HypothesisFields>

        {saveError && <S.ErrorMessage>{saveError}</S.ErrorMessage>}

        <S.ButtonGroup>
          <S.Button type="submit" $primary disabled={saving}>
            {saving ? 'Сохраняем…' : 'Сохранить'}
          </S.Button>
          <S.Button
            type="button"
            $danger
            onClick={handleDelete}
            disabled={deleting}
          >
            {deleting ? 'Удаляем…' : 'Удалить гипотезу'}
          </S.Button>
        </S.ButtonGroup>

        {deleteError && <S.ErrorMessage>{deleteError}</S.ErrorMessage>}
      </S.Form>

      <S.ProductSection>
        <S.SectionTitle>Продукт</S.SectionTitle>

        {productLoading && <S.Message>Загрузка продукта…</S.Message>}

        {!productLoading && product && (
          <>
            <S.ProductName>{product.name}</S.ProductName>
            <S.ProductCategory>
              Категория: {getCategoryLabel(product.category)}
            </S.ProductCategory>
            <S.Button
              type="button"
              onClick={() => navigate(`/hypothesis/${id}/product`)}
            >
              Редактировать продукт
            </S.Button>
          </>
        )}

        {!productLoading && !product && (
          <S.Button
            type="button"
            onClick={() => navigate(`/hypothesis/${id}/product`)}
          >
            Создать продукт
          </S.Button>
        )}
      </S.ProductSection>
    </S.Page>
  )
}
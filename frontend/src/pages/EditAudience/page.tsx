import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { skipToken } from '@reduxjs/toolkit/query'
import type { AudienceGender } from '../../api/structs'
import {
  AUDIENCE_GENDERS,
  getGenderLabel,
  isApiNotFound,
  rtkErrorMessage,
} from '../../api/structs'
import {
  useCreateProductAudienceMutation,
  useDeleteAudienceMutation,
  useGetAudienceQuery,
  useGetHypothesisProductQuery,
  useUpdateAudienceMutation,
} from '../../store/generatedApi'
import * as S from './styles'

function parseIntOrNull(value: string): number | null {
  const trimmed = value.trim()
  if (trimmed === '') {
    return null
  }
  const parsed = Number(trimmed)
  if (Number.isNaN(parsed)) {
    return null
  }
  return parsed
}

export function EditAudiencePage() {
  const navigate = useNavigate()
  const { hid, id } = useParams<{ hid: string; id?: string }>()

  const hypothesisArg = hid ? { id: hid } : skipToken

  const {
    data: product,
    isLoading: productLoading,
    error: productError,
  } = useGetHypothesisProductQuery(hypothesisArg)
  const {
    data: audience,
    isLoading: audienceLoading,
    error: audienceError,
  } = useGetAudienceQuery(id ? { id } : skipToken)

  const [createAudience, { isLoading: creating }] = useCreateProductAudienceMutation()
  const [updateAudience, { isLoading: updating }] = useUpdateAudienceMutation()
  const [deleteAudience, { isLoading: deleting }] = useDeleteAudienceMutation()

  const isCreate = !id
  const loading = productLoading || (!isCreate && audienceLoading)

  const [description, setDescription] = useState('')
  const [ageFrom, setAgeFrom] = useState('')
  const [ageTo, setAgeTo] = useState('')
  const [gender, setGender] = useState<AudienceGender>('all')
  const [interests, setInterests] = useState('')
  const [geography, setGeography] = useState('')
  const [painPoints, setPainPoints] = useState('')
  const [desiresMotivation, setDesiresMotivation] = useState('')

  const [saveError, setSaveError] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  useEffect(() => {
    if (audience) {
      setDescription(audience.description)
      setAgeFrom(audience.age_from?.toString() ?? '')
      setAgeTo(audience.age_to?.toString() ?? '')
      setGender(audience.gender ?? 'all')
      setInterests(audience.interests ?? '')
      setGeography(audience.geography ?? '')
      setPainPoints(audience.pain_points ?? '')
      setDesiresMotivation(audience.desires_motivation ?? '')
    } else if (isCreate) {
      setDescription('')
      setAgeFrom('')
      setAgeTo('')
      setGender('all')
      setInterests('')
      setGeography('')
      setPainPoints('')
      setDesiresMotivation('')
    }
  }, [audience, isCreate])

  if (!hid) {
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

  if (productError && !isApiNotFound(productError)) {
    return (
      <S.Page>
        <S.ErrorMessage>
          {rtkErrorMessage(productError, 'Не удалось загрузить продукт')}
        </S.ErrorMessage>
      </S.Page>
    )
  }

  if (!product) {
    return (
      <S.Page>
        <S.ErrorMessage>
          Для создания аудитории нужен продукт
        </S.ErrorMessage>
      </S.Page>
    )
  }

  if (!isCreate && audienceError) {
    return (
      <S.Page>
        <S.ErrorMessage>
          {rtkErrorMessage(audienceError, 'Не удалось загрузить аудиторию')}
        </S.ErrorMessage>
      </S.Page>
    )
  }

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()

    if (description.trim() === '') {
      setSaveError('Описание обязательно')
      return
    }

    setSaveError(null)

    const trimmedDescription = description.trim()
    const nextAgeFrom = parseIntOrNull(ageFrom)
    const nextAgeTo = parseIntOrNull(ageTo)
    const nextGender = gender
    const trimmedInterests = interests.trim()
    const trimmedGeography = geography.trim()
    const trimmedPainPoints = painPoints.trim()
    const trimmedDesiresMotivation = desiresMotivation.trim()

    try {
      if (isCreate) {
        await createAudience({
          id: product!.id,
          createAudienceRequest: {
            description: trimmedDescription,
            age_from: nextAgeFrom,
            age_to: nextAgeTo,
            gender: nextGender,
            interests: trimmedInterests === '' ? null : trimmedInterests,
            geography: trimmedGeography === '' ? null : trimmedGeography,
            pain_points: trimmedPainPoints === '' ? null : trimmedPainPoints,
            desires_motivation:
              trimmedDesiresMotivation === '' ? null : trimmedDesiresMotivation,
          },
        }).unwrap()
      } else {
        if (!audience) {
          return
        }

        const body: {
          description?: string | null
          age_from?: number | null
          age_to?: number | null
          gender?: AudienceGender
          interests?: string | null
          geography?: string | null
          pain_points?: string | null
          desires_motivation?: string | null
        } = {}

        if (trimmedDescription !== audience.description) {
          body.description = trimmedDescription
        }
        if (nextAgeFrom !== audience.age_from) {
          body.age_from = nextAgeFrom
        }
        if (nextAgeTo !== audience.age_to) {
          body.age_to = nextAgeTo
        }
        if (nextGender !== audience.gender) {
          body.gender = nextGender
        }
        if (trimmedInterests !== (audience.interests ?? '')) {
          body.interests = trimmedInterests
        }
        if (trimmedGeography !== (audience.geography ?? '')) {
          body.geography = trimmedGeography
        }
        if (trimmedPainPoints !== (audience.pain_points ?? '')) {
          body.pain_points = trimmedPainPoints
        }
        if (trimmedDesiresMotivation !== (audience.desires_motivation ?? '')) {
          body.desires_motivation = trimmedDesiresMotivation
        }

        await updateAudience({ id: id!, updateAudienceRequest: body }).unwrap()
      }

      navigate(`/hypothesis/${hid}/product`)
    } catch (err) {
      setSaveError(rtkErrorMessage(err, 'Не удалось сохранить'))
    }
  }

  async function handleDelete() {
    if (!id) {
      return
    }

    setDeleteError(null)

    try {
      await deleteAudience({ id }).unwrap()
      navigate(`/hypothesis/${hid}/product`)
    } catch (err) {
      setDeleteError(rtkErrorMessage(err, 'Не удалось удалить'))
    }
  }

  const saving = creating || updating

  return (
    <S.Page>
      <S.Button
        type="button"
        onClick={() => navigate(`/hypothesis/${hid}/product`)}
      >
        Назад к продукту
      </S.Button>

      <S.Title>{isCreate ? 'Создать аудиторию' : 'Редактировать аудиторию'}</S.Title>

      <S.Form onSubmit={handleSubmit}>
        <S.Field>
          Описание
          <S.TextArea
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Для кого этот продукт?"
            disabled={saving}
          />
        </S.Field>

        <S.FieldRow>
          <S.Field>
            Возраст от
            <S.Input
              type="number"
              min={0}
              max={100}
              value={ageFrom}
              onChange={(e) => setAgeFrom(e.target.value)}
              placeholder="0"
              disabled={saving}
            />
          </S.Field>

          <S.Field>
            Возраст до
            <S.Input
              type="number"
              min={0}
              max={100}
              value={ageTo}
              onChange={(e) => setAgeTo(e.target.value)}
              placeholder="100"
              disabled={saving}
            />
          </S.Field>
        </S.FieldRow>

        <S.Field>
          Пол
          <S.Select
            value={gender}
            onChange={(e) => setGender(e.target.value as AudienceGender)}
            disabled={saving}
          >
            {AUDIENCE_GENDERS.map((g) => (
              <option key={g} value={g}>
                {getGenderLabel(g)}
              </option>
            ))}
          </S.Select>
        </S.Field>

        <S.Field>
          Интересы
          <S.TextArea
            rows={2}
            value={interests}
            onChange={(e) => setInterests(e.target.value)}
            placeholder="Хобби, темы, бренды, которые им важны"
            disabled={saving}
          />
        </S.Field>

        <S.Field>
          География
          <S.Input
            type="text"
            value={geography}
            onChange={(e) => setGeography(e.target.value)}
            placeholder="Страна, город, регион"
            disabled={saving}
          />
        </S.Field>

        <S.Field>
          Боли
          <S.TextArea
            rows={2}
            value={painPoints}
            onChange={(e) => setPainPoints(e.target.value)}
            placeholder="Проблемы, с которыми сталкивается аудитория"
            disabled={saving}
          />
        </S.Field>

        <S.Field>
          Желания / мотивация
          <S.TextArea
            rows={2}
            value={desiresMotivation}
            onChange={(e) => setDesiresMotivation(e.target.value)}
            placeholder="Что мотивирует их покупать?"
            disabled={saving}
          />
        </S.Field>

        {saveError && <S.ErrorMessage>{saveError}</S.ErrorMessage>}

        <S.ButtonGroup>
          <S.Button type="submit" $primary disabled={saving}>
            {saving ? 'Сохраняем…' : 'Сохранить'}
          </S.Button>
          {!isCreate && (
            <S.Button
              type="button"
              $danger
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? 'Удаляем…' : 'Удалить аудиторию'}
            </S.Button>
          )}
        </S.ButtonGroup>

        {deleteError && <S.ErrorMessage>{deleteError}</S.ErrorMessage>}
      </S.Form>
    </S.Page>
  )
}
import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { skipToken } from '@reduxjs/toolkit/query'
import type { ProductCategory, ProductFileType } from '../../api/structs'
import {
  PRODUCT_CATEGORIES,
  getCategoryLabel,
  getFileTypeLabel,
  isApiNotFound,
  rtkErrorMessage,
} from '../../api/structs'
import {
  useCreateProductMutation,
  useDeleteProductFileMutation,
  useDeleteProductMutation,
  useGetProductAnalysisQuery,
  useGetHypothesisProductQuery,
  useListProductFilesQuery,
  useStartProductAnalysisMutation,
  useUpdateProductMutation,
  useUploadProductFileMutation,
} from '../../store/generatedApi'
import type { ProductPrice, UpdateProductRequest } from '../../store/generatedApi'
import { JsonTree } from './components/JsonTree/page'
import * as S from './styles'

const MAX_FILE_SIZE = 50 * 1024 * 1024

function formatFileSize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString()
}

function listFromText(value: string): string[] {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter((item) => item !== '')
}

function listToText(value: string[]): string {
  return value.join('\n')
}

function areListsEqual(left: string[], right: string[]): boolean {
  return left.length === right.length && left.every((item, index) => item === right[index])
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return isRecord(value) ? value : null
}

export function ProductPage() {
  const navigate = useNavigate()
  const { hid } = useParams<{ hid: string }>()

  const productArg = hid ? { id: hid } : skipToken

  const {
    data: product,
    isLoading: productLoading,
    error: productError,
  } = useGetHypothesisProductQuery(productArg)
  const { data: files, isLoading: filesLoading } = useListProductFilesQuery(productArg)
  const {
    data: analysis,
    error: analysisError,
  } = useGetProductAnalysisQuery(
    product ? { id: product.id } : skipToken,
    { pollingInterval: product ? 10000 : 0 },
  )

  const [createProduct, { isLoading: creating }] = useCreateProductMutation()
  const [updateProduct, { isLoading: updating }] = useUpdateProductMutation()
  const [deleteProduct, { isLoading: deleting }] = useDeleteProductMutation()
  const [uploadProductFile, { isLoading: uploading }] = useUploadProductFileMutation()
  const [deleteProductFile] = useDeleteProductFileMutation()
  const [startAnalysis, { isLoading: startingAnalysis }] = useStartProductAnalysisMutation()

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState<ProductCategory | ''>('')
  const [customCategory, setCustomCategory] = useState('')
  const [brand, setBrand] = useState('')
  const [country, setCountry] = useState('')
  const [town, setTown] = useState('')
  const [experimentGoal, setExperimentGoal] = useState('')
  const [goalOrigin, setGoalOrigin] = useState<'demo_assumption' | 'user_provided'>('user_provided')
  const [targetAction, setTargetAction] = useState('')
  const [primaryMetric, setPrimaryMetric] = useState('')
  const [metricDirection, setMetricDirection] = useState<'increase' | 'decrease'>('increase')
  const [metricNumerator, setMetricNumerator] = useState('')
  const [metricDenominator, setMetricDenominator] = useState('')
  const [successCriterion, setSuccessCriterion] = useState('')
  const [allowedChannels, setAllowedChannels] = useState('')
  const [forbiddenClaims, setForbiddenClaims] = useState('')
  const [knownConstraints, setKnownConstraints] = useState('')
  const [unknowns, setUnknowns] = useState('')
  const [keywords, setKeywords] = useState('')
  const [priceStatus, setPriceStatus] = useState<ProductPrice['status']>('unknown')
  const [priceDescription, setPriceDescription] = useState('Цена не указана.')
  const [link, setLink] = useState('')

  const [saveError, setSaveError] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [fileType, setFileType] = useState<ProductFileType>('other')
  const fileInputRef = useRef<HTMLInputElement>(null)

  const [uploadError, setUploadError] = useState<string | null>(null)
  const [fileDeleteError, setFileDeleteError] = useState<string | null>(null)
  const [deletingFileId, setDeletingFileId] = useState<string | null>(null)
  const [analysisStartError, setAnalysisStartError] = useState<string | null>(null)


  const hasBrandbook = (files ?? []).some((file) => file.type === 'brandbook')
  const noProduct = !product && isApiNotFound(productError)

  useEffect(() => {
    if (product) {
      setName(product.name)
      setDescription(product.description)
      setCategory(product.category)
      setCustomCategory(product.custom_category ?? '')
      setBrand(product.brand)
      setCountry(product.country)
      setTown(product.town ?? '')
      setExperimentGoal(product.experiment_goal)
      setGoalOrigin(product.goal_origin)
      setTargetAction(product.target_action)
      setPrimaryMetric(product.primary_metric)
      setMetricDirection(product.metric_direction)
       setMetricNumerator(product.metric_numerator)
       setMetricDenominator(product.metric_denominator)
       setSuccessCriterion(product.success_criterion ?? '')
       setAllowedChannels(listToText(product.allowed_channels))
       setForbiddenClaims(listToText(product.forbidden_claims))
      setKnownConstraints(listToText(product.known_constraints))
      setUnknowns(listToText(product.unknowns))
      setKeywords(product.keywords.join(', '))
      setPriceStatus(product.price.status)
      setPriceDescription(product.price.description)
      setLink(product.link ?? '')
    } else if (noProduct) {
      setName('')
      setDescription('')
      setCategory('')
      setCustomCategory('')
      setBrand('')
      setCountry('')
      setTown('')
      setExperimentGoal('')
      setGoalOrigin('user_provided')
      setTargetAction('')
      setPrimaryMetric('')
      setMetricDirection('increase')
      setMetricNumerator('')
      setMetricDenominator('')
      setSuccessCriterion('')
       setAllowedChannels('')
      setForbiddenClaims('')
      setKnownConstraints('')
      setUnknowns('')
      setKeywords('')
      setPriceStatus('unknown')
      setPriceDescription('Цена не указана.')
      setLink('')
    }
  }, [product, noProduct])

  useEffect(() => {
    if (hasBrandbook && fileType === 'brandbook') {
      setFileType('other')
    }
  }, [hasBrandbook, fileType])

  if (!hid) {
    return (
      <S.Page>
        <S.ErrorMessage>Отсутствует идентификатор гипотезы</S.ErrorMessage>
      </S.Page>
    )
  }

  if (productLoading) {
    return (
      <S.Page>
        <S.Message>Загрузка…</S.Message>
      </S.Page>
    )
  }

  if (productError && !noProduct) {
    return (
      <S.Page>
        <S.ErrorMessage>
          {rtkErrorMessage(productError, 'Не удалось загрузить продукт')}
        </S.ErrorMessage>
      </S.Page>
    )
  }

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()

    setSaveError(null)

    const trimmedName = name.trim()
    const trimmedDescription = description.trim()
    const trimmedCustomCategory = customCategory.trim()
    const trimmedBrand = brand.trim()
    const trimmedCountry = country.trim()
    const trimmedTown = town.trim()
    const trimmedExperimentGoal = experimentGoal.trim()
    const trimmedTargetAction = targetAction.trim()
    const trimmedPrimaryMetric = primaryMetric.trim()
    const trimmedMetricNumerator = metricNumerator.trim()
    const trimmedMetricDenominator = metricDenominator.trim()
    const trimmedSuccessCriterion = successCriterion.trim()
    let parsedAllowedChannels: string[]
    let parsedForbiddenClaims: string[]
    let parsedKnownConstraints: string[]
    let parsedUnknowns: string[]
    try {
      parsedAllowedChannels = listFromText(allowedChannels)
      parsedForbiddenClaims = listFromText(forbiddenClaims)
      parsedKnownConstraints = listFromText(knownConstraints)
      parsedUnknowns = listFromText(unknowns)
    } catch (err) {
      setSaveError(rtkErrorMessage(err, 'Проверьте данные продукта'))
      return
    }
    const trimmedKeywords = keywords
      .split(',')
      .map((k) => k.trim())
      .filter((k) => k !== '')
    const parsedPrice: ProductPrice = {
      status: priceStatus,
      description: priceDescription.trim(),
    }
    const trimmedLink = link.trim()

    if (category === 'other' && trimmedCustomCategory === '') {
      setSaveError('Укажите свою категорию продукта')
      return
    }

    try {
      if (product) {
        const body: UpdateProductRequest = {}

        if (trimmedName !== product.name) {
          body.name = trimmedName
        }
        if (trimmedDescription !== product.description) {
          body.description = trimmedDescription
        }
        if (category !== product.category) {
          body.category = category as ProductCategory
        }
        if (category === 'other' && trimmedCustomCategory !== (product.custom_category ?? '')) {
          body.custom_category = trimmedCustomCategory
        }
        if (trimmedBrand !== product.brand) {
          body.brand = trimmedBrand
        }
        if (trimmedCountry !== product.country) {
          body.country = trimmedCountry
        }
        if (trimmedTown !== (product.town ?? '')) {
          body.town = trimmedTown
        }
        if (trimmedExperimentGoal !== product.experiment_goal) {
          body.experiment_goal = trimmedExperimentGoal
        }
        if (trimmedTargetAction !== product.target_action) {
          body.target_action = trimmedTargetAction
        }
        if (goalOrigin !== product.goal_origin) body.goal_origin = goalOrigin
        if (trimmedPrimaryMetric !== product.primary_metric) body.primary_metric = trimmedPrimaryMetric
        if (metricDirection !== product.metric_direction) body.metric_direction = metricDirection
        if (trimmedMetricNumerator !== product.metric_numerator) body.metric_numerator = trimmedMetricNumerator
        if (trimmedMetricDenominator !== product.metric_denominator) body.metric_denominator = trimmedMetricDenominator
        if (trimmedSuccessCriterion !== (product.success_criterion ?? '')) body.success_criterion = trimmedSuccessCriterion || null
        if (JSON.stringify(parsedPrice) !== JSON.stringify(product.price)) body.price = parsedPrice
        if (!areListsEqual(parsedAllowedChannels, product.allowed_channels)) {
          body.allowed_channels = parsedAllowedChannels
        }
        if (!areListsEqual(parsedForbiddenClaims, product.forbidden_claims)) {
          body.forbidden_claims = parsedForbiddenClaims
        }
        if (!areListsEqual(parsedKnownConstraints, product.known_constraints)) {
          body.known_constraints = parsedKnownConstraints
        }
        if (!areListsEqual(parsedUnknowns, product.unknowns)) body.unknowns = parsedUnknowns
        if (!areListsEqual(trimmedKeywords, product.keywords)) {
          body.keywords = trimmedKeywords
        }
        if (trimmedLink !== (product.link ?? '')) {
          body.link = trimmedLink
        }

        await updateProduct({ id: product.id, updateProductRequest: body }).unwrap()
      } else {
        await createProduct({
          createProductRequest: {
            name: trimmedName,
            description: trimmedDescription,
            category: category as ProductCategory,
            custom_category: category === 'other' ? trimmedCustomCategory : undefined,
            brand: trimmedBrand,
            country: trimmedCountry,
            town: trimmedTown === '' ? null : trimmedTown,
            experiment_goal: trimmedExperimentGoal,
            goal_origin: goalOrigin,
            target_action: trimmedTargetAction,
            primary_metric: trimmedPrimaryMetric,
            metric_direction: metricDirection,
            metric_numerator: trimmedMetricNumerator,
            metric_denominator: trimmedMetricDenominator,
            success_criterion: trimmedSuccessCriterion || null,
            price: parsedPrice,
            allowed_channels: parsedAllowedChannels,
            forbidden_claims: parsedForbiddenClaims,
            known_constraints: parsedKnownConstraints,
            unknowns: parsedUnknowns,
            keywords: trimmedKeywords,
            link: trimmedLink === '' ? null : trimmedLink,
            hypothesis_id: hid!,
          },
        }).unwrap()
      }
    } catch (err) {
      setSaveError(rtkErrorMessage(err, 'Не удалось сохранить'))
    }
  }

  async function handleDelete() {
    if (!product) {
      return
    }

    setDeleteError(null)

    try {
      await deleteProduct({ id: product.id }).unwrap()
      navigate(`/edit-hypothesis/${hid}`)
    } catch (err) {
      setDeleteError(rtkErrorMessage(err, 'Не удалось удалить'))
    }
  }

  async function handleUpload(event: React.FormEvent) {
    event.preventDefault()

    if (!selectedFile) {
      setUploadError('Выберите файл для загрузки')
      return
    }

    if (selectedFile.size > MAX_FILE_SIZE) {
      setUploadError('Файл превышает лимит 50 МБ')
      return
    }

    setUploadError(null)

    try {
      await uploadProductFile({
        id: hid!,
        body: { file: selectedFile, type: fileType },
      }).unwrap()

      setSelectedFile(null)
      setFileType('other')

      if (fileInputRef.current) {
        fileInputRef.current.value = ''
      }
    } catch (err) {
      setUploadError(rtkErrorMessage(err, 'Не удалось загрузить'))
    }
  }

  async function handleDeleteFile(fileId: string) {
    setDeletingFileId(fileId)
    setFileDeleteError(null)

    try {
      await deleteProductFile({ id: fileId }).unwrap()
    } catch (err) {
      setFileDeleteError(rtkErrorMessage(err, 'Не удалось удалить'))
    } finally {
      setDeletingFileId(null)
    }
  }

  async function handleStartAnalysis() {
    if (!product) {
      return
    }

    setAnalysisStartError(null)

    try {
      await startAnalysis({ id: product.id }).unwrap()
    } catch (err) {
      setAnalysisStartError(rtkErrorMessage(err, 'Не удалось начать анализ'))
    }
  }

  const saving = creating || updating
  const filesList = files ?? []
  const hypothesisResult = asRecord(analysis?.hypothesis_result)
  const hypotheses = Array.isArray(hypothesisResult?.hypotheses)
    ? hypothesisResult.hypotheses.filter(isRecord)
    : []

  return (
    <S.Page>
      <S.Button type="button" onClick={() => navigate(`/edit-hypothesis/${hid}`)}>
        Назад к гипотезе
      </S.Button>

      <S.Title>{product ? 'Редактировать продукт' : 'Создать продукт'}</S.Title>

      <S.Form onSubmit={handleSubmit}>
        <S.Field>
          Название
          <S.Input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Название продукта"
            disabled={saving}
            required
          />
        </S.Field>

        <S.Field>
          Описание
          <S.TextArea
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Краткое описание продукта"
            disabled={saving}
            required
          />
        </S.Field>

        <S.Field>
          Бренд
          <S.Input
            type="text"
            value={brand}
            onChange={(e) => setBrand(e.target.value)}
            placeholder="Название бренда"
            disabled={saving}
            required
          />
        </S.Field>

        <S.Field>
          Страна
          <S.Input
            type="text"
            value={country}
            onChange={(e) => setCountry(e.target.value)}
            placeholder="Россия"
            disabled={saving}
            required
          />
        </S.Field>

        <S.Field>
          Город
          <S.Input
            type="text"
            value={town}
            onChange={(e) => setTown(e.target.value)}
            placeholder="Москва"
            disabled={saving}
          />
        </S.Field>

        <S.Field>
          Цель эксперимента
          <S.TextArea
            rows={3}
            value={experimentGoal}
            onChange={(e) => setExperimentGoal(e.target.value)}
            placeholder="Какую гипотезу или результат нужно проверить"
            disabled={saving}
            required
          />
        </S.Field>

        <S.Field>
          Происхождение цели
          <S.Select value={goalOrigin} onChange={(e) => setGoalOrigin(e.target.value as 'demo_assumption' | 'user_provided')} disabled={saving}>
            <option value="user_provided">Цель пользователя</option>
            <option value="demo_assumption">Демонстрационное допущение</option>
          </S.Select>
        </S.Field>

        <S.Field>
          Целевое действие
          <S.TextArea
            rows={2}
            value={targetAction}
            onChange={(e) => setTargetAction(e.target.value)}
            placeholder="Какое действие должен совершить пользователь"
            disabled={saving}
            required
          />
        </S.Field>

        <S.Field>
          Основная метрика
          <S.Input type="text" value={primaryMetric} onChange={(e) => setPrimaryMetric(e.target.value)} placeholder="application_submit_rate" disabled={saving} required />
        </S.Field>

        <S.Field>
          Направление метрики
          <S.Select value={metricDirection} onChange={(e) => setMetricDirection(e.target.value as 'increase' | 'decrease')} disabled={saving}>
            <option value="increase">Увеличить</option>
            <option value="decrease">Уменьшить</option>
          </S.Select>
        </S.Field>

        <S.Field>
          Числитель метрики
          <S.TextArea rows={2} value={metricNumerator} onChange={(e) => setMetricNumerator(e.target.value)} placeholder="Число уникальных пользователей, совершивших действие" disabled={saving} required />
        </S.Field>

        <S.Field>
          Знаменатель метрики
          <S.TextArea rows={2} value={metricDenominator} onChange={(e) => setMetricDenominator(e.target.value)} placeholder="Число уникальных пользователей, включенных в эксперимент" disabled={saving} required />
        </S.Field>

        <S.Field>
          Критерий успеха (необязательно)
          <S.TextArea rows={2} value={successCriterion} onChange={(e) => setSuccessCriterion(e.target.value)} placeholder="Определите после baseline и расчета выборки" disabled={saving} />
        </S.Field>

        <S.Field>
          Разрешенные каналы (каждый с новой строки)
          <S.TextArea
            rows={3}
            value={allowedChannels}
            onChange={(e) => setAllowedChannels(e.target.value)}
            placeholder={'in_app_banner\npush\nemail'}
            disabled={saving}
          />
        </S.Field>

        <S.Field>
          Запрещенные формулировки (каждая с новой строки)
          <S.TextArea
            rows={3}
            value={forbiddenClaims}
            onChange={(e) => setForbiddenClaims(e.target.value)}
            placeholder="гарантированное одобрение"
            disabled={saving}
          />
        </S.Field>

        <S.Field>
          Известные ограничения (каждое с новой строки)
          <S.TextArea
            rows={3}
            value={knownConstraints}
            onChange={(e) => setKnownConstraints(e.target.value)}
            placeholder="Условия или ограничения продукта"
            disabled={saving}
          />
        </S.Field>

        <S.Field>
          Неизвестные условия и ограничения (каждое с новой строки)
          <S.TextArea rows={3} value={unknowns} onChange={(e) => setUnknowns(e.target.value)} placeholder="Тарифы требуют проверки" disabled={saving} />
        </S.Field>

        <S.Field>
          Категория
          <S.Select
            value={category}
            onChange={(e) => setCategory(e.target.value as ProductCategory)}
            disabled={saving}
            required
          >
            <option value="" disabled>
              Выберите категорию
            </option>
            {PRODUCT_CATEGORIES.map((c) => (
              <option key={c} value={c}>
                {getCategoryLabel(c)}
              </option>
            ))}
          </S.Select>
        </S.Field>

        {category === 'other' && (
          <S.Field>
            Своя категория
            <S.Input
              type="text"
              value={customCategory}
              onChange={(e) => setCustomCategory(e.target.value)}
              placeholder="Например, страхование"
              disabled={saving}
              required
              maxLength={64}
            />
          </S.Field>
        )}

        <S.Field>
          Ключевые слова (через запятую)
          <S.Input
            type="text"
            value={keywords}
            onChange={(e) => setKeywords(e.target.value)}
            placeholder="ключевое слово 1, ключевое слово 2"
            disabled={saving}
          />
        </S.Field>

        <S.Field>
          Статус стоимости
          <S.Select value={priceStatus} onChange={(e) => setPriceStatus(e.target.value as ProductPrice['status'])} disabled={saving}>
            <option value="unknown">Неизвестна</option>
            <option value="known">Подтверждена</option>
          </S.Select>
        </S.Field>

        <S.Field>
          Описание стоимости
          <S.TextArea rows={2} value={priceDescription} onChange={(e) => setPriceDescription(e.target.value)} disabled={saving} required />
        </S.Field>

        <S.Field>
          Ссылка
          <S.Input
            type="text"
            value={link}
            onChange={(e) => setLink(e.target.value)}
            placeholder="https://example.com"
            disabled={saving}
          />
        </S.Field>

        {saveError && <S.ErrorMessage>{saveError}</S.ErrorMessage>}

        <S.ButtonGroup>
          <S.Button type="submit" $primary disabled={saving}>
            {saving ? 'Сохраняем…' : 'Сохранить'}
          </S.Button>
          {product && (
            <S.Button type="button" $danger onClick={handleDelete} disabled={deleting}>
              {deleting ? 'Удаляем…' : 'Удалить продукт'}
            </S.Button>
          )}
        </S.ButtonGroup>

        {deleteError && <S.ErrorMessage>{deleteError}</S.ErrorMessage>}
      </S.Form>

      <S.AudiencesSection>
        <S.SectionTitle>Анализ и гипотезы</S.SectionTitle>
        <S.Message>Используются сохранённые данные продукта и загруженные документы.</S.Message>
        {analysis && (
          <S.AudienceGenerationStatus>
            {getAnalysisStatusLabel(analysis.status)} · {getAnalysisStageLabel(analysis.current_stage)}
          </S.AudienceGenerationStatus>
        )}
        {analysisError && !isApiNotFound(analysisError) && (
          <S.ErrorMessage>{rtkErrorMessage(analysisError, 'Не удалось загрузить результаты анализа')}</S.ErrorMessage>
        )}
        {isRecord(analysis?.product_result) && (
          <S.AudienceDetails>
            <S.AudienceSummary>Паспорт продукта</S.AudienceSummary>
            <JsonTree value={analysis.product_result} />
          </S.AudienceDetails>
        )}
        {isRecord(analysis?.audience_result) && (
          <S.AudienceDetails>
            <S.AudienceSummary>Выбранная аудитория и ограничения</S.AudienceSummary>
            <JsonTree value={analysis.audience_result} />
          </S.AudienceDetails>
        )}
        {hypotheses.length > 0 && (
          <S.Message>Это черновики для проверки человеком; данные аудиторий синтетические.</S.Message>
        )}
        {hypotheses.length > 0 && (
          <S.AudienceList>
            {hypotheses.map((hypothesis, index) => (
              <S.AudienceItem key={typeof hypothesis.candidate_id === 'string' ? hypothesis.candidate_id : index}>
                <S.AudienceDetails>
                  <S.AudienceSummary>
                    <S.AudienceInfo>
                      <S.AudienceDescription>
                        {typeof hypothesis.title === 'string' ? hypothesis.title : `Гипотеза ${index + 1}`}
                      </S.AudienceDescription>
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
              </S.AudienceItem>
            ))}
          </S.AudienceList>
        )}

        <S.Button
          type="button"
          $success
          onClick={handleStartAnalysis}
          disabled={!product || startingAnalysis || analysis?.status === 'queued' || analysis?.status === 'running'}
        >
          {startingAnalysis ? 'Запускаем…' : 'Проанализировать и создать гипотезы'}
        </S.Button>
        {analysisStartError && <S.ErrorMessage>{analysisStartError}</S.ErrorMessage>}

      </S.AudiencesSection>

      <S.FilesSection>
        <S.SectionTitle>Файлы</S.SectionTitle>

        {filesLoading && <S.Message>Загрузка…</S.Message>}

        {!filesLoading && filesList.length === 0 && (
          <S.Message>Пока нет загруженных файлов</S.Message>
        )}

        {!filesLoading && filesList.length > 0 && (
          <S.FileList>
            {filesList.map((file) => (
              <S.FileItem key={file.id}>
                <S.FileInfo>
                  <S.FileName>{file.filename}</S.FileName>
                  <S.FileMeta>
                    {formatFileSize(file.size)} · {formatDate(file.created_at)}
                  </S.FileMeta>
                </S.FileInfo>
                <S.TypeBadge $type={file.type}>{getFileTypeLabel(file.type)}</S.TypeBadge>
                <S.Button
                  type="button"
                  $danger
                  onClick={() => handleDeleteFile(file.id)}
                  disabled={deletingFileId === file.id}
                >
                  {deletingFileId === file.id ? 'Удаляем…' : 'Удалить'}
                </S.Button>
              </S.FileItem>
            ))}
          </S.FileList>
        )}

        {fileDeleteError && <S.ErrorMessage>{fileDeleteError}</S.ErrorMessage>}

        <S.UploadForm onSubmit={handleUpload}>
          <S.Field>
            Файл (.pdf, .doc, .docx, .txt, .md, макс. 50 МБ)
            <S.FileInput
              type="file"
              accept=".pdf,.doc,.docx,.txt,.md"
              ref={fileInputRef}
              onChange={(e) => setSelectedFile(e.target.files?.[0] ?? null)}
              disabled={uploading}
            />
          </S.Field>

          <S.Field>
            Тип
            <S.Select
              value={fileType}
              onChange={(e) => setFileType(e.target.value as ProductFileType)}
              disabled={uploading}
            >
              <option value="other">{getFileTypeLabel('other')}</option>
              <option value="brandbook" disabled={hasBrandbook}>
                {getFileTypeLabel('brandbook')}
              </option>
            </S.Select>
          </S.Field>

          {uploadError && <S.ErrorMessage>{uploadError}</S.ErrorMessage>}

          <S.Button type="submit" $primary disabled={uploading || !selectedFile}>
            {uploading ? 'Загружаем…' : 'Загрузить файл'}
          </S.Button>
        </S.UploadForm>
      </S.FilesSection>
    </S.Page>
  )
}

function getAnalysisStatusLabel(status: string): string {
  if (status === 'queued') {
    return 'в очереди'
  }
  if (status === 'running') {
    return 'в работе'
  }
  if (status === 'completed') {
    return 'завершена'
  }
  if (status === 'failed') {
    return 'не удалось завершить'
  }
  if (status === 'blocked') {
    return 'остановлен: недостаточно данных'
  }
  return status
}

function getAnalysisStageLabel(stage: string): string {
  if (stage === 'product') return 'анализ продукта'
  if (stage === 'audience') return 'подбор аудитории'
  if (stage === 'hypotheses') return 'подготовка гипотез'
  return stage
}

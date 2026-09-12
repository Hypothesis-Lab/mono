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
  useGetHypothesisProductQuery,
  useListProductAudiencesQuery,
  useListProductFilesQuery,
  useUpdateProductMutation,
  useUploadProductFileMutation,
} from '../../store/generatedApi'
import type { EligibilityRule, ProductFact, UpdateProductRequest } from '../../store/generatedApi'
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

function factsToText(facts: ProductFact[]): string {
  return facts.map((fact) => `${fact.fact_id}: ${fact.text}`).join('\n')
}

function factsFromText(value: string): ProductFact[] {
  return listFromText(value).map((line) => {
    const separator = line.indexOf(':')
    const factId = line.slice(0, separator).trim()
    const text = line.slice(separator + 1).trim()
    if (separator <= 0 || factId === '' || text === '') {
      throw new Error('Укажите каждый факт в формате F1: текст факта')
    }
    return {
      fact_id: factId,
      text,
    }
  })
}

function rulesToText(rules: EligibilityRule[]): string {
  return JSON.stringify(rules, null, 2)
}

function rulesFromText(value: string): EligibilityRule[] {
  const parsed: unknown = JSON.parse(value)
  if (!Array.isArray(parsed)) {
    throw new Error('Правила соответствия должны быть JSON-массивом')
  }
  return parsed as EligibilityRule[]
}

function areListsEqual(left: string[], right: string[]): boolean {
  return left.length === right.length && left.every((item, index) => item === right[index])
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
    data: audiences,
    isLoading: audiencesLoading,
  } = useListProductAudiencesQuery(
    product ? { id: product.id } : skipToken,
    { pollingInterval: product ? 3000 : 0 },
  )

  const [createProduct, { isLoading: creating }] = useCreateProductMutation()
  const [updateProduct, { isLoading: updating }] = useUpdateProductMutation()
  const [deleteProduct, { isLoading: deleting }] = useDeleteProductMutation()
  const [uploadProductFile, { isLoading: uploading }] = useUploadProductFileMutation()
  const [deleteProductFile] = useDeleteProductFileMutation()

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState<ProductCategory | ''>('')
  const [brand, setBrand] = useState('')
  const [geography, setGeography] = useState('')
  const [experimentGoal, setExperimentGoal] = useState('')
  const [targetAction, setTargetAction] = useState('')
  const [facts, setFacts] = useState('')
  const [eligibilityRules, setEligibilityRules] = useState('[]')
  const [allowedChannels, setAllowedChannels] = useState('')
  const [forbiddenClaims, setForbiddenClaims] = useState('')
  const [knownConstraints, setKnownConstraints] = useState('')
  const [keywords, setKeywords] = useState('')
  const [price, setPrice] = useState('')
  const [link, setLink] = useState('')

  const [saveError, setSaveError] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [fileType, setFileType] = useState<ProductFileType>('other')
  const fileInputRef = useRef<HTMLInputElement>(null)

  const [uploadError, setUploadError] = useState<string | null>(null)
  const [fileDeleteError, setFileDeleteError] = useState<string | null>(null)
  const [deletingFileId, setDeletingFileId] = useState<string | null>(null)


  const hasBrandbook = (files ?? []).some((file) => file.type === 'brandbook')
  const noProduct = !product && isApiNotFound(productError)

  useEffect(() => {
    if (product) {
      setName(product.name)
      setDescription(product.description)
      setCategory(product.category)
      setBrand(product.brand)
      setGeography(product.geography)
      setExperimentGoal(product.experiment_goal)
      setTargetAction(product.target_action)
      setFacts(factsToText(product.facts))
      setEligibilityRules(rulesToText(product.eligibility_rules))
      setAllowedChannels(listToText(product.allowed_channels))
      setForbiddenClaims(listToText(product.forbidden_claims))
      setKnownConstraints(listToText(product.known_constraints))
      setKeywords(product.keywords.join(', '))
      setPrice(product.price ?? '')
      setLink(product.link ?? '')
    } else if (noProduct) {
      setName('')
      setDescription('')
      setCategory('')
      setBrand('')
      setGeography('')
      setExperimentGoal('')
      setTargetAction('')
      setFacts('')
      setEligibilityRules('[]')
      setAllowedChannels('')
      setForbiddenClaims('')
      setKnownConstraints('')
      setKeywords('')
      setPrice('')
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
    const trimmedBrand = brand.trim()
    const trimmedGeography = geography.trim()
    const trimmedExperimentGoal = experimentGoal.trim()
    const trimmedTargetAction = targetAction.trim()
    let parsedFacts: ProductFact[]
    let parsedEligibilityRules: EligibilityRule[]
    let parsedAllowedChannels: string[]
    let parsedForbiddenClaims: string[]
    let parsedKnownConstraints: string[]
    try {
      parsedFacts = factsFromText(facts)
      parsedEligibilityRules = rulesFromText(eligibilityRules)
      parsedAllowedChannels = listFromText(allowedChannels)
      parsedForbiddenClaims = listFromText(forbiddenClaims)
      parsedKnownConstraints = listFromText(knownConstraints)
    } catch (err) {
      setSaveError(rtkErrorMessage(err, 'Проверьте данные продукта'))
      return
    }
    const trimmedKeywords = keywords
      .split(',')
      .map((k) => k.trim())
      .filter((k) => k !== '')
    const trimmedPrice = price.trim()
    const trimmedLink = link.trim()

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
        if (trimmedBrand !== product.brand) {
          body.brand = trimmedBrand
        }
        if (trimmedGeography !== product.geography) {
          body.geography = trimmedGeography
        }
        if (trimmedExperimentGoal !== product.experiment_goal) {
          body.experiment_goal = trimmedExperimentGoal
        }
        if (trimmedTargetAction !== product.target_action) {
          body.target_action = trimmedTargetAction
        }
        if (JSON.stringify(parsedFacts) !== JSON.stringify(product.facts)) {
          body.facts = parsedFacts
        }
        if (JSON.stringify(parsedEligibilityRules) !== JSON.stringify(product.eligibility_rules)) {
          body.eligibility_rules = parsedEligibilityRules
        }
        if (!areListsEqual(parsedAllowedChannels, product.allowed_channels)) {
          body.allowed_channels = parsedAllowedChannels
        }
        if (!areListsEqual(parsedForbiddenClaims, product.forbidden_claims)) {
          body.forbidden_claims = parsedForbiddenClaims
        }
        if (!areListsEqual(parsedKnownConstraints, product.known_constraints)) {
          body.known_constraints = parsedKnownConstraints
        }
        if (!areListsEqual(trimmedKeywords, product.keywords)) {
          body.keywords = trimmedKeywords
        }
        if (trimmedPrice !== (product.price ?? '')) {
          body.price = trimmedPrice
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
            brand: trimmedBrand,
            geography: trimmedGeography,
            experiment_goal: trimmedExperimentGoal,
            target_action: trimmedTargetAction,
            facts: parsedFacts,
            eligibility_rules: parsedEligibilityRules,
            allowed_channels: parsedAllowedChannels,
            forbidden_claims: parsedForbiddenClaims,
            known_constraints: parsedKnownConstraints,
            keywords: trimmedKeywords,
            price: trimmedPrice === '' ? null : trimmedPrice,
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

  const saving = creating || updating
  const filesList = files ?? []

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
          География
          <S.Input
            type="text"
            value={geography}
            onChange={(e) => setGeography(e.target.value)}
            placeholder="Россия"
            disabled={saving}
            required
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
          Подтвержденные факты (каждый с новой строки: F1: текст)
          <S.TextArea
            rows={6}
            value={facts}
            onChange={(e) => setFacts(e.target.value)}
            placeholder={'F1: Первый подтвержденный факт\nF2: Второй подтвержденный факт\nF3: Третий подтвержденный факт'}
            disabled={saving}
            required
          />
        </S.Field>

        <S.Field>
          Разрешенные каналы (каждый с новой строки)
          <S.TextArea
            rows={3}
            value={allowedChannels}
            onChange={(e) => setAllowedChannels(e.target.value)}
            placeholder={'in_app_banner\npush\nemail'}
            disabled={saving}
            required
          />
        </S.Field>

        <S.Field>
          Правила соответствия (JSON-массив, необязательно)
          <S.TextArea
            rows={7}
            value={eligibilityRules}
            onChange={(e) => setEligibilityRules(e.target.value)}
            placeholder={'[\n  {\n    "rule_id": "E1",\n    "field": "age",\n    "operator": "between",\n    "value": [21, 70],\n    "description": "Возраст от 21 до 70 лет"\n  }\n]'}
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
          Цена
          <S.Input
            type="text"
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            placeholder="$9.99"
            disabled={saving}
          />
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
        <S.SectionTitle>Аудитории</S.SectionTitle>

        {audiencesLoading && <S.Message>Загрузка…</S.Message>}

        {!audiencesLoading && (audiences ?? []).length === 0 && (
          <S.Message>Пока нет аудиторий</S.Message>
        )}

        {!audiencesLoading && (audiences ?? []).length > 0 && (
          <S.AudienceList>
            {audiences!.map((audience) => (
              <S.AudienceItem key={audience.id}>
                <S.AudienceDetails open>
                  <S.AudienceSummary>
                    <S.AudienceInfo>
                      <S.AudienceDescription>{audience.persona.label}</S.AudienceDescription>
                      <S.AudienceMeta>
                        Ранг {audience.rank} · Оценка {audience.score}
                      </S.AudienceMeta>
                    </S.AudienceInfo>
                  </S.AudienceSummary>
                  <S.AudienceData>{JSON.stringify(audience, null, 2)}</S.AudienceData>
                </S.AudienceDetails>
              </S.AudienceItem>
            ))}
          </S.AudienceList>
        )}

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
            Файл (.pdf, .docx, макс. 50 МБ)
            <S.FileInput
              type="file"
              accept=".pdf,.docx"
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

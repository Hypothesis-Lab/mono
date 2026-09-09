import { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { skipToken } from '@reduxjs/toolkit/query'
import type { ProductCategory, ProductFileType } from '../../api/structs'
import {
  PRODUCT_CATEGORIES,
  getCategoryLabel,
  getFileTypeLabel,
  getGenderLabel,
  isApiNotFound,
  rtkErrorMessage,
} from '../../api/structs'
import {
  useCreateProductMutation,
  useDeleteAudienceMutation,
  useDeleteProductFileMutation,
  useDeleteProductMutation,
  useGetHypothesisProductQuery,
  useListProductAudiencesQuery,
  useListProductFilesQuery,
  useUpdateProductMutation,
  useUploadProductFileMutation,
} from '../../store/generatedApi'
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
  } = useListProductAudiencesQuery(product ? { id: product.id } : skipToken)

  const [createProduct, { isLoading: creating }] = useCreateProductMutation()
  const [updateProduct, { isLoading: updating }] = useUpdateProductMutation()
  const [deleteProduct, { isLoading: deleting }] = useDeleteProductMutation()
  const [uploadProductFile, { isLoading: uploading }] = useUploadProductFileMutation()
  const [deleteProductFile] = useDeleteProductFileMutation()
  const [deleteAudience] = useDeleteAudienceMutation()

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [category, setCategory] = useState<ProductCategory | ''>('')
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

  const [deletingAudienceId, setDeletingAudienceId] = useState<string | null>(null)
  const [audienceDeleteError, setAudienceDeleteError] = useState<string | null>(null)

  const hasBrandbook = (files ?? []).some((file) => file.type === 'brandbook')
  const noProduct = !product && isApiNotFound(productError)

  useEffect(() => {
    if (product) {
      setName(product.name)
      setDescription(product.description ?? '')
      setCategory(product.category)
      setKeywords(product.keywords.join(', '))
      setPrice(product.price ?? '')
      setLink(product.link ?? '')
    } else if (noProduct) {
      setName('')
      setDescription('')
      setCategory('')
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
    const trimmedKeywords = keywords
      .split(',')
      .map((k) => k.trim())
      .filter((k) => k !== '')
    const trimmedPrice = price.trim()
    const trimmedLink = link.trim()

    try {
      if (product) {
        const body: {
          name?: string
          description?: string
          category?: ProductCategory
          keywords?: string[]
          price?: string
          link?: string
        } = {}

        if (trimmedName !== product.name) {
          body.name = trimmedName
        }
        if (trimmedDescription !== (product.description ?? '')) {
          body.description = trimmedDescription
        }
        if (category !== product.category) {
          body.category = category as ProductCategory
        }
        const keywordsChanged =
          trimmedKeywords.length !== product.keywords.length ||
          trimmedKeywords.some((k, i) => k !== product.keywords[i])
        if (keywordsChanged) {
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
            description: trimmedDescription === '' ? null : trimmedDescription,
            category: category as ProductCategory,
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

  async function handleDeleteAudience(audienceId: string) {
    setDeletingAudienceId(audienceId)
    setAudienceDeleteError(null)

    try {
      await deleteAudience({ id: audienceId }).unwrap()
    } catch (err) {
      setAudienceDeleteError(rtkErrorMessage(err, 'Не удалось удалить'))
    } finally {
      setDeletingAudienceId(null)
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
          />
        </S.Field>

        <S.Field>
          Категория
          <S.Select
            value={category}
            onChange={(e) => setCategory(e.target.value as ProductCategory)}
            disabled={saving}
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
                <S.AudienceInfo>
                  <S.AudienceDescription>{audience.description}</S.AudienceDescription>
                  <S.AudienceMeta>
                    {getGenderLabel(audience.gender ?? 'all')}
                    {audience.age_from != null && audience.age_to != null
                      ? ` · ${audience.age_from}-${audience.age_to} лет`
                      : null}
                  </S.AudienceMeta>
                </S.AudienceInfo>
                <S.Button
                  type="button"
                  onClick={() =>
                    navigate(
                      `/hypothesis/${hid}/product/audiences/${audience.id}`,
                    )
                  }
                >
                  Редактировать
                </S.Button>
                <S.Button
                  type="button"
                  $danger
                  onClick={() => handleDeleteAudience(audience.id)}
                  disabled={deletingAudienceId === audience.id}
                >
                  {deletingAudienceId === audience.id ? 'Удаляем…' : 'Удалить'}
                </S.Button>
              </S.AudienceItem>
            ))}
          </S.AudienceList>
        )}

        {audienceDeleteError && (
          <S.ErrorMessage>{audienceDeleteError}</S.ErrorMessage>
        )}

        {product && (
          <S.Button
            type="button"
            $primary
            onClick={() => navigate(`/hypothesis/${hid}/product/audiences`)}
          >
            Добавить аудиторию
          </S.Button>
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
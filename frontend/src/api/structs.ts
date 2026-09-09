import type { FetchBaseQueryError } from '@reduxjs/toolkit/query'
import type { ErrBody } from '../store/generatedApi'

export type AuthTokenResponse = { token: string }
export type LoginRequest = { email: string; password: string }
export type RegisterRequest = { email: string; password: string }

export type ApiError = {
  status: number
  message: string
}

function errorMessage(err: unknown, fallback: string): string {
  const apiError = err as ApiError | undefined
  if (apiError !== undefined && typeof apiError.message === 'string' && apiError.message.length > 0) {
    return apiError.message
  }
  if (err instanceof Error && err.message.length > 0) {
    return err.message
  }
  return fallback
}

export function rtkErrorMessage(err: unknown, fallback: string): string {
  if (err !== null && typeof err === 'object' && 'data' in err) {
    const body = (err as FetchBaseQueryError).data as ErrBody | undefined
    const description = body?.error?.description
    if (typeof description === 'string' && description.length > 0) {
      return description
    }
  }
  return errorMessage(err, fallback)
}

export function isApiNotFound(err: unknown): boolean {
  if (err === null || typeof err !== 'object' || !('status' in err)) {
    return false
  }
  const error = err as FetchBaseQueryError
  if (error.status === 404) {
    return true
  }
  return (
    error.status === 'PARSING_ERROR' && error.originalStatus === 404
  )
}

export type HypothesisStatus = 'idle' | 'in_progress'

export type Hypothesis = {
  id: string
  description: string | null
  status: HypothesisStatus
}

export type ProductCategory =
  | 'electronics'
  | 'clothing'
  | 'food'
  | 'furniture'
  | 'beauty'
  | 'sports'
  | 'toys'
  | 'books'
  | 'automotive'
  | 'health'
  | 'home'
  | 'office'
  | 'garden'
  | 'pets'
  | 'music'
  | 'other'

export const PRODUCT_CATEGORIES: ProductCategory[] = [
  'electronics',
  'clothing',
  'food',
  'furniture',
  'beauty',
  'sports',
  'toys',
  'books',
  'automotive',
  'health',
  'home',
  'office',
  'garden',
  'pets',
  'music',
  'other',
]

export type Product = {
  id: string
  name: string
  description: string | null
  category: ProductCategory
  keywords: string[]
  price: string | null
  link: string | null
  hypothesis_id: string
}

export type AudienceGender = 'men' | 'women' | 'all'

export const AUDIENCE_GENDERS: AudienceGender[] = ['men', 'women', 'all']

export type Audience = {
  id: string
  product_id: string
  description: string
  age_from: number | null
  age_to: number | null
  gender: AudienceGender | null
  interests: string | null
  geography: string | null
  pain_points: string | null
  desires_motivation: string | null
  created_at: string
}

export type ProductFileType = 'other' | 'brandbook'

export type ProductFile = {
  id: string
  hypothesis_id: string
  type: ProductFileType
  filename: string
  content_type: string
  size: number
  created_at: string
}

const statusLabels: Record<HypothesisStatus, string> = {
  idle: 'В ожидании',
  in_progress: 'В процессе',
}

export function getStatusLabel(status: string): string {
  return statusLabels[status as HypothesisStatus] ?? status
}

const categoryLabels: Record<ProductCategory, string> = {
  electronics: 'Электроника',
  clothing: 'Одежда',
  food: 'Еда',
  furniture: 'Мебель',
  beauty: 'Красота',
  sports: 'Спорт',
  toys: 'Игрушки',
  books: 'Книги',
  automotive: 'Авто',
  health: 'Здоровье',
  home: 'Дом',
  office: 'Офис',
  garden: 'Сад',
  pets: 'Питомцы',
  music: 'Музыка',
  other: 'Другое',
}

export function getCategoryLabel(category: string): string {
  return categoryLabels[category as ProductCategory] ?? category
}

const genderLabels: Record<AudienceGender, string> = {
  men: 'Мужчины',
  women: 'Женщины',
  all: 'Все',
}

export function getGenderLabel(gender: string): string {
  return genderLabels[gender as AudienceGender] ?? gender
}

const fileTypeLabels: Record<ProductFileType, string> = {
  other: 'другое',
  brandbook: 'брендбук',
}

export function getFileTypeLabel(type: string): string {
  return fileTypeLabels[type as ProductFileType] ?? type
}

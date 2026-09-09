const TOKEN_KEY = 'auth-token'

export const readAuthToken = (): string | null =>
  typeof localStorage !== 'undefined' ? localStorage.getItem(TOKEN_KEY) : null

export const storeAuthToken = (token: string): void => {
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(TOKEN_KEY, token)
  }
}

export const clearAuthToken = (): void => {
  if (typeof localStorage !== 'undefined') {
    localStorage.removeItem(TOKEN_KEY)
  }
}
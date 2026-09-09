import { createSlice, type PayloadAction } from '@reduxjs/toolkit'

import { readAuthToken } from '../authToken'
import type { RootState } from '../store'

type AuthState = {
  token: string | null
}

const initialState: AuthState = {
  token: readAuthToken(),
}

const authSlice = createSlice({
  name: 'auth',
  initialState,
  reducers: {
    setToken(state, action: PayloadAction<string>) {
      state.token = action.payload
    },
    clearToken(state) {
      state.token = null
    },
  },
})

export const { setToken, clearToken } = authSlice.actions

export const selectAuthToken = (state: RootState): string | null =>
  state.auth.token

export const selectIsAuthenticated = (state: RootState): boolean =>
  state.auth.token !== null

export default authSlice.reducer
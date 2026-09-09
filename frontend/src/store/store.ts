import {
  configureStore,
  createListenerMiddleware,
} from '@reduxjs/toolkit'
import type { UnknownAction } from 'redux'
import type { ApiError } from '../api/structs'
import authReducer from './slices/authSlice'
import { emptySplitApi } from './emptyApi'
import { api } from './generatedApi'
import './apiTags'
import { clearAuthToken } from './authToken'
import { clearToken } from './slices/authSlice'

const UNAUTHENTICATED_ENDPOINTS = new Set(['loginUser', 'registerUser'])

type RtkQueryMeta = {
  arg?: { endpointName?: string }
}

const listenerMiddleware = createListenerMiddleware()

listenerMiddleware.startListening({
  predicate: (action: UnknownAction) => action.type.endsWith('/rejected'),
  effect: (action, listenerApi) => {
    const payload = action.payload as ApiError | undefined
    const state = listenerApi.getState() as RootState

    if (payload?.status !== 401 || state.auth.token === null) {
      return
    }

    const meta = (action.meta as RtkQueryMeta | undefined) ?? undefined
    const endpointName = meta?.arg?.endpointName
    if (endpointName !== undefined && UNAUTHENTICATED_ENDPOINTS.has(endpointName)) {
      return
    }

    clearAuthToken()
    listenerApi.dispatch(clearToken())
    listenerApi.dispatch(api.util.resetApiState())
  },
})

export function makeStore() {
  return configureStore({
    reducer: {
      auth: authReducer,
      [emptySplitApi.reducerPath]: emptySplitApi.reducer,
    },
    middleware: (getDefaultMiddleware) =>
      getDefaultMiddleware().prepend(listenerMiddleware.middleware).concat(emptySplitApi.middleware),
  })
}

export const store = makeStore()

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react'

import { apiConfig } from '../config/api.config'
import type { RootState } from './store'

export const emptySplitApi = createApi({
  reducerPath: 'api',
  baseQuery: fetchBaseQuery({
    baseUrl: apiConfig.baseUrl,
    prepareHeaders: (headers, { getState }) => {
      const token = (getState() as RootState).auth?.token
      if (token) {
        headers.set('Authorization', `Bearer ${token}`)
      }
      return headers
    },
  }),
  tagTypes: ['auth', 'hypothesis', 'product', 'file'],
  refetchOnMountOrArgChange: true,
  refetchOnReconnect: true,
  endpoints: () => ({}),
})

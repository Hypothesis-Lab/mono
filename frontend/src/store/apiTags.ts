import { api } from './generatedApi'

api.enhanceEndpoints({
  endpoints: {
    listHypotheses: {
      providesTags: (result) => [
        ...(result ?? []).map(({ id }) => ({ type: 'hypothesis' as const, id })),
        { type: 'hypothesis', id: 'LIST' },
      ],
    },
    getHypothesis: {
      providesTags: (_result, _error, { id }) => [{ type: 'hypothesis', id }],
    },
    createHypothesis: {
      invalidatesTags: [{ type: 'hypothesis', id: 'LIST' }],
    },
    updateHypothesis: {
      invalidatesTags: (_result, _error, { id }) => [{ type: 'hypothesis', id }, { type: 'hypothesis', id: 'LIST' }],
    },
    deleteHypothesis: {
      invalidatesTags: (_result, _error, { id }) => [{ type: 'hypothesis', id }, { type: 'hypothesis', id: 'LIST' }],
    },
    launchHypothesis: {
      invalidatesTags: (_result, _error, { id }) => [{ type: 'hypothesis', id }],
    },
    stopHypothesis: {
      invalidatesTags: (_result, _error, { id }) => [{ type: 'hypothesis', id }],
    },
    getHypothesisProduct: {
      providesTags: (result, _error, { id }) => [
        { type: 'product', id: `Hypothesis-${id}` },
        { type: 'product', id: 'HYPOTHESIS' },
        ...(result ? [{ type: 'product' as const, id: result.id }] : []),
      ],
    },
    createProduct: {
      invalidatesTags: (_result, _error, { createProductRequest }) => [
        { type: 'product', id: `Hypothesis-${createProductRequest.hypothesis_id}` },
      ],
    },
    updateProduct: {
      invalidatesTags: (_result, _error, { id }) => [
        { type: 'product', id },
        { type: 'product', id: 'HYPOTHESIS' },
      ],
    },
    deleteProduct: {
      invalidatesTags: (_result, _error, { id }) => [
        { type: 'product', id },
        { type: 'product', id: 'HYPOTHESIS' },
      ],
    },
    listProductAudiences: {
      providesTags: (result, _error, { id }) => [
        ...(result ?? []).map(({ id: audienceId }) => ({ type: 'audience' as const, id: audienceId })),
        { type: 'audience', id: `Product-${id}` },
      ],
    },
    createProductAudience: {
      invalidatesTags: (_result, _error, { id }) => [{ type: 'audience', id: `Product-${id}` }],
    },
    getAudience: {
      providesTags: (_result, _error, { id }) => [{ type: 'audience', id }],
    },
    updateAudience: {
      invalidatesTags: (_result, _error, { id }) => [{ type: 'audience', id }],
    },
    deleteAudience: {
      invalidatesTags: (_result, _error, { id }) => [{ type: 'audience', id }],
    },
    listProductFiles: {
      providesTags: (result, _error, { id }) => [
        { type: 'file', id: `Hypothesis-${id}` },
        ...(result ?? []).map((file) => ({ type: 'file' as const, id: file.id })),
      ],
    },
    uploadProductFile: {
      query: ({ id, body }) => {
        const formData = new FormData()
        formData.append('file', body.file)
        formData.append('type', body.type)
        return {
          url: `/api/v1/hypothesis/${id}/files`,
          method: 'POST',
          body: formData,
        }
      },
      invalidatesTags: (_result, _error, { id }) => [{ type: 'file', id: `Hypothesis-${id}` }],
    },
    deleteProductFile: {
      invalidatesTags: (_result, _error, { id }) => [{ type: 'file', id }],
    },
  },
})

export { api as enhancedApi }
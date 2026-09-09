# Redux Review — `/app/frontend/src/store`

## TL;DR

The Redux layer is **over-used for page-local data**. The main concrete problems are:

1. **Loading/error state is broken or unused** for many operations.
2. **Global per-slice loading flags** collide when multiple pages/components use the same slice.
3. **Most CRUD state does not need Redux at all** — it is fetched, edited, and discarded on a single page.
4. There is a lot of **boilerplate duplication** and a few **anti-patterns** (token read inside every thunk, selectors not typed to `RootState`, non-memoized derived selectors).

Recommended direction: keep **only auth in Redux**, move page-level data fetching into page-local hooks/state (or a lightweight data-fetching library), and delete the rest of the slices/thunks.

---

## 1. Concrete Bugs

### 1.1 Single-resource fetches do not set `loading`/`error`

`hypothesesSlice` only handles pending/rejected for `fetchHypotheses` (the list). The single fetch `fetchHypothesis` has only a `.fulfilled` handler. As a result, on `HypothesisPage` and `EditHypothesisPage`:

- `selectHypothesesLoading` stays `false` while the hypothesis is loading.
- `selectHypothesesError` is never populated if `fetchHypothesis` fails.

Same issue in `audiencesSlice`: only `fetchAudiencesByProduct` handles pending/rejected; `fetchAudience` does not. On `EditAudiencePage` the audience-loading spinner never appears and the fetch error is lost.

### 1.2 Mutation loading/error state is effectively dead code

All slices define `loading` and `error`, but pages never read the Redux mutation state. Instead every page maintains its own local flags (`saving`, `deleting`, `uploading`, etc.). Examples:

- `productsSlice` sets `loading = true` on `createProduct` / `updateProduct` / `deleteProduct`, but `ProductPage` uses local `saving`/`deleting` states.
- `hypothesesSlice` does not even set loading on mutations, but `HypothesisPage`/`EditHypothesisPage` use local `launching`/`stopping`/`saving`/`deleting`.
- `productFilesSlice` sets loading only on `fetchProductFiles`; `ProductPage` uses local `uploading`/`deletingFileId`.

This makes the slice-level `loading`/`error` fields misleading: they are global, mutation-agnostic, and mostly ignored by the UI.

### 1.3 Global loading flags are shared across unrelated UI

A single `loading` boolean per slice is incorrect when one slice serves multiple screens. For example:

- `products.loading` is toggled by `fetchProductByHypothesis`, `createProduct`, `updateProduct`, and `deleteProduct`. If two hypothesis pages are open (unlikely today, but still a design flaw) or if a mutation runs while a list is visible, the same flag controls both.
- `audiences.loading` is shared between `fetchAudiencesByProduct` (in `ProductPage`) and would be shared with mutations if they set it.

The correct shape is **per-request** loading state, or at minimum per-operation loading state.

### 1.4 `selectProductFilesError` and `selectProductsError` are never used

The selectors exist but no component reads them. `ProductPage` shows local error state for upload/delete instead.

### 1.5 `EditAudiencePage` fetches audiences it never renders

```ts
useEffect(() => {
  if (product?.id && !id) {
    dispatch(fetchAudiencesByProduct(product.id))
  }
}, [dispatch, product?.id, id])
```

The result is not selected or displayed. The dispatch is a no-op from the UI perspective.

---

## 2. Architectural Concerns

### 2.1 Most slices are page-local, not global

| Slice | Truly global? | Used on pages |
|-------|---------------|---------------|
| `auth` | Yes (token, protected routes, header) | Header, ProtectedRoute, Login, Register |
| `hypotheses` | Partially (`fetchHypotheses` list is dashboard-only) | DashboardMain, Hypothesis, EditHypothesis, CreateHypothesis |
| `products` | No | Hypothesis, Product, EditHypothesis, EditAudience |
| `audiences` | No | Product, EditAudience |
| `productFiles` | No | Product |

The user is right: product, audience, file, and single-hypothesis state are fetched for and consumed by one or two adjacent pages. They do not need to live in a global store.

### 2.2 Thunks are tightly coupled to auth state

Every thunk does:

```ts
const token = getToken(getState() as RootState)
return await api.createProduct(token, body)
```

This is repetitive and leaks auth concerns into every feature. A better pattern is an API client/request interceptor that reads the token automatically, or at least a single helper that injects the token.

### 2.3 Selectors are not typed to `RootState`

Example:

```ts
export const selectProducts = (state: { products: ProductsState }) =>
  state.products.items
```

This bypasses the typed `RootState` and will silently break if the store shape changes. Prefer:

```ts
import type { RootState } from '../store'
export const selectProducts = (state: RootState) => state.products.items
```

### 2.4 Derived selectors are not memoized

```ts
export const selectAudiencesByProductId = (productId: string) => (state: {
  audiences: AudiencesState
}) =>
  state.audiences.items
    .filter((item) => item.product_id === productId)
    .sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime())
```

This returns a new array on every call, causing re-renders. Use `createSelector` from RTK.

### 2.5 State shape is inconsistent

- `productFiles` is keyed by `hypothesisId` (`Record<string, ProductFile[]>`), which is reasonable.
- `products`, `hypotheses`, and `audiences` are flat arrays, so every lookup is `O(n)` and merging logic is duplicated in every slice.

For global state, normalizing by ID is the Redux Toolkit recommendation.

### 2.6 Listener middleware logs out on *any* rejected thunk with 401

```ts
listenerMiddleware.startListening({
  predicate: (action: UnknownAction) => action.type.endsWith('/rejected'),
  ...
})
```

This is fine, but it is a broad net. If a non-auth endpoint ever legitimately returns 401 and the user should stay logged in (e.g. token expired mid-request handled by a refresh flow), this will force logout. Today there is no refresh, so it is acceptable; just worth noting.

---

## 3. Code-Level Issues

### 3.1 `ThunkApiConfig` is incomplete

```ts
export type ThunkApiConfig = {
  rejectValue: ThunkError
}
```

It should include `state: RootState` so thunks do not need `getState() as RootState`:

```ts
export type ThunkApiConfig = {
  state: RootState
  rejectValue: ThunkError
}
```

### 3.2 `logout` thunk has no error handling

```ts
export const logout = createAsyncThunk<void, void, ThunkApiConfig>(
  'auth/logout',
  async () => {
    localStorage.removeItem(TOKEN_KEY)
  },
)
```

`localStorage.removeItem` cannot fail in a useful way, but mixing sync localStorage access with async thunks is unnecessary. This could be a plain synchronous action or a helper called by the component.

### 3.3 `getToken` is exported from `authThunks.ts`

```ts
export const getToken = () => localStorage.getItem(TOKEN_KEY)
```

This is used by `authSlice.ts` for initial state and by nothing else. It belongs in an auth utility module, not a thunk file.

### 3.4 `productsSlice` mixes matcher-based and case-based handlers

The slice handles `fetchProductByHypothesis.fulfilled` with `addCase` but uses `isPending`/`isRejected`/`isFulfilled` matchers for loading. This is inconsistent with `hypothesesSlice` and `audiencesSlice`, which use `addCase` for pending/rejected on the list fetch only. Pick one style per slice.

### 3.5 `hypothesis.description` can be `null`

```ts
export type Hypothesis = {
  id: string
  description: string | null
  status: HypothesisStatus
}
```

But forms treat it as required (`if (description.trim() === '')`). The slice and thunk types should clarify whether `description` is nullable in creation/update.

### 3.6 `ProductPage` dependency array is noisy

```ts
useEffect(() => { ... }, [
  product,
  product?.id,
  product?.name,
  ...
])
```

Just `product` is sufficient. Listing every field is brittle.

---

## 4. Recommended Path Forward

### Option A — Minimal keep-Redux refactor (short term)

If you want to keep Redux:

1. Add missing `pending`/`rejected` handlers for single-resource fetches (`fetchHypothesis`, `fetchAudience`).
2. Replace global `loading`/`error` booleans with **per-operation** state, e.g.:

   ```ts
   type ProductsState = {
     items: Product[]
     fetchStatus: 'idle' | 'loading' | 'succeeded' | 'failed'
     fetchError: string | null
     mutationStatus: Record<productId, 'idle' | 'loading' | 'succeeded' | 'failed'>
   }
   ```

   Or use `createEntityAdapter` + per-call tracking.
3. Remove unused selectors (`selectProductsError`, `selectProductFilesError`) or start using them.
4. Type all selectors with `RootState`.
5. Move token retrieval out of thunks into the API client or an interceptor.
6. Memoize derived selectors with `createSelector`.
7. Delete the useless `fetchAudiencesByProduct` dispatch in `EditAudiencePage` create mode.

### Option B — Move page data out of Redux (recommended)

This aligns with your intuition and is much simpler.

1. **Keep `authSlice` only.** It is genuinely global.
2. **Replace data slices with page-level hooks**, for example:

   ```ts
   // pages/Product/useProduct.ts
   export function useProduct(hypothesisId: string | undefined) {
     const [product, setProduct] = useState<Product | null>(null)
     const [loading, setLoading] = useState(false)
     const [error, setError] = useState<string | null>(null)

     useEffect(() => {
       if (!hypothesisId) return
       setLoading(true)
       api.getProductByHypothesis(getAuthToken(), hypothesisId)
         .then(setProduct)
         .catch((e) => setError(errorMessage(e, '...')))
         .finally(() => setLoading(false))
     }, [hypothesisId])

     return { product, loading, error, refresh: ... }
   }
   ```

3. **Mutations become direct API calls** in event handlers, with local loading/error state (which the pages already maintain).
4. Delete `productsSlice`, `hypothesesSlice`, `audiencesSlice`, `productFilesSlice` and all associated thunks.
5. Keep the auth listener middleware or handle 401 inside the API client.

Benefits:
- Fixes the loading bugs by design (each page owns its own state).
- Removes ~70% of the Redux boilerplate.
- No shared global flags.
- Easier to test and reason about.

### Option C — Adopt RTK Query or TanStack Query

If the app will grow, RTK Query (already in the `@reduxjs/toolkit` package) or TanStack Query is the idiomatic solution. It handles caching, loading/error states per endpoint, automatic refetching, and deduplication.

This is more setup but pays off if the data model becomes more complex.

---

## 5. Suggested Immediate Action List

1. Decide between Option A and Option B.
2. If Option B (recommended):
   - Create a small auth utility (`src/auth/token.ts`) and move `getToken` / `logout` there.
   - Simplify `authSlice` to use a synchronous `setToken`/`clearToken` reducer if desired.
   - Write page hooks for Product, Hypothesis, Audience, and Files.
   - Remove the five non-auth slices and their thunks.
   - Update `store.ts` to only contain `auth`.
   - Run `npm run build` and `npm run lint`.

---

## 6. Verification Commands

- `npm run build` — currently passes.
- `npm run lint` — currently passes.

Neither tool catches the logical loading-state bugs; those must be fixed by hand.

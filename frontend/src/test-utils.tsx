import { type RenderOptions, render } from '@testing-library/react'
import { Provider } from 'react-redux'
import { MemoryRouter } from 'react-router-dom'
import type { ReactElement, ReactNode } from 'react'
import { makeStore } from './store/store'

export { server } from './test/server'

export function renderWithProviders(
  ui: ReactElement,
  options?: RenderOptions,
) {
  const store = makeStore()

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <Provider store={store}>
        <MemoryRouter>{children}</MemoryRouter>
      </Provider>
    )
  }

  return { store, ...render(ui, { wrapper: Wrapper, ...options }) }
}

export * from '@testing-library/react'
import { describe, expect, test } from 'vitest'
import { http, HttpResponse } from 'msw'
import { renderWithProviders, screen, server } from '../../test-utils'
import { DashboardMainPage } from './page'

const HYPOTHESES_URL = '*/api/v1/hypothesis'

describe('DashboardMainPage', () => {
  test('renders a list of hypotheses with localized status labels', async () => {
    server.use(
      http.get(HYPOTHESES_URL, () =>
        HttpResponse.json([
          { id: '1', description: 'First hypothesis', status: 'idle' },
          { id: '2', description: 'Second hypothesis', status: 'in_progress' },
        ]),
      ),
    )

    renderWithProviders(<DashboardMainPage />)

    expect(
      await screen.findByRole('heading', { level: 2, name: 'First hypothesis' }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { level: 2, name: 'Second hypothesis' }),
    ).toBeInTheDocument()

    expect(screen.getByText('Статус: В ожидании')).toBeInTheDocument()
    expect(screen.getByText('Статус: В процессе')).toBeInTheDocument()

    expect(screen.getAllByRole('button', { name: 'Открыть' })).toHaveLength(2)
  })
})
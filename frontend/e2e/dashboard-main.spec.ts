import { test, expect, type Page } from '@playwright/test'

const HYPOTHESES_URL = '**/api/v1/hypothesis'

async function authenticate(page: Page) {
  await page.addInitScript(() => {
    window.localStorage.setItem('auth-token', 'e2e-token')
  })
}

test.describe('DashboardMainPage', () => {
  test('shows the loading message while hypotheses are being fetched', async ({
    page,
  }) => {
    await authenticate(page)

    let resolveResponse!: () => void
    const responseHeld = new Promise<void>((resolve) => {
      resolveResponse = resolve
    })

    await page.route(HYPOTHESES_URL, async (route) => {
      await responseHeld
      await route.fulfill({ json: [] })
    })

    await page.goto('/')

    await expect(page.locator('body')).toMatchAriaSnapshot(`
      - banner:
        - link "Hypothesis Lab":
          - /url: /
        - button "Выйти"
      - main:
        - heading "Гипотезы" [level=1]
        - button "Создать гипотезу"
        - paragraph: Загрузка гипотез…
    `)

    resolveResponse()

    await expect(page.locator('body')).toMatchAriaSnapshot(`
      - banner:
        - link "Hypothesis Lab":
          - /url: /
        - button "Выйти"
      - main:
        - heading "Гипотезы" [level=1]
        - button "Создать гипотезу"
        - paragraph: Пока нет гипотез.
    `)
  })

  test('renders the list once hypotheses arrive', async ({ page }) => {
    await authenticate(page)

    await page.route(HYPOTHESES_URL, async (route) => {
      await route.fulfill({
        json: [
          {
            id: '1',
            description: 'First hypothesis',
            status: 'idle',
          },
          {
            id: '2',
            description: 'Second hypothesis',
            status: 'in_progress',
          },
        ],
      })
    })

    await page.goto('/')

    await expect(page.locator('body')).toMatchAriaSnapshot(`
      - banner:
        - link "Hypothesis Lab":
          - /url: /
        - button "Выйти"
      - main:
        - heading "Гипотезы" [level=1]
        - button "Создать гипотезу"
        - list:
          - listitem:
            - heading "First hypothesis" [level=2]
            - text: "Статус: В ожидании"
            - button "Открыть"
          - listitem:
            - heading "Second hypothesis" [level=2]
            - text: "Статус: В процессе"
            - button "Открыть"
    `)
  })

  test('shows the error message when the request fails', async ({ page }) => {
    await authenticate(page)

    await page.route(HYPOTHESES_URL, async (route) => {
      await route.fulfill({ status: 500, json: {} })
    })

    await page.goto('/')

    await expect(page.locator('body')).toMatchAriaSnapshot(`
      - banner:
        - link "Hypothesis Lab":
          - /url: /
        - button "Выйти"
      - main:
        - heading "Гипотезы" [level=1]
        - button "Создать гипотезу"
        - paragraph: Не удалось загрузить гипотезы
    `)
  })
})
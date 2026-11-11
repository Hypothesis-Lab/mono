import { css } from 'styled-components'

type PageLayoutOptions = {
  tabletMaxWidth: string
  desktopMaxWidth: string
}

export const pageLayout = ({ tabletMaxWidth, desktopMaxWidth }: PageLayoutOptions) => css`
  max-width: 100%;
  margin: 40px auto;
  padding: 0 16px;

  @media (min-width: 640px) {
    max-width: ${tabletMaxWidth};
    margin: 60px auto;
    padding: 0 24px;
  }

  @media (min-width: 1024px) {
    max-width: ${desktopMaxWidth};
    margin: 80px auto;
    padding: 0 32px;
  }

  @media (min-width: 1280px) {
    padding: 0 40px;
  }
`

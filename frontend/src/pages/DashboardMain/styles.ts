import styled from 'styled-components'
import { pageLayout } from '../../styles/layout'

export const Page = styled.main`
  ${pageLayout({ tabletMaxWidth: '960px', desktopMaxWidth: '1120px' })}
`

export const Title = styled.h1`
  font-size: 28px;
  margin-bottom: 8px;
  color: #111;

  @media (min-width: 640px) {
    font-size: 32px;
  }
`

export const Description = styled.p`
  font-size: 16px;
  color: #555;
  margin-bottom: 16px;
`

export const CreateButton = styled.button`
  padding: 10px 16px;
  margin-bottom: 24px;
  border: 1px solid #111;
  border-radius: 6px;
  background: #111;
  color: #fff;
  font-size: 14px;
  cursor: pointer;

  &:hover {
    background: #333;
  }
`

export const Section = styled.section`
  display: flex;
  flex-direction: column;
  gap: 16px;
`

export const Message = styled.p`
  font-size: 16px;
  color: #666;
`

export const ErrorMessage = styled.p`
  font-size: 16px;
  color: #c00;
`

export const List = styled.ul`
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
`

export const Item = styled.li`
  padding: 16px;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
  background: #fff;
  display: flex;
  flex-direction: column;
  gap: 12px;
`

export const ItemContent = styled.div`
  display: flex;
  flex-direction: column;
  gap: 8px;
`

export const ItemName = styled.h2`
  display: block;
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  color: #111;
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }

  &:focus-visible {
    outline: 2px solid #111;
    outline-offset: 3px;
  }
`

export const ItemDescription = styled.p`
  font-size: 14px;
  margin: 0;
  color: #555;
`

export const ItemMeta = styled.span`
  font-size: 12px;
  color: #888;
`

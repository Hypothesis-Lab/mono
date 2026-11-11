import { Link } from 'react-router-dom'
import styled from 'styled-components'
import { pageLayout } from '../../styles/layout'

export const Page = styled.main`
  ${pageLayout({ tabletMaxWidth: '720px', desktopMaxWidth: '960px' })}
`

export const Title = styled.h1`
  font-size: 28px;
  margin-bottom: 16px;
  color: #111;
`

export const Description = styled.p`
  font-size: 16px;
  color: #555;
  margin-bottom: 16px;
`

export const Meta = styled.p`
  font-size: 14px;
  color: #888;
`

export const Message = styled.p`
  font-size: 16px;
  color: #666;
`

export const ErrorMessage = styled.p`
  font-size: 16px;
  color: #c00;
`

export const ConnectionError = styled.div`
  display: flex;
  flex-direction: column;
  gap: 8px;
`

export const ConnectionLink = styled(Link)`
  color: #111;
  font-size: 14px;
  text-decoration: underline;
`

export const ProductSection = styled.section`
  margin-top: 24px;
  padding: 16px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: #fafafa;
`

export const SectionTitle = styled.h2`
  font-size: 18px;
  margin-bottom: 12px;
  color: #111;
`

export const ProductName = styled.p`
  font-size: 16px;
  font-weight: 600;
  color: #333;
  margin-bottom: 8px;
`

export const ButtonGroup = styled.div`
  display: flex;
  gap: 12px;
  margin-top: 24px;
`

export const EditButton = styled.button`
  padding: 10px 16px;
  border: 1px solid #111;
  border-radius: 6px;
  background: #111;
  color: #fff;
  font-size: 14px;
  cursor: pointer;

  &:hover:not(:disabled) {
    background: #333;
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
`

export const LaunchButton = styled.button`
  padding: 10px 16px;
  border: 1px solid #2a8a2a;
  border-radius: 6px;
  background: #2a8a2a;
  color: #fff;
  font-size: 14px;
  cursor: pointer;

  &:hover:not(:disabled) {
    background: #237023;
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
`

export const StopButton = styled.button`
  padding: 10px 16px;
  border: 1px solid #c00;
  border-radius: 6px;
  background: #c00;
  color: #fff;
  font-size: 14px;
  cursor: pointer;

  &:hover:not(:disabled) {
    background: #a00;
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
`

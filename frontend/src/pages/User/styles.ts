import styled from 'styled-components'
import { pageLayout } from '../../styles/layout'

export const Page = styled.main`
  ${pageLayout({ tabletMaxWidth: '640px', desktopMaxWidth: '720px' })}
`

export const Title = styled.h1`
  margin: 0 0 8px;
  color: #111;
  font-size: 28px;

  @media (min-width: 640px) {
    font-size: 32px;
  }
`

export const Description = styled.p`
  margin: 0 0 24px;
  color: #555;
  font-size: 16px;
  line-height: 1.5;
`

export const Card = styled.section`
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 16px;
  padding: 20px;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
  background: #fff;
`

export const StatusRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
`

export const StatusLabel = styled.h2`
  margin: 0;
  color: #111;
  font-size: 18px;
`

export const StatusBadge = styled.span<{ $status: 'connected' | 'disconnected' | 'revoked' }>`
  padding: 5px 9px;
  border-radius: 999px;
  background: ${({ $status }) => ($status === 'connected' ? '#e6f4ea' : '#fff3e0')};
  color: ${({ $status }) => ($status === 'connected' ? '#176b35' : '#8a4b08')};
  font-size: 13px;
  font-weight: 600;
`

export const Details = styled.dl`
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
`

export const Detail = styled.div`
  display: flex;
  justify-content: space-between;
  gap: 16px;
  color: #555;
  font-size: 14px;
`

export const DetailLabel = styled.dt``

export const DetailValue = styled.dd`
  margin: 0;
  color: #111;
  text-align: right;
`

export const Subtitle = styled.h2`
  margin: 0;
  color: #111;
  font-size: 18px;
`

export const Form = styled.form`
  display: flex;
  flex-direction: column;
  gap: 16px;
`

export const Field = styled.label`
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: #333;
  font-size: 14px;
`

export const Input = styled.input`
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
`

export const Button = styled.button<{ $primary?: boolean }>`
  align-self: flex-start;
  padding: 10px 16px;
  border: 1px solid ${({ $primary }) => ($primary ? '#111' : '#ccc')};
  border-radius: 6px;
  background: ${({ $primary }) => ($primary ? '#111' : '#fff')};
  color: ${({ $primary }) => ($primary ? '#fff' : '#111')};
  font-size: 14px;
  cursor: pointer;

  &:hover:not(:disabled) {
    background: ${({ $primary }) => ($primary ? '#333' : '#f5f5f5')};
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
`

export const Message = styled.p`
  color: #666;
  font-size: 16px;
`

export const ErrorMessage = styled.p`
  color: #c00;
  font-size: 16px;
`

export const SuccessMessage = styled.p`
  color: #176b35;
  font-size: 16px;
`

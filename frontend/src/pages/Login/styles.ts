import styled from 'styled-components'
import { pageLayout } from '../../styles/layout'

export const Page = styled.main`
  ${pageLayout({ tabletMaxWidth: '420px', desktopMaxWidth: '480px' })}
`

export const Title = styled.h1`
  font-size: 28px;
  margin-bottom: 24px;
  color: #111;
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
  font-size: 14px;
  color: #333;
`

export const Input = styled.input`
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
`

export const ErrorMessage = styled.p`
  font-size: 16px;
  color: #c00;
`

export const Button = styled.button<{ $primary?: boolean }>`
  padding: 10px;
  border: 1px solid #ccc;
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

export const Toggle = styled.button`
  padding: 0;
  margin-top: 16px;
  border: none;
  background: transparent;
  color: #111;
  font-size: 14px;
  text-decoration: underline;
  cursor: pointer;
`

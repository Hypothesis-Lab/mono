import styled from 'styled-components'
import { pageLayout } from '../../styles/layout'

export const Page = styled.main`
  ${pageLayout({ tabletMaxWidth: '560px', desktopMaxWidth: '640px' })}
`

export const Title = styled.h1`
  font-size: 28px;
  margin-bottom: 8px;
  color: #111;
`

export const Meta = styled.p`
  font-size: 14px;
  color: #888;
  margin-bottom: 24px;
`

export const Form = styled.form`
  display: flex;
  flex-direction: column;
  gap: 16px;
`

export const SectionTitle = styled.h2`
  font-size: 18px;
  margin: 0;
  color: #111;
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

export const TextArea = styled.textarea`
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
  resize: vertical;
`

export const HypothesisFields = styled.div`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: #fff;
`

export const ButtonGroup = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 8px;
`

export const Button = styled.button<{ $primary?: boolean; $danger?: boolean }>`
  padding: 10px 16px;
  border: 1px solid ${({ $danger }) => ($danger ? '#c00' : '#ccc')};
  border-radius: 6px;
  background: ${({ $primary, $danger }) =>
    $primary ? '#111' : $danger ? '#c00' : '#fff'};
  color: ${({ $primary, $danger }) => ($primary || $danger ? '#fff' : '#111')};
  font-size: 14px;
  cursor: pointer;

  &:hover:not(:disabled) {
    background: ${({ $primary, $danger }) =>
      $primary ? '#333' : $danger ? '#a00' : '#f5f5f5'};
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
`

export const Message = styled.p`
  font-size: 16px;
  color: #666;
`

export const ErrorMessage = styled.p`
  font-size: 16px;
  color: #c00;
`

export const ProductSection = styled.section`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: #fafafa;
  margin-top: 16px;
`

export const ProductName = styled.p`
  font-size: 16px;
  font-weight: 600;
  color: #333;
  margin: 0;
`

export const ProductCategory = styled.p`
  font-size: 14px;
  color: #666;
  margin: 0;
`

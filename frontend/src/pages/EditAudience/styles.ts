import styled from 'styled-components'

export const Page = styled.main`
  max-width: 100%;
  margin: 40px auto;
  padding: 0 16px;

  @media (min-width: 640px) {
    max-width: 560px;
    margin: 60px auto;
    padding: 0 24px;
  }

  @media (min-width: 1024px) {
    max-width: 640px;
    margin: 80px auto;
    padding: 0 32px;
  }

  @media (min-width: 1280px) {
    padding: 0 40px;
  }
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

export const FieldRow = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
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

export const Select = styled.select`
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
  background: #fff;
`

export const ButtonGroup = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
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

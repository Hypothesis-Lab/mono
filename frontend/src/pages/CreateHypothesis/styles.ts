import styled from 'styled-components'

export const Page = styled.main`
  max-width: 100%;
  margin: 40px auto;
  padding: 0 16px;

  @media (min-width: 640px) {
    max-width: 420px;
    margin: 60px auto;
    padding: 0 24px;
  }

  @media (min-width: 1024px) {
    max-width: 480px;
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

import { Link } from 'react-router-dom'
import styled from 'styled-components'

export const Container = styled.header`
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #e5e5e5;
  background: #f9f9f9;

  @media (min-width: 640px) {
    padding: 16px 24px;
  }

  @media (min-width: 1024px) {
    padding: 16px 32px;
  }

  @media (min-width: 1280px) {
    padding: 16px 40px;
  }
`

export const Logo = styled(Link)`
  font-size: 18px;
  font-weight: 600;
  color: #111;
  text-decoration: none;

  @media (min-width: 640px) {
    font-size: 20px;
  }
`

export const AuthButton = styled.button`
  padding: 8px 12px;
  border: 1px solid #ccc;
  border-radius: 6px;
  background: #fff;
  color: #111;
  font-size: 14px;
  cursor: pointer;

  &:hover {
    background: #f5f5f5;
  }
`

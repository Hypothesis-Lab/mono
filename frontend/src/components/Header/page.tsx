import { useLocation, useNavigate } from 'react-router-dom'
import { useAppDispatch, useAppSelector } from '../../store/hooks'
import { selectIsAuthenticated, clearToken } from '../../store/slices/authSlice'
import { api } from '../../store/generatedApi'
import { clearAuthToken } from '../../store/authToken'
import * as S from './styles'

export function Header() {
  const navigate = useNavigate()
  const location = useLocation()
  const dispatch = useAppDispatch()
  const isAuthenticated = useAppSelector(selectIsAuthenticated)

  const onAuthPage = ['/login', '/register'].includes(location.pathname)

  function handleAuthClick() {
    if (isAuthenticated) {
      clearAuthToken()
      dispatch(clearToken())
      dispatch(api.util.resetApiState())
    } else {
      navigate('/login')
    }
  }

  if (onAuthPage) {
    return (
      <S.Container>
        <S.Logo to="/">Hypothesis Lab</S.Logo>
      </S.Container>
    )
  }

  return (
    <S.Container>
      <S.Logo to="/">Hypothesis Lab</S.Logo>
      <S.Actions>
        {isAuthenticated && <S.UserLink to="/user">Yandex Direct</S.UserLink>}
        <S.AuthButton type="button" onClick={handleAuthClick}>
          {isAuthenticated ? 'Выйти' : 'Войти'}
        </S.AuthButton>
      </S.Actions>
    </S.Container>
  )
}

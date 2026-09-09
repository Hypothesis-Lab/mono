import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAppDispatch, useAppSelector } from '../../store/hooks'
import { useLoginUserMutation } from '../../store/generatedApi'
import { selectIsAuthenticated, setToken } from '../../store/slices/authSlice'
import { storeAuthToken } from '../../store/authToken'
import { rtkErrorMessage } from '../../api/structs'
import * as S from './styles'

export function LoginPage() {
  const navigate = useNavigate()
  const dispatch = useAppDispatch()
  const isAuthenticated = useAppSelector(selectIsAuthenticated)
  const [loginUser, { isLoading }] = useLoginUserMutation()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/', { replace: true })
    }
  }, [isAuthenticated, navigate])

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()

    if (email.trim() === '' || password.trim() === '') {
      setError('Email и пароль обязательны')
      return
    }

    setError(null)

    try {
      const { token } = await loginUser({
        loginUserRequest: { email: email.trim(), password: password.trim() },
      }).unwrap()
      storeAuthToken(token)
      dispatch(setToken(token))
    } catch (err) {
      setError(rtkErrorMessage(err, 'Ошибка входа'))
    }
  }

  return (
    <S.Page>
      <S.Title>Вход</S.Title>

      <S.Form onSubmit={handleSubmit}>
        <S.Field>
          Email
          <S.Input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
            disabled={isLoading}
          />
        </S.Field>

        <S.Field>
          Пароль
          <S.Input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="••••••"
            disabled={isLoading}
          />
        </S.Field>

        {error && <S.ErrorMessage>{error}</S.ErrorMessage>}

        <S.Button type="submit" $primary disabled={isLoading}>
          {isLoading ? 'Входим…' : 'Войти'}
        </S.Button>
      </S.Form>

      <S.Toggle
        type="button"
        onClick={() => navigate('/register')}
        disabled={isLoading}
      >
        Нет аккаунта? Создать
      </S.Toggle>
    </S.Page>
  )
}
import { useState } from 'react'
import { rtkErrorMessage } from '../../api/structs'
import {
  useConnectYandexDirectMutation,
  useDisconnectYandexDirectMutation,
  useGetYandexDirectAuthorizationUrlQuery,
  useGetYandexDirectConnectionQuery,
} from '../../store/generatedApi'
import type { YandexDirectConnectionResponse } from '../../store/generatedApi'
import * as S from './styles'

function formatDate(value?: string | null): string | null {
  if (!value) {
    return null
  }

  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('ru-RU')
}

function statusLabel(status: YandexDirectConnectionResponse['status']): string {
  switch (status) {
    case 'connected':
      return 'Подключено'
    case 'revoked':
      return 'Требуется переподключение'
    default:
      return 'Не подключено'
  }
}

function getDisconnectedStatus(): YandexDirectConnectionResponse {
  return {
    status: 'disconnected',
    connected_at: null,
    disconnected_at: new Date().toISOString(),
    token_expires_at: null,
  }
}

export function UserPage() {
  const {
    data: fetchedConnection,
    error: connectionError,
    isLoading: loadingConnection,
  } = useGetYandexDirectConnectionQuery()
  const {
    data: authorization,
    error: authorizationError,
    isLoading: loadingAuthorization,
  } = useGetYandexDirectAuthorizationUrlQuery()
  const [connectYandexDirect, { isLoading: connecting }] = useConnectYandexDirectMutation()
  const [disconnectYandexDirect, { isLoading: disconnecting }] = useDisconnectYandexDirectMutation()
  const [localConnection, setLocalConnection] = useState<YandexDirectConnectionResponse | null>(null)
  const [code, setCode] = useState('')
  const [showConfirmationForm, setShowConfirmationForm] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)

  const connection = localConnection ?? fetchedConnection
  const isConnected = connection?.status === 'connected'
  const isBusy = loadingAuthorization || connecting || disconnecting
  const showCodeForm = showConfirmationForm || connection?.status === 'revoked'

  function handleConnectClick() {
    setError(null)
    setMessage(null)

    if (!authorization?.authorization_url) {
      setError(
        authorizationError
          ? rtkErrorMessage(authorizationError, 'Не удалось открыть авторизацию Yandex')
          : 'Авторизация Yandex ещё не готова. Попробуйте снова через несколько секунд.',
      )
      return
    }

    window.open(authorization.authorization_url, '_blank', 'noopener,noreferrer')
    setShowConfirmationForm(true)
    setMessage('Откройте страницу Yandex и введите полученный код подтверждения ниже.')
  }

  async function handleCodeSubmit(event: React.FormEvent) {
    event.preventDefault()
    const trimmedCode = code.trim()

    if (trimmedCode === '') {
      setError('Введите код подтверждения')
      return
    }

    setError(null)
    setMessage(null)

    try {
      const nextConnection = await connectYandexDirect({
        yandexDirectConnectRequest: { code: trimmedCode },
      }).unwrap()
      setLocalConnection(nextConnection)
      setCode('')
      setShowConfirmationForm(false)
      setMessage('Yandex Direct подключён.')
    } catch (err) {
      setError(rtkErrorMessage(err, 'Не удалось подключить Yandex Direct'))
    }
  }

  async function handleDisconnect() {
    setError(null)
    setMessage(null)

    try {
      await disconnectYandexDirect().unwrap()
      setLocalConnection(getDisconnectedStatus())
      setCode('')
      setShowConfirmationForm(false)
      setMessage('Yandex Direct отключён.')
    } catch (err) {
      setError(rtkErrorMessage(err, 'Не удалось отключить Yandex Direct'))
    }
  }

  return (
    <S.Page>
      <S.Title>Yandex Direct</S.Title>
      <S.Description>Подключите рекламный кабинет, чтобы использовать Yandex Direct.</S.Description>

      {loadingConnection && <S.Message>Загружаем статус подключения…</S.Message>}

      {connectionError && (
        <S.ErrorMessage>
          {rtkErrorMessage(connectionError, 'Не удалось загрузить статус подключения')}
        </S.ErrorMessage>
      )}

      {connection && (
        <S.Card>
          <S.StatusRow>
            <S.StatusLabel>Статус</S.StatusLabel>
            <S.StatusBadge $status={connection.status}>
              {statusLabel(connection.status)}
            </S.StatusBadge>
          </S.StatusRow>

          <S.Details>
            {connection.connected_at && (
              <S.Detail>
                <S.DetailLabel>Подключено</S.DetailLabel>
                <S.DetailValue>{formatDate(connection.connected_at)}</S.DetailValue>
              </S.Detail>
            )}
            {connection.disconnected_at && (
              <S.Detail>
                <S.DetailLabel>Отключено</S.DetailLabel>
                <S.DetailValue>{formatDate(connection.disconnected_at)}</S.DetailValue>
              </S.Detail>
            )}
            {connection.token_expires_at && (
              <S.Detail>
                <S.DetailLabel>Срок действия токена</S.DetailLabel>
                <S.DetailValue>{formatDate(connection.token_expires_at)}</S.DetailValue>
              </S.Detail>
            )}
          </S.Details>

          {!isConnected && (
            <S.Button type="button" $primary onClick={handleConnectClick} disabled={isBusy}>
              {loadingAuthorization
                ? 'Подготавливаем Yandex…'
                : connection.status === 'revoked'
                  ? 'Переподключить Yandex Direct'
                  : 'Подключить Yandex Direct'}
            </S.Button>
          )}

          {isConnected && (
            <S.Button type="button" onClick={handleDisconnect} disabled={isBusy}>
              {disconnecting ? 'Отключаем…' : 'Отключить'}
            </S.Button>
          )}

          {connection.status === 'revoked' && (
            <S.Button type="button" onClick={handleDisconnect} disabled={isBusy}>
              {disconnecting ? 'Отключаем…' : 'Отключить'}
            </S.Button>
          )}
        </S.Card>
      )}

      {showCodeForm && (
        <S.Card>
          <S.Subtitle>Подтверждение подключения</S.Subtitle>
          <S.Form onSubmit={handleCodeSubmit}>
            <S.Field>
              Код подтверждения
              <S.Input
                value={code}
                onChange={(event) => setCode(event.target.value)}
                placeholder="Введите код из Yandex"
                autoComplete="one-time-code"
                disabled={isBusy}
              />
            </S.Field>
            <S.Button type="submit" $primary disabled={isBusy}>
              {connecting ? 'Подключаем…' : 'Подтвердить подключение'}
            </S.Button>
          </S.Form>
        </S.Card>
      )}

      {error && <S.ErrorMessage>{error}</S.ErrorMessage>}
      {message && <S.SuccessMessage>{message}</S.SuccessMessage>}
    </S.Page>
  )
}

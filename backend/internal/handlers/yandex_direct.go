package handlers

import (
	"context"
	"errors"
	"net/http"

	"app/internal/api"
	"app/internal/clients/yandex_oauth"
	"app/internal/http/middlewares"
	"app/internal/services"

	"go.uber.org/zap"
)

func (s *Server) GetYandexDirectAuthorizationURL(_ context.Context, _ api.GetYandexDirectAuthorizationURLRequestObject) (api.GetYandexDirectAuthorizationURLResponseObject, error) {
	return api.GetYandexDirectAuthorizationURL200JSONResponse{
		AuthorizationUrl: s.yandexDirectConnection.AuthorizationURL(),
	}, nil
}

func (s *Server) GetYandexDirectConnection(ctx context.Context, _ api.GetYandexDirectConnectionRequestObject) (api.GetYandexDirectConnectionResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("get yandex direct connection: missing user id in context")

		return api.GetYandexDirectConnection500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	status, err := s.yandexDirectConnection.Status(ctx, userID)
	if err != nil {
		s.logger.Error("get yandex direct connection", zap.Error(err))

		return api.GetYandexDirectConnection500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.GetYandexDirectConnection200JSONResponse(yandexDirectConnectionResponse(status)), nil
}

func (s *Server) ConnectYandexDirect(ctx context.Context, req api.ConnectYandexDirectRequestObject) (api.ConnectYandexDirectResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("connect yandex direct: missing user id in context")

		return api.ConnectYandexDirect500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	status, err := s.yandexDirectConnection.Connect(ctx, userID, req.Body.Code)
	if err != nil {
		if errors.Is(err, yandexoauth.ErrInvalidConfirmationCode) {
			s.logger.Error("connect yandex direct: invalid confirmation code", zap.Error(err))

			return api.ConnectYandexDirect400JSONResponse(errBody(http.StatusBadRequest, "invalid or expired confirmation code")), nil
		}

		s.logger.Error("connect yandex direct", zap.Error(err))

		return api.ConnectYandexDirect500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.ConnectYandexDirect200JSONResponse(yandexDirectConnectionResponse(status)), nil
}

func (s *Server) DisconnectYandexDirect(ctx context.Context, _ api.DisconnectYandexDirectRequestObject) (api.DisconnectYandexDirectResponseObject, error) {
	userID, ok := middlewares.UserIDFromContext(ctx)
	if !ok {
		s.logger.Error("disconnect yandex direct: missing user id in context")

		return api.DisconnectYandexDirect500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	if err := s.yandexDirectConnection.Disconnect(ctx, userID); err != nil {
		s.logger.Error("disconnect yandex direct", zap.Error(err))

		return api.DisconnectYandexDirect500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.DisconnectYandexDirect204Response{}, nil
}

func yandexDirectConnectionResponse(status *services.YandexDirectConnectionStatus) api.YandexDirectConnectionResponse {
	return api.YandexDirectConnectionResponse{
		Status:         api.YandexDirectConnectionResponseStatus(status.Status),
		ConnectedAt:    status.ConnectedAt,
		DisconnectedAt: status.DisconnectedAt,
		TokenExpiresAt: status.TokenExpiresAt,
	}
}

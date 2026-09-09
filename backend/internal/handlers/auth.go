package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"app/internal/api"
	"app/internal/services"

	"go.uber.org/zap"
)

func (s *Server) LoginUser(ctx context.Context, req api.LoginUserRequestObject) (api.LoginUserResponseObject, error) {
	token, err := s.userAuth.Login(ctx, string(req.Body.Email), req.Body.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			s.logger.Error("login user", zap.Error(err))

			return api.LoginUser401JSONResponse(errBody(http.StatusUnauthorized, "invalid credentials")), nil
		}

		s.logger.Error("login user", zap.Error(err))

		return api.LoginUser500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
	}

	return api.LoginUser200JSONResponse(api.AuthTokenResponse{Token: token}), nil
}

func (s *Server) RegisterUser(ctx context.Context, req api.RegisterUserRequestObject) (api.RegisterUserResponseObject, error) {
	token, err := s.userAuth.Register(ctx, string(req.Body.Email), req.Body.Password)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidEmail):
			s.logger.Error("register user", zap.Error(err))

			return api.RegisterUser400JSONResponse(errBody(http.StatusBadRequest, "email must be a valid email address")), nil
		case errors.Is(err, services.ErrPasswordTooShort):
			s.logger.Error("register user", zap.Error(err))

			return api.RegisterUser400JSONResponse(errBody(http.StatusBadRequest, fmt.Sprintf("password must be at least %d characters long", services.MinPasswordLength))), nil
		case errors.Is(err, services.ErrEmailExists):
			s.logger.Error("register user", zap.Error(err))

			return api.RegisterUser409JSONResponse(errBody(http.StatusConflict, "email already registered")), nil
		default:
			s.logger.Error("register user", zap.Error(err))

			return api.RegisterUser500JSONResponse(errBody(http.StatusInternalServerError, "internal server error")), nil
		}
	}

	return api.RegisterUser201JSONResponse(api.AuthTokenResponse{Token: token}), nil
}

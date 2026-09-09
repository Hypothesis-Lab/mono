package http

import (
	"fmt"

	"app/internal/api"
	"app/internal/config"
	"app/internal/http/middlewares"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

func NewRouter(server api.ServerInterface, cfg *config.Config, logger *zap.Logger) (*chi.Mux, error) {
	auth, err := middlewares.AuthMiddleware(logger)
	if err != nil {
		return nil, fmt.Errorf("build auth middleware: %w", err)
	}

	r := chi.NewRouter()

	r.Use(middlewares.RequestLogger(logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Use(middlewares.UserContextMiddleware(cfg.JWTSecret, logger))
	r.Use(auth)

	api.HandlerFromMux(server, r)

	return r, nil
}

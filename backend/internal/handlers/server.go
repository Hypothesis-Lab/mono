package handlers

import (
	"app/internal/api"
	"app/internal/clients"
	"app/internal/repositories/audience"
	"app/internal/repositories/file"
	"app/internal/repositories/hypothesis"
	"app/internal/repositories/persona1m"
	"app/internal/repositories/product"
	"app/internal/services"
	"app/internal/storage"

	"go.uber.org/zap"
)

var _ api.StrictServerInterface = (*Server)(nil)

type Server struct {
	hypotheses             *hypothesis.Repository
	products               *product.Repository
	audiences              *audience.Repository
	files                  *file.Repository
	personas1m             *persona1m.Repository
	storage                *storage.S3
	generator              *clients.GeneratorClient
	hypothesisService      *services.HypothesisService
	productService         *services.ProductService
	productFeedbackSurvey  *services.ProductFeedbackSurveyService
	userAuth               *services.UserAuthService
	yandexDirectConnection *services.YandexDirectConnectionService
	logger                 *zap.Logger
}

func NewServer(
	hypotheses *hypothesis.Repository,
	products *product.Repository,
	audiences *audience.Repository,
	files *file.Repository,
	personas1m *persona1m.Repository,
	storage *storage.S3,
	generator *clients.GeneratorClient,
	hypothesisService *services.HypothesisService,
	productService *services.ProductService,
	productFeedbackSurvey *services.ProductFeedbackSurveyService,
	userAuth *services.UserAuthService,
	yandexDirectConnection *services.YandexDirectConnectionService,
	logger *zap.Logger,
) *Server {
	return &Server{
		hypotheses:             hypotheses,
		products:               products,
		audiences:              audiences,
		files:                  files,
		personas1m:             personas1m,
		storage:                storage,
		generator:              generator,
		hypothesisService:      hypothesisService,
		productService:         productService,
		productFeedbackSurvey:  productFeedbackSurvey,
		userAuth:               userAuth,
		yandexDirectConnection: yandexDirectConnection,
		logger:                 logger,
	}
}

func NewServerInterface(s *Server) api.ServerInterface {
	return api.NewStrictHandlerWithOptions(s, nil, strictOptions(s.logger))
}

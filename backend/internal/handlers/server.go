package handlers

import (
	"app/internal/api"
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
	files                  *file.Repository
	personas1m             *persona1m.Repository
	storage                *storage.S3
	hypothesisService      *services.HypothesisService
	productService         *services.ProductService
	productAnalysis        *services.ProductAnalysisService
	productAnalysisStarter *services.ProductAnalysisStarter
	productFeedbackSurvey  *services.ProductFeedbackSurveyService
	userAuth               *services.UserAuthService
	yandexDirectConnection *services.YandexDirectConnectionService
	logger                 *zap.Logger
}

func NewServer(
	hypotheses *hypothesis.Repository,
	products *product.Repository,
	files *file.Repository,
	personas1m *persona1m.Repository,
	storage *storage.S3,
	hypothesisService *services.HypothesisService,
	productService *services.ProductService,
	productAnalysis *services.ProductAnalysisService,
	productAnalysisStarter *services.ProductAnalysisStarter,
	productFeedbackSurvey *services.ProductFeedbackSurveyService,
	userAuth *services.UserAuthService,
	yandexDirectConnection *services.YandexDirectConnectionService,
	logger *zap.Logger,
) *Server {
	return &Server{
		hypotheses:             hypotheses,
		products:               products,
		files:                  files,
		personas1m:             personas1m,
		storage:                storage,
		hypothesisService:      hypothesisService,
		productService:         productService,
		productAnalysis:        productAnalysis,
		productAnalysisStarter: productAnalysisStarter,
		productFeedbackSurvey:  productFeedbackSurvey,
		userAuth:               userAuth,
		yandexDirectConnection: yandexDirectConnection,
		logger:                 logger,
	}
}

func NewServerInterface(s *Server) api.ServerInterface {
	return api.NewStrictHandlerWithOptions(s, nil, strictOptions(s.logger))
}

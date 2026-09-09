package handlers

import (
	"app/internal/api"
	"app/internal/repositories/audience"
	"app/internal/repositories/file"
	"app/internal/repositories/hypothesis"
	"app/internal/repositories/product"
	"app/internal/services"
	"app/internal/storage"

	"go.uber.org/zap"
)

var _ api.StrictServerInterface = (*Server)(nil)

type Server struct {
	hypotheses        *hypothesis.Repository
	products          *product.Repository
	audiences         *audience.Repository
	files             *file.Repository
	storage           *storage.S3
	hypothesisService *services.HypothesisService
	productService    *services.ProductService
	userAuth          *services.UserAuthService
	logger            *zap.Logger
}

func NewServer(
	hypotheses *hypothesis.Repository,
	products *product.Repository,
	audiences *audience.Repository,
	files *file.Repository,
	storage *storage.S3,
	hypothesisService *services.HypothesisService,
	productService *services.ProductService,
	userAuth *services.UserAuthService,
	logger *zap.Logger,
) *Server {
	return &Server{
		hypotheses:        hypotheses,
		products:          products,
		audiences:         audiences,
		files:             files,
		storage:           storage,
		hypothesisService: hypothesisService,
		productService:    productService,
		userAuth:          userAuth,
		logger:            logger,
	}
}

func NewServerInterface(s *Server) api.ServerInterface {
	return api.NewStrictHandlerWithOptions(s, nil, strictOptions(s.logger))
}

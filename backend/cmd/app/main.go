package main

import (
	"net/http"

	"app/internal/config"
	"app/internal/db"
	"app/internal/handlers"
	apphttp "app/internal/http"
	"app/internal/logger"
	"app/internal/repositories/audience"
	"app/internal/repositories/file"
	"app/internal/repositories/hypothesis"
	"app/internal/repositories/product"
	"app/internal/repositories/user"
	"app/internal/seeder"
	"app/internal/services"
	"app/internal/storage"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(logger.New),
		fx.Provide(config.New),
		fx.Provide(db.NewDB),
		fx.Provide(storage.NewS3),
		fx.Provide(hypothesis.NewRepository),
		fx.Provide(product.NewRepository),
		fx.Provide(audience.NewRepository),
		fx.Provide(file.NewRepository),
		fx.Provide(user.NewRepository),
		fx.Provide(services.NewHypothesisService),
		fx.Provide(services.NewProductService),
		fx.Provide(services.NewUserAuthService),
		fx.Provide(handlers.NewServer),
		fx.Provide(handlers.NewServerInterface),
		fx.Provide(apphttp.NewRouter),
		fx.Provide(apphttp.NewServer),
		fx.Provide(seeder.New),
		fx.Invoke(func(*db.DB, *http.Server, *seeder.Seeder) {}),
	).Run()
}

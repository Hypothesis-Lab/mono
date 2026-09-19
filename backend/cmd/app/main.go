package main

import (
	"net/http"

	"app/internal/clients"
	"app/internal/clients/yandex_direct"
	"app/internal/config"
	"app/internal/db"
	"app/internal/handlers"
	apphttp "app/internal/http"
	"app/internal/logger"
	"app/internal/repositories/adcampaign"
	"app/internal/repositories/audience"
	"app/internal/repositories/file"
	"app/internal/repositories/hypothesis"
	"app/internal/repositories/hypothesislaunch"
	"app/internal/repositories/persona"
	"app/internal/repositories/product"
	"app/internal/repositories/productfeedbacksurvey"
	"app/internal/repositories/user"
	"app/internal/services"
	"app/internal/storage"
	"app/internal/temporal"

	"go.uber.org/fx"
)

const personaDatasetDirectory = "datasets/matraix-persona-dev-sample"

func main() {
	fx.New(
		fx.Supply(personaDatasetDirectory),
		fx.Provide(logger.New),
		fx.Provide(config.New),
		fx.Provide(clients.NewGeneratorClient),
		fx.Provide(clients.NewJevClient),
		fx.Provide(yandexdirect.NewFromConfig),
		fx.Provide(db.NewDB),
		fx.Provide(storage.NewS3),
		fx.Provide(temporal.NewClient),
		fx.Provide(hypothesis.NewRepository),
		fx.Provide(hypothesislaunch.NewRepository),
		fx.Provide(persona.NewRepository),
		fx.Provide(product.NewRepository),
		fx.Provide(productfeedbacksurvey.NewRepository),
		fx.Provide(audience.NewRepository),
		fx.Provide(adcampaign.NewRepository),
		fx.Provide(file.NewRepository),
		fx.Provide(user.NewRepository),
		fx.Provide(services.NewHypothesisService),
		fx.Provide(services.NewHypothesisLaunchWorker),
		fx.Provide(services.NewHypothesisLaunchStarter),
		fx.Provide(services.NewHypothesisStopStarter),
		fx.Provide(services.NewProductService),
		fx.Provide(services.NewAudienceGenerationWorker),
		fx.Provide(services.NewUserAuthService),
		fx.Provide(services.NewProductFeedbackSurveyService),
		fx.Provide(handlers.NewServer),
		fx.Provide(handlers.NewServerInterface),
		fx.Provide(apphttp.NewRouter),
		fx.Provide(apphttp.NewServer),
		fx.Invoke(func(*db.DB, *http.Server, *persona.Repository, *services.AudienceGenerationWorker, *services.HypothesisLaunchWorker, *services.HypothesisLaunchStarter, *services.HypothesisStopStarter) {
		}),
	).Run()
}

package main

import (
	"context"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ihsansolusi/lib7-service-go/logging"
	"github.com/ihsansolusi/lib7-service-go/token"
	"github.com/ihsansolusi/policy7/internal/api"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
)

func main() {
	// Load environment variables if .env file exists
	_ = godotenv.Load()

	// Initialize Logger
	log := logging.NewLogger(logging.Options{
		Level:  zerolog.DebugLevel,
		Pretty: true, // Set to true for dev
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	ctx := context.Background()

	// Initialize DB
	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		log.Fatal().Msg("DATABASE_URL is required")
	}

	dbPool, err := store.NewDBPool(ctx, dbUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer dbPool.Close()

	querier := store.New(dbPool)

	// Initialize Redis
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		log.Fatal().Msg("REDIS_URL is required")
	}

	redisCache, err := store.NewRedisCache(ctx, redisUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to redis")
	}

	// Initialize NATS (using custom policy7 NATS wrapper for event publishing/cache invalidation)
	natsUrl := os.Getenv("NATS_URL")
	if natsUrl == "" {
		natsUrl = "nats://localhost:4222"
	}

	natsClient, err := service.NewNATSClient(natsUrl, redisCache, querier)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to connect to NATS, continuing without event streaming")
	} else {
		defer natsClient.Close()
		log.Info().Msg("Successfully connected to NATS")
		if err := natsClient.StartSubscriptions(); err != nil {
			log.Error().Err(err).Msg("Failed to start NATS subscriptions")
		}
	}

	// Initialize Service
	paramSvc := service.NewParameterService(querier, redisCache)
	adminSvc := service.NewAdminParameterService(querier, redisCache, natsClient)

	// Cache warming (background)
	go func() {
		if err := paramSvc.WarmUpCache(context.Background()); err != nil {
			log.Warn().Err(err).Msg("Cache warming failed")
		}
	}()

	// Token maker for JWT validation on /v1 and /admin/v1 routes.
	// Pattern matches core7-service-enterprise and audit7: bearer token OR
	// X-Service-Key bypass for trusted service-to-service callers.
	var tokenMaker token.Maker
	if jwksURI := os.Getenv("TOKEN_JWKS"); jwksURI != "" {
		tokenMaker = token.NewRSAJWKSMaker(jwksURI, 5*time.Minute)
	} else {
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			secret = "policy7-dev-secret-32-bytes-long-changeme"
		}
		jwtMaker, err := token.NewJWTMaker(secret)
		if err != nil {
			log.Fatal().Err(err).Msg("init JWT maker failed")
		}
		tokenMaker = jwtMaker
	}

	// Initialize Gin router
	r := gin.Default()

	// Setup routes
	api.SetupRoutes(r, paramSvc, adminSvc, tokenMaker, log)

	// Start server
	log.Info().Msgf("Starting policy7 server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}

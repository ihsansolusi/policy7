package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ihsansolusi/lib7-service-go/logging"
	"github.com/ihsansolusi/lib7-service-go/metrics"
	"github.com/ihsansolusi/lib7-service-go/token"
	"github.com/ihsansolusi/lib7-service-go/tracing"
	"github.com/ihsansolusi/policy7/internal/api"
	"github.com/ihsansolusi/policy7/internal/service"
	"github.com/ihsansolusi/policy7/internal/service/branchscope"
	"github.com/ihsansolusi/policy7/internal/store"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
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
	paramSvc := service.NewParameterService(querier, redisCache, querier)
	adminSvc := service.NewAdminParameterService(querier, redisCache, natsClient)

	// Cache warming (background)
	go func() {
		if err := paramSvc.WarmUpCache(context.Background()); err != nil {
			log.Warn().Err(err).Msg("Cache warming failed")
		}
	}()

	// Branch scope sync poller (syncs from enterprise /v1/source-contracts/branch-scope)
	branchScopeURL := os.Getenv("ENTERPRISE_URL")
	if branchScopeURL != "" {
		branchScopeURL += "/v1/source-contracts/branch-scope"
	}
	orgIDStr := os.Getenv("ORG_ID")
	orgID, _ := uuid.Parse(orgIDStr)
	bsPoller := branchscope.NewPoller(branchscope.Config{
		SourceURL:     branchScopeURL,
		ClientID:      os.Getenv("M2M_CLIENT_ID"),
		ClientSecret:  os.Getenv("M2M_CLIENT_SECRET"),
		TokenEndpoint: os.Getenv("AUTH_TOKEN_ENDPOINT"),
		OrgID:         orgID,
	}, querier, log)
	go bsPoller.Start(ctx)

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

	// Initialize OTel tracer provider.
	// When OTEL_EXPORTER_OTLP_ENDPOINT is unset the global provider remains a
	// no-op, so otel.Tracer returns a no-op tracer — no export, no overhead.
	var otelTracer trace.Tracer
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		shutdownTracer, err := tracing.InitTracer(ctx, tracing.Options{
			ServiceName:  "policy7",
			OTLPEndpoint: endpoint,
			Sampling:     1.0,
		})
		if err != nil {
			log.Fatal().Err(err).Msg("failed to init tracer")
		}
		defer shutdownTracer()
		log.Info().Str("endpoint", endpoint).Msg("OTel tracer initialized")
	}
	otelTracer = otel.Tracer("policy7")

	// Initialize Prometheus metrics registry.
	metricsReg := metrics.New("policy7")

	// Expose /metrics on a separate port (default :9090).
	// This keeps the scrape endpoint off the main service port so it is not
	// accidentally exposed through the API gateway.
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(metricsReg.Prometheus(), promhttp.HandlerOpts{}))
		metricsPort := os.Getenv("METRICS_PORT")
		if metricsPort == "" {
			metricsPort = "9090"
		}
		log.Info().Msgf("Starting metrics server on port %s", metricsPort)
		if err := http.ListenAndServe(":"+metricsPort, mux); err != nil {
			log.Error().Err(err).Msg("metrics server stopped")
		}
	}()

	// Initialize Gin router with explicit middleware (no gin.Default built-ins).
	r := gin.New()

	// Setup routes (attaches global middleware + all route groups)
	api.SetupRoutes(r, paramSvc, adminSvc, tokenMaker, log, otelTracer, metricsReg)

	// Start server
	log.Info().Msgf("Starting policy7 server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
	}
}

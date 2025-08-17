package main

import (
	"context"
	"errors"
	config "github.com/NordCoder/Pingerus/internal/config/api-gateway"
	checkdomain "github.com/NordCoder/Pingerus/internal/domain/check"
	"github.com/NordCoder/Pingerus/internal/services/api-gateway/auth"
	check "github.com/NordCoder/Pingerus/internal/services/api-gateway/check"
	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc/reflection"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	"github.com/NordCoder/Pingerus/internal/obs"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"

	pbauth "github.com/NordCoder/Pingerus/generated/v1"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

func main() {
	cfg, err := config.Load("../config/api-gateway.yaml")
	if err != nil {
		panic(err)
	}

	logger, err := obs.NewLogger(obs.LogConfig{
		Level: cfg.Log.Level, Pretty: cfg.Log.Pretty,
		App: cfg.App.Name, Env: cfg.App.Env, Ver: cfg.App.Version,
	})
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	logger.Info("starting api-gateway", zap.Any("env", cfg.App.Env))

	ctx := context.Background()
	otelCloser, err := obs.SetupOTel(ctx, obs.OTELConfig{
		Enable: cfg.OTEL.Enable, Endpoint: cfg.OTEL.OTLPEndpoint,
		ServiceName: cfg.OTEL.ServiceName, SampleRatio: cfg.OTEL.SampleRatio,
	})
	if err != nil {
		logger.Fatal("otel init", zap.Error(err))
	}
	defer otelCloser.Shutdown(context.Background())

	db, err := pg.NewDB(ctx, pg.Config{
		DSN:               cfg.DB.DSN,
		MaxConns:          cfg.DB.MaxConns,
		MinConns:          cfg.DB.MinConns,
		MaxConnLifetime:   cfg.DB.MaxConnLifetime,
		MaxConnIdleTime:   cfg.DB.MaxConnIdleTime,
		HealthCheckPeriod: cfg.DB.HealthCheckPeriod,
		QueryTimeout:      cfg.DB.QueryTimeout,
	})
	if err != nil {
		logger.Fatal("db connect", zap.Error(err))
	}
	defer db.Close()

	var checkRepo checkdomain.Repo = pg.NewCheckRepo(db)
	checkUC := check.NewUsecase(checkRepo)
	checkServ := check.NewServer(logger, checkUC)

	userRepo := pg.NewUserRepo(db)
	rtRepo := pg.NewRefreshTokenRepo(db)

	authUsecase := auth.NewUseCase(
		userRepo, rtRepo,
		auth.Config{
			Secret:     []byte(cfg.Auth.JWTSecret),
			AccessTTL:  cfg.Auth.AccessTTL,
			RefreshTTL: cfg.Auth.RefreshTTL,
		},
	)

	authServer := auth.NewServer(
		authUsecase, userRepo,
		auth.Opts{
			CookieName:   cfg.Auth.CookieName,
			CookieDomain: cfg.Auth.CookieDomain,
			CookiePath:   cfg.Auth.CookiePath,
			CookieSecure: cfg.Auth.CookieSecure,
			RefreshTTL:   cfg.Auth.RefreshTTL,
		},
	)

	grpcMetrics := grpcprometheus.NewServerMetrics()
	opts := obs.GRPCServerOpts()

	opts = append(opts,
		grpc.ChainUnaryInterceptor(
			auth.UnaryAuthInterceptor(authUsecase.ParseAccess),
		),
	)

	grpcServer := grpc.NewServer(opts...)
	grpcMetrics.InitializeMetrics(grpcServer)

	pb.RegisterCheckServiceServer(grpcServer, checkServ)

	pbauth.RegisterAuthServiceServer(grpcServer, authServer)
	reflection.Register(grpcServer)

	grpcLn, err := net.Listen("tcp", cfg.Server.GRPCAddr)
	if err != nil {
		logger.Fatal("grpc listen", zap.Error(err))
	}

	mux := runtime.NewServeMux()
	dialOpts := []grpc.DialOption{grpc.WithInsecure()}
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := pb.RegisterCheckServiceHandlerFromEndpoint(ctx, mux, cfg.Server.GRPCAddr, dialOpts); err != nil {
			logger.Fatal("register http gateway", zap.Error(err))
		}

		if err := pbauth.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, cfg.Server.GRPCAddr, dialOpts); err != nil {
			logger.Fatal("register auth http gateway", zap.Error(err))
		}
	}

	root := http.NewServeMux()
	root.Handle("/", mux)
	root.Handle("/metrics", obs.MetricsHandler())
	root.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		hctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		if err := db.Pool.Ping(hctx); err != nil {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	httpSrv := &http.Server{
		Addr:         cfg.Server.HTTPAddr,
		Handler:      root,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	errCh := make(chan error, 2)
	go func() {
		logger.Info("grpc listening", zap.String("addr", cfg.Server.GRPCAddr))
		errCh <- grpcServer.Serve(grpcLn)
	}()
	go func() {
		logger.Info("http listening", zap.String("addr", cfg.Server.HTTPAddr))
		errCh <- httpSrv.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case s := <-sig:
		logger.Info("shutdown signal", zap.String("sig", s.String()))
	case err = <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", zap.Error(err))
		}
	}

	shCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.GracefulTimeout)
	defer cancel()
	grpcServer.GracefulStop()
	_ = httpSrv.Shutdown(shCtx)
	logger.Info("bye")
}

type checkService struct {
	pb.UnimplementedCheckServiceServer
	uc *check.Usecase
}

func NewCheckServiceServer(uc *check.Usecase) *checkService {
	return &checkService{uc: uc}
}

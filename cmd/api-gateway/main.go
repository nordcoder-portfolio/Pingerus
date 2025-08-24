package main

import (
	"context"
	"errors"
	"google.golang.org/grpc/connectivity"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/NordCoder/Pingerus/internal/config/api-gateway"
	checkdomain "github.com/NordCoder/Pingerus/internal/domain/check"
	"github.com/NordCoder/Pingerus/internal/obs"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"
	"github.com/NordCoder/Pingerus/internal/services/api-gateway/auth"
	check "github.com/NordCoder/Pingerus/internal/services/api-gateway/check"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	pbauth "github.com/NordCoder/Pingerus/generated/v1"

	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func dialGRPCBlocking(ctx context.Context, target string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	conn.Connect()

	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return conn, nil
		}
		if ok := conn.WaitForStateChange(ctx, state); !ok {
			_ = conn.Close()
			return nil, context.DeadlineExceeded
		}
	}
}

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
	defer func() { _ = logger.Sync() }()
	logger.Info("starting api-gateway", zap.Any("env", cfg.App.Env))

	ctx := context.Background()
	otelCloser, err := obs.SetupOTel(ctx, obs.OTELConfig{
		Enable: cfg.OTEL.Enable, Endpoint: cfg.OTEL.OTLPEndpoint,
		ServiceName: cfg.OTEL.ServiceName, SampleRatio: cfg.OTEL.SampleRatio,
	})
	if err != nil {
		logger.Fatal("otel init", zap.Error(err))
	}
	defer func() { _ = otelCloser.Shutdown(context.Background()) }()

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
	checkSrv := check.NewServer(logger, checkUC)

	userRepo := pg.NewUserRepo(db)
	rtRepo := pg.NewRefreshTokenRepo(db)
	authUC := auth.NewUseCase(
		userRepo, rtRepo,
		auth.Config{
			Secret:     []byte(cfg.Auth.JWTSecret),
			AccessTTL:  cfg.Auth.AccessTTL,
			RefreshTTL: cfg.Auth.RefreshTTL,
		},
	)
	authSrv := auth.NewServer(
		authUC, userRepo,
		auth.Opts{
			Logger:       logger,
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
			grpcMetrics.UnaryServerInterceptor(),
			auth.UnaryAuthInterceptor(authUC.ParseAccess),
		),
		grpc.ChainStreamInterceptor(
			grpcMetrics.StreamServerInterceptor(),
		),
	)

	grpcServer := grpc.NewServer(opts...)
	grpcMetrics.InitializeMetrics(grpcServer)

	pb.RegisterCheckServiceServer(grpcServer, checkSrv)
	pbauth.RegisterAuthServiceServer(grpcServer, authSrv)

	reflection.Register(grpcServer)

	grpcLn, err := net.Listen("tcp", cfg.Server.GRPCAddr)
	if err != nil {
		logger.Fatal("grpc listen", zap.Error(err))
	}

	go func() {
		logger.Info("grpc listening", zap.String("addr", cfg.Server.GRPCAddr))
		if err := grpcServer.Serve(grpcLn); err != nil {
			logger.Error("grpc serve", zap.Error(err))
		}
	}()

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dialCancel()
	conn, err := dialGRPCBlocking(dialCtx, cfg.Server.GRPCAddr)
	if err != nil {
		logger.Fatal("grpc dial for gateway", zap.Error(err))
	}
	defer func() { _ = conn.Close() }()

	mux := runtime.NewServeMux()
	if err := pb.RegisterCheckServiceHandler(context.Background(), mux, conn); err != nil {
		logger.Fatal("register check http gateway", zap.Error(err))
	}
	if err := pbauth.RegisterAuthServiceHandler(context.Background(), mux, conn); err != nil {
		logger.Fatal("register auth http gateway", zap.Error(err))
	}

	root := http.NewServeMux()
	root.Handle("/", mux)
	root.Handle("/metrics", obs.MetricsHandler())
	root.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		hctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		if err := db.Pool.Ping(hctx); err != nil {
			http.Error(w, "unhealthy: db", http.StatusServiceUnavailable)
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
		logger.Info("http listening", zap.String("addr", cfg.Server.HTTPAddr))
		errCh <- httpSrv.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	var serveErr error
	select {
	case s := <-sig:
		logger.Info("shutdown signal", zap.String("sig", s.String()))
	case serveErr = <-errCh:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("server error", zap.Error(serveErr))
		}
	}

	shCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.GracefulTimeout)
	defer cancel()
	grpcServer.GracefulStop()
	_ = httpSrv.Shutdown(shCtx)
	logger.Info("bye")
}

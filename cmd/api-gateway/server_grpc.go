package main

import (
	pb "github.com/NordCoder/Pingerus/generated/v1"
	"github.com/NordCoder/Pingerus/internal/domain/check"
	"github.com/NordCoder/Pingerus/internal/obs"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"
	"github.com/NordCoder/Pingerus/internal/services/api-gateway/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"

	pbauth "github.com/NordCoder/Pingerus/generated/v1"
	checksvc "github.com/NordCoder/Pingerus/internal/services/api-gateway/check"

	config "github.com/NordCoder/Pingerus/internal/config/api-gateway"
	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
)

func buildGRPCServer(cfg *config.Config, logger *zap.Logger, db *pg.DB) (*grpc.Server, net.Listener, *grpcprometheus.ServerMetrics, error) {
	var checkRepo check.Repo = pg.NewCheckRepo(db)
	checkUC := checksvc.NewUsecase(checkRepo)
	checkSrv := checksvc.NewServer(logger, checkUC)

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

	ln, err := net.Listen("tcp", cfg.Server.GRPCAddr)
	if err != nil {
		return nil, nil, nil, err
	}

	return grpcServer, ln, grpcMetrics, nil
}

func serveGRPC(s *grpc.Server, ln net.Listener, cfg *config.Config, logger *zap.Logger) error {
	logger.Info("grpc listening", zap.String("addr", cfg.Server.GRPCAddr))
	return s.Serve(ln)
}

func gracefulStopGRPC(s *grpc.Server) { s.GracefulStop() }

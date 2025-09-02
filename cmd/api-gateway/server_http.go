package main

import (
	"context"
	pb "github.com/NordCoder/Pingerus/generated/v1"
	"github.com/NordCoder/Pingerus/internal/obs"
	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"net/http"
	"time"

	config "github.com/NordCoder/Pingerus/internal/config/api-gateway"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"

	pbauth "github.com/NordCoder/Pingerus/generated/v1"
)

func buildHTTPServer(ctx context.Context, cfg *config.Config, logger *zap.Logger, db *pg.DB, grpcMetrics *grpcprometheus.ServerMetrics) (*http.Server, error) {
	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dialCancel()

	conn, err := grpc.DialContext(
		dialCtx,
		cfg.Server.GRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}

	mux := runtime.NewServeMux()
	if err := pb.RegisterCheckServiceHandler(context.Background(), mux, conn); err != nil {
		return nil, err
	}
	if err := pbauth.RegisterAuthServiceHandler(context.Background(), mux, conn); err != nil {
		return nil, err
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
		_, _ = w.Write([]byte("ok"))
	})

	handler := cors([]string{"http://frontend:80"})(root) // todo config cors

	httpSrv := &http.Server{
		Addr:              cfg.Server.HTTPAddr,
		Handler:           handler,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}
	_ = grpcMetrics
	return httpSrv, nil
}

func serveHTTP(srv *http.Server, cfg *config.Config, logger *zap.Logger) error {
	logger.Info("http listening", zap.String("addr", cfg.Server.HTTPAddr))
	return srv.ListenAndServe()
}

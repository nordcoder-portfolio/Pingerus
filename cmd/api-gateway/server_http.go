package main

import (
	"context"
	config "github.com/NordCoder/Pingerus/internal/config/api-gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"net/http"
	"time"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	pbauth "github.com/NordCoder/Pingerus/generated/v1"
	"github.com/NordCoder/Pingerus/internal/obs"
	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"

	grpcprometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
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

func buildHTTPServer(
	ctx context.Context,
	cfg *config.Config,
	logger *zap.Logger,
	db *pg.DB,
	grpcMetrics *grpcprometheus.ServerMetrics,
) (*http.Server, *grpc.ClientConn, error) {

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := dialGRPCBlocking(dialCtx, cfg.Server.GRPCAddr)
	if err != nil {
		return nil, nil, err
	}

	mux := runtime.NewServeMux()
	if err := pb.RegisterCheckServiceHandler(ctx, mux, conn); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	if err := pbauth.RegisterAuthServiceHandler(ctx, mux, conn); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	root := http.NewServeMux()
	root.Handle("/", mux)
	root.Handle("/metrics", obs.MetricsHandler())
	root.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		hctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		if err := db.Pool.Ping(hctx); err != nil {
			http.Error(w, "unhealthy: db", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// todo: вынести в конфиг
	handler := cors([]string{"http://frontend:80"})(root)

	httpSrv := &http.Server{
		Addr:              cfg.Server.HTTPAddr,
		Handler:           handler,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}
	_ = grpcMetrics

	return httpSrv, conn, nil
}

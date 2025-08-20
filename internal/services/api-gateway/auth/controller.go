package auth

import (
	"context"
	"errors"
	"github.com/NordCoder/Pingerus/internal/domain/auth"
	"net/http"
	"strings"
	"time"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	"github.com/NordCoder/Pingerus/internal/domain/user"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	pb.UnimplementedAuthServiceServer
	log          *zap.Logger
	uc           auth.Usecase
	users        user.Repo
	cookieName   string
	cookieDomain string
	cookiePath   string
	cookieSecure bool
	refreshTTL   time.Duration
}

type Opts struct {
	Logger       *zap.Logger
	CookieName   string
	CookieDomain string
	CookiePath   string
	CookieSecure bool
	RefreshTTL   time.Duration
}

func NewServer(uc auth.Usecase, users user.Repo, o Opts) *Server {
	log := o.Logger
	if log == nil {
		log, _ = zap.NewProduction()
	}
	return &Server{
		log:          log,
		uc:           uc,
		users:        users,
		cookieName:   o.CookieName,
		cookieDomain: o.CookieDomain,
		cookiePath:   o.CookiePath,
		cookieSecure: o.CookieSecure,
		refreshTTL:   o.RefreshTTL,
	}
}

func (s *Server) SignUp(ctx context.Context, req *pb.SignUpRequest) (*pb.AuthResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	s.log.Info("auth.signup", zap.String("email", req.GetEmail()))

	u, access, refresh, err := s.uc.SignUp(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, s.mapErr(err)
	}

	s.setRefreshCookie(ctx, refresh)
	return &pb.AuthResponse{AccessToken: access, User: toPBUser(u)}, nil
}

func (s *Server) SignIn(ctx context.Context, req *pb.SignInRequest) (*pb.AuthResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	s.log.Info("auth.signin", zap.String("email", req.GetEmail()))

	u, access, refresh, err := s.uc.SignIn(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, s.mapErr(err)
	}

	s.setRefreshCookie(ctx, refresh)
	return &pb.AuthResponse{AccessToken: access, User: toPBUser(u)}, nil
}

func (s *Server) Refresh(ctx context.Context, _ *emptypb.Empty) (*pb.AccessTokenResponse, error) {
	raw := s.getRefreshFromCtx(ctx)

	s.log.Info("auth.refresh")

	access, refresh, _, err := s.uc.Refresh(ctx, raw)
	if err != nil {
		s.clearRefreshCookie(ctx)
		return nil, s.mapErr(err)
	}

	s.setRefreshCookie(ctx, refresh)
	return &pb.AccessTokenResponse{AccessToken: access}, nil
}

func (s *Server) Logout(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	raw := s.getRefreshFromCtx(ctx)

	_ = s.uc.Logout(ctx, raw)

	s.clearRefreshCookie(ctx)

	s.log.Info("auth.logout")

	return &emptypb.Empty{}, nil
}

func (s *Server) Me(ctx context.Context, _ *emptypb.Empty) (*pb.User, error) {
	token := bearer(ctx)

	id, err := s.uc.ParseAccess(token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid access token")
	}

	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	return toPBUser(u), nil
}

func (s *Server) mapErr(err error) error {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, ErrEmailExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, ErrWeakPassword):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return err
	}
}

func toPBUser(u *user.User) *pb.User {
	return &pb.User{
		Id:        u.ID,
		Email:     u.Email,
		CreatedAt: timestamppb.New(u.CreatedAt),
		UpdatedAt: timestamppb.New(u.UpdatedAt),
	}
}

func (s *Server) setRefreshCookie(ctx context.Context, raw string) {
	maxAge := int(s.refreshTTL.Seconds())
	expires := time.Now().Add(s.refreshTTL).UTC()
	c := &http.Cookie{
		Name:     s.cookieName,
		Value:    raw,
		Path:     s.cookiePath,
		Domain:   s.cookieDomain,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  expires,
	}
	md := metadata.Pairs("Set-Cookie", c.String())
	_ = grpc.SetHeader(ctx, md)
}

func (s *Server) clearRefreshCookie(ctx context.Context) {
	c := &http.Cookie{
		Name:     s.cookieName,
		Value:    "",
		Path:     s.cookiePath,
		Domain:   s.cookieDomain,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
	}
	md := metadata.Pairs("Set-Cookie", c.String())
	_ = grpc.SetHeader(ctx, md)
}

func (s *Server) getRefreshFromCtx(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("grpcgateway-cookie"); len(vals) > 0 {
			for _, v := range vals {
				if raw := parseCookie(v, s.cookieName); raw != "" {
					return raw
				}
			}
		}
		if vals := md.Get("cookie"); len(vals) > 0 {
			for _, v := range vals {
				if raw := parseCookie(v, s.cookieName); raw != "" {
					return raw
				}
			}
		}
		if vals := md.Get("x-refresh-token"); len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	return ""
}

func parseCookie(header, name string) string {
	parts := strings.Split(header, ";")
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 && kv[0] == name {
			return kv[1]
		}
	}
	return ""
}

func bearer(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("authorization"); len(vals) > 0 {
			v := vals[0]
			if strings.HasPrefix(strings.ToLower(v), "bearer ") {
				return strings.TrimSpace(v[7:])
			}
			return v
		}
	}
	return ""
}

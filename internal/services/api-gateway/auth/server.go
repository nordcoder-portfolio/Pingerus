package auth

import (
	"context"
	"github.com/NordCoder/Pingerus/internal/domain/user"
	"github.com/golang/protobuf/ptypes/empty"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	pb.UnimplementedAuthServiceServer
	uc           *Usecase
	users        user.Repo
	cookieName   string
	cookieDomain string
	cookiePath   string
	cookieSecure bool
	refreshTTL   time.Duration
}

type Opts struct {
	CookieName   string
	CookieDomain string
	CookiePath   string
	CookieSecure bool
	RefreshTTL   time.Duration
}

func NewServer(uc *Usecase, users user.Repo, o Opts) *Server {
	return &Server{
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
	u, access, refresh, err := s.uc.SignUp(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	s.setRefreshCookie(ctx, refresh)
	return &pb.AuthResponse{
		AccessToken: access,
		User:        toPBUser(u),
	}, nil
}

func (s *Server) SignIn(ctx context.Context, req *pb.SignInRequest) (*pb.AuthResponse, error) {
	u, access, refresh, err := s.uc.SignIn(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	s.setRefreshCookie(ctx, refresh)
	return &pb.AuthResponse{
		AccessToken: access,
		User:        toPBUser(u),
	}, nil
}

func (s *Server) Refresh(ctx context.Context, _ *empty.Empty) (*pb.AccessTokenResponse, error) {
	raw := s.getRefreshFromCtx(ctx)
	access, refresh, _, err := s.uc.Refresh(ctx, raw)
	if err != nil {
		s.clearRefreshCookie(ctx)
		return nil, err
	}
	s.setRefreshCookie(ctx, refresh)
	return &pb.AccessTokenResponse{AccessToken: access}, nil
}

func (s *Server) Logout(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	raw := s.getRefreshFromCtx(ctx)
	_ = s.uc.Logout(ctx, raw)
	s.clearRefreshCookie(ctx)
	return &empty.Empty{}, nil
}

func (s *Server) Me(ctx context.Context, _ *empty.Empty) (*pb.User, error) {
	token := bearer(ctx)
	id, err := s.uc.ParseAccess(token)
	if err != nil {
		return nil, err
	}
	u, err := s.users.GetByID(id)
	if err != nil {
		return nil, err
	}
	return toPBUser(u), nil
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
	c := &http.Cookie{
		Name:     s.cookieName,
		Value:    raw,
		Path:     s.cookiePath,
		Domain:   s.cookieDomain,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
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
	cs := strings.Split(header, ";")
	for _, p := range cs {
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

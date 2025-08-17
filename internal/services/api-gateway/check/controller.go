package check

import (
	"context"
	"errors"
	"time"

	"github.com/NordCoder/Pingerus/internal/domain/check"
	"github.com/NordCoder/Pingerus/internal/services/api-gateway/auth"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	pb.UnimplementedCheckServiceServer
	log *zap.Logger
	uc  *Usecase
}

func NewServer(log *zap.Logger, uc *Usecase) *Server {
	return &Server{log: log, uc: uc}
}

func toPB(c *check.Check) *pb.Check {
	chk := &pb.Check{
		Id:          c.ID,
		UserId:      c.UserID,
		Url:         c.URL,
		IntervalSec: int32(c.Interval / time.Second),
		NextRun:     timestamppb.New(c.NextRun),
		UpdatedAt:   timestamppb.New(c.UpdatedAt),
		LastStatus:  nil,
	}
	if c.LastStatus != nil {
		chk.LastStatus = c.LastStatus
	}
	return chk
}

func fromPB(in *pb.Check) *check.Check {
	var ls *bool
	last := in.GetLastStatus()
	ls = &last

	return &check.Check{
		ID:         in.GetId(),
		UserID:     in.GetUserId(),
		URL:        in.Url,
		Interval:   time.Duration(in.GetIntervalSec()) * time.Second,
		LastStatus: ls,
		NextRun:    in.GetNextRun().AsTime(),
		UpdatedAt:  in.GetUpdatedAt().AsTime(),
		Active:     true,
	}
}

func (s *Server) userID(ctx context.Context) (int64, error) {
	uid, ok := auth.UserIDFromCtx(ctx)
	if !ok {
		return 0, status.Error(codes.Unauthenticated, "auth required")
	}
	return uid, nil
}

func (s *Server) mapErr(err error) error {
	switch {
	case errors.Is(err, ErrInvalidInterval):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	default:
		return err
	}
}

func (s *Server) CreateCheck(ctx context.Context, req *pb.CreateCheckRequest) (*pb.CreateCheckResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	uid, err := s.userID(ctx)
	if err != nil {
		return nil, err
	}

	s.log.Info("CreateCheck request", zap.Int64("uid", uid), zap.String("url", req.GetUrl()), zap.Int32("interval_sec", req.GetIntervalSec()))

	c, err := s.uc.Create(ctx, uid, req.GetUrl(), time.Duration(req.GetIntervalSec())*time.Second)
	if err != nil {
		return nil, s.mapErr(err)
	}
	return &pb.CreateCheckResponse{Check: toPB(c)}, nil
}

func (s *Server) GetCheck(ctx context.Context, req *pb.GetCheckRequest) (*pb.Check, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	uid, err := s.userID(ctx)
	if err != nil {
		return nil, err
	}
	s.log.Info("GetCheck request", zap.Int64("uid", uid), zap.Int64("id", req.GetId()))

	c, err := s.uc.Get(ctx, uid, req.GetId())
	if err != nil {
		return nil, s.mapErr(err)
	}
	return toPB(c), nil
}

func (s *Server) UpdateCheck(ctx context.Context, req *pb.UpdateCheckRequest) (*pb.Check, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	uid, err := s.userID(ctx)
	if err != nil {
		return nil, err
	}
	in := req.GetCheck()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "empty check payload")
	}

	d := fromPB(in)
	d.UserID = uid

	s.log.Info("UpdateCheck request", zap.Int64("uid", uid), zap.Int64("id", in.GetId()), zap.String("url", d.URL), zap.Duration("interval", d.Interval))

	updated, err := s.uc.Update(ctx, uid, d)
	if err != nil {
		return nil, s.mapErr(err)
	}
	return toPB(updated), nil
}

func (s *Server) DeleteCheck(ctx context.Context, req *pb.DeleteCheckRequest) (*emptypb.Empty, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	uid, err := s.userID(ctx)
	if err != nil {
		return nil, err
	}

	s.log.Info("DeleteCheck request", zap.Int64("uid", uid), zap.Int64("id", req.GetId()))

	if err := s.uc.Delete(ctx, uid, req.GetId()); err != nil {
		return nil, s.mapErr(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListChecks(ctx context.Context, req *pb.ListChecksRequest) (*pb.ListChecksResponse, error) {
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	uid, err := s.userID(ctx)
	if err != nil {
		return nil, err
	}

	s.log.Info("ListChecks request", zap.Int64("uid", uid))

	list, err := s.uc.ListByUser(ctx, uid)
	if err != nil {
		return nil, s.mapErr(err)
	}
	out := make([]*pb.Check, 0, len(list))
	for _, c := range list {
		out = append(out, toPB(c))
	}
	return &pb.ListChecksResponse{Checks: out}, nil
}

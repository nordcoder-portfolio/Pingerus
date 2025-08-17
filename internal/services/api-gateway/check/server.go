package check

import (
	"context"
	"github.com/NordCoder/Pingerus/internal/domain/check"
	"github.com/NordCoder/Pingerus/internal/services/api-gateway/auth"
	"time"

	pb "github.com/NordCoder/Pingerus/generated/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type GrpcServer struct {
	pb.UnimplementedCheckServiceServer
	uc *Usecase
}

func NewServer(uc *Usecase) *GrpcServer { return &GrpcServer{uc: uc} }

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

func (s *GrpcServer) CreateCheck(ctx context.Context, req *pb.CreateCheckRequest) (*pb.CreateCheckResponse, error) {
	uid, ok := auth.UserIDFromCtx(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "auth required")
	}

	c := &check.Check{
		UserID:   uid,
		URL:      req.GetHost(),
		Interval: time.Duration(req.GetIntervalSec()) * time.Second,
		Active:   true,
	}
	created, err := s.uc.Create(ctx, c)
	if err != nil {
		return nil, err
	}
	return &pb.CreateCheckResponse{Check: toPB(created)}, nil
}

func (s *GrpcServer) GetCheck(ctx context.Context, req *pb.GetCheckRequest) (*pb.Check, error) {
	uid, ok := auth.UserIDFromCtx(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "auth required")
	}
	chk, err := s.uc.Get(ctx, int64(req.GetId()))
	if err != nil {
		return nil, err
	}
	if chk.UserID != uid {
		return nil, status.Error(codes.PermissionDenied, "not your check")
	}
	return toPB(chk), nil
}

func (s *GrpcServer) UpdateCheck(ctx context.Context, req *pb.UpdateCheckRequest) (*pb.Check, error) {
	uid, ok := auth.UserIDFromCtx(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "auth required")
	}
	in := req.GetCheck()
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "empty check payload")
	}

	current, err := s.uc.Get(ctx, in.GetId())
	if err != nil {
		return nil, err
	}
	if current.UserID != uid {
		return nil, status.Error(codes.PermissionDenied, "not your check")
	}

	d := fromPB(in)
	d.UserID = uid

	updated, err := s.uc.Update(ctx, d)
	if err != nil {
		return nil, err
	}
	return toPB(updated), nil
}

func (s *GrpcServer) DeleteCheck(ctx context.Context, req *pb.DeleteCheckRequest) (*emptypb.Empty, error) {
	uid, ok := auth.UserIDFromCtx(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "auth required")
	}
	chk, err := s.uc.Get(ctx, int64(req.GetId()))
	if err != nil {
		return nil, err
	}
	if chk.UserID != uid {
		return nil, status.Error(codes.PermissionDenied, "not your check")
	}

	if err := s.uc.Delete(ctx, int64(req.GetId())); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *GrpcServer) ListChecks(ctx context.Context, _ *pb.ListChecksRequest) (*pb.ListChecksResponse, error) {
	uid, ok := auth.UserIDFromCtx(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "auth required")
	}

	list, err := s.uc.ListByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.Check, 0, len(list))
	for _, c := range list {
		out = append(out, toPB(c))
	}
	return &pb.ListChecksResponse{Checks: out}, nil
}

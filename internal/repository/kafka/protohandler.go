package kafka

import (
	"context"

	"google.golang.org/protobuf/proto"
)

func ProtoHandler[M proto.Message](ctor func() M, handle func(context.Context, []byte, M) error) Handler {
	return func(ctx context.Context, key, value []byte) error {
		msg := ctor()
		if err := proto.Unmarshal(value, msg); err != nil {
			return err
		}
		return handle(ctx, key, msg)
	}
}

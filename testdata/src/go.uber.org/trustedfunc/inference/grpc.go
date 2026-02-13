package inference

import (
	"stubs/google.golang.org/grpc/codes"
	"stubs/google.golang.org/grpc/status"
)

func grpcStatusErrorTest(c string) (*int, error) {
	switch c {
	case "status.Error with non-OK code":
		// status.Error with a non-OK code always returns non-nil error.
		// NilAway should not flag this.
		return nil, status.Error(codes.InvalidArgument, "input is invalid")
	case "status.Error (false negative - OK code)":
		// status.Error with codes.OK technically returns nil, but we model
		// it as non-nil for simplicity. This is a conscious trade-off.
		return nil, status.Error(codes.OK, "this is ok")
	case "status.Errorf with non-OK code":
		// status.Errorf variant should also be modeled as non-nil.
		return nil, status.Errorf(codes.NotFound, "resource %s not found", "foo")
	}
	i := 0
	return &i, nil
}

package validator

import (
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"google.golang.org/grpc"
)

// Middlewares returns middlewares (unary and stream)
// for validation of user input values
func Middlewares() []grpc.ServerOption {
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpc_validator.UnaryServerInterceptor()),
		grpc.StreamInterceptor(grpc_validator.StreamServerInterceptor()),
	}

	return opts
}

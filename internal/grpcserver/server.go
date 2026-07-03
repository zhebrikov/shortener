// Package grpcserver реализует gRPC-транспорт сервиса сокращения ссылок.
package grpcserver

import (
	"context"
	"errors"
	"log"

	"github.com/zhebrikov/shortener/internal/app"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/service"
	pb "github.com/zhebrikov/shortener/pkg/shortenerv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Server реализует ShortenerService.
type Server struct {
	pb.UnimplementedShortenerServiceServer
	app       *app.ShortenerApp
	secretKey string
}

// NewServer создаёт gRPC-обработчик.
func NewServer(shortenerApp *app.ShortenerApp, secretKey string) *Server {
	return &Server{app: shortenerApp, secretKey: secretKey}
}

// Register регистрирует сервис и auth-interceptor на gRPC-сервере.
func Register(grpcServer *grpc.Server, shortenerApp *app.ShortenerApp, secretKey string) {
	srv := NewServer(shortenerApp, secretKey)
	pb.RegisterShortenerServiceServer(grpcServer, srv)
}

// authUnaryInterceptor добавляет user id в контекст из metadata authorization.
func authUnaryInterceptor(secretKey string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = auth.ContextFromMetadata(ctx, secretKey)
		return handler(ctx, req)
	}
}

// NewGRPCServer создаёт настроенный grpc.Server с auth-interceptor.
func NewGRPCServer(shortenerApp *app.ShortenerApp, secretKey string) *grpc.Server {
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(authUnaryInterceptor(secretKey)))
	Register(grpcServer, shortenerApp, secretKey)
	return grpcServer
}

// ShortenURL сокращает URL.
func (s *Server) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	result, err := s.app.ShortenURL(ctx, req.GetUrl())
	if err != nil {
		log.Printf("ShortenURL: %v", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return pb.URLShortenResponse_builder{
		Result: proto.String(result.ShortURL),
	}.Build(), nil
}

// ExpandURL возвращает оригинальный URL по id.
func (s *Server) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	originalURL, err := s.app.ExpandURL(ctx, req.GetId())
	if errors.Is(err, service.ErrLinkDeleted) {
		return nil, status.Error(codes.FailedPrecondition, "link deleted")
	}
	if errors.Is(err, service.ErrLinkNotFound) {
		return nil, status.Error(codes.NotFound, "link not found")
	}
	if err != nil {
		log.Printf("ExpandURL: %v", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return pb.URLExpandResponse_builder{
		Result: proto.String(originalURL),
	}.Build(), nil
}

// ListUserURLs возвращает ссылки текущего пользователя.
func (s *Server) ListUserURLs(ctx context.Context, _ *pb.ListUserURLsRequest) (*pb.UserURLsResponse, error) {
	links, err := s.app.ListUserURLs(ctx)
	if errors.Is(err, app.ErrUnauthorized) || errors.Is(err, app.ErrEmptyAuth) {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	if err != nil {
		log.Printf("ListUserURLs: %v", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	out := make([]*pb.URLData, 0, len(links))
	for _, l := range links {
		out = append(out, pb.URLData_builder{
			ShortUrl:    proto.String(l.ShortURL),
			OriginalUrl: proto.String(l.OriginalURL),
		}.Build())
	}
	return pb.UserURLsResponse_builder{Url: out}.Build(), nil
}

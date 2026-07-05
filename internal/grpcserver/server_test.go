package grpcserver

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/zhebrikov/shortener/internal/app"
	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/service"
	"github.com/zhebrikov/shortener/internal/storage"
	pb "github.com/zhebrikov/shortener/pkg/shortenerv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func startTestClient(t *testing.T) (pb.ShortenerServiceClient, func()) {
	t.Helper()
	const secret = "grpc-test-secret"

	store := storage.NewMemoryStorage()
	shortener := service.NewShortener("localhost:8080")
	shortenerApp := app.NewShortenerApp(shortener, store, nil)
	grpcSrv := NewGRPCServer(shortenerApp, secret)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = grpcSrv.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		_ = conn.Close()
		grpcSrv.GracefulStop()
	}
	return pb.NewShortenerServiceClient(conn), cleanup
}

func TestGRPC_ShortenAndExpand(t *testing.T) {
	client, cleanup := startTestClient(t)
	defer cleanup()

	ctx := context.Background()
	shortenResp, err := client.ShortenURL(ctx, pb.URLShortenRequest_builder{
		Url: proto.String("https://example.com/grpc"),
	}.Build())
	if err != nil {
		t.Fatalf("ShortenURL: %v", err)
	}
	if shortenResp.GetResult() == "" {
		t.Fatal("empty shorten result")
	}

	shortCode := shortenResp.GetResult()[len(shortenResp.GetResult())-8:]
	expandResp, err := client.ExpandURL(ctx, pb.URLExpandRequest_builder{
		Id: proto.String(shortCode),
	}.Build())
	if err != nil {
		t.Fatalf("ExpandURL: %v", err)
	}
	if expandResp.GetResult() != "https://example.com/grpc" {
		t.Fatalf("result = %q", expandResp.GetResult())
	}
}

func TestGRPC_ListUserURLs_requiresAuth(t *testing.T) {
	client, cleanup := startTestClient(t)
	defer cleanup()

	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", ""))
	_, err := client.ListUserURLs(ctx, pb.ListUserURLsRequest_builder{}.Build())
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, err = %v", status.Code(err), err)
	}
}

func TestGRPC_ListUserURLs_withAuth(t *testing.T) {
	client, cleanup := startTestClient(t)
	defer cleanup()

	const secret = "grpc-test-secret"
	userID := "user-grpc-1"
	token := auth.SignUserID(userID, secret)
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", token))

	shortenResp, err := client.ShortenURL(ctx, pb.URLShortenRequest_builder{
		Url: proto.String("https://example.com/mine-grpc"),
	}.Build())
	if err != nil {
		t.Fatalf("ShortenURL: %v", err)
	}

	listResp, err := client.ListUserURLs(ctx, pb.ListUserURLsRequest_builder{}.Build())
	if err != nil {
		t.Fatalf("ListUserURLs: %v", err)
	}
	if len(listResp.GetUrl()) != 1 {
		t.Fatalf("urls = %+v", listResp.GetUrl())
	}
	if listResp.GetUrl()[0].GetOriginalUrl() != "https://example.com/mine-grpc" {
		t.Fatalf("item = %+v", listResp.GetUrl()[0])
	}
	if listResp.GetUrl()[0].GetShortUrl() != shortenResp.GetResult() {
		t.Fatalf("short url mismatch")
	}
}

func TestGRPC_ExpandURL_notFound(t *testing.T) {
	client, cleanup := startTestClient(t)
	defer cleanup()

	_, err := client.ExpandURL(context.Background(), pb.URLExpandRequest_builder{
		Id: proto.String("missing1"),
	}.Build())
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v, err = %v", status.Code(err), err)
	}
}

func TestGRPC_ExpandURL_deleted(t *testing.T) {
	const secret = "grpc-test-secret"
	store := storage.NewMemoryStorage()
	shortener := service.NewShortener("localhost:8080")
	shortenerApp := app.NewShortenerApp(shortener, store, nil)
	grpcSrv := NewGRPCServer(shortenerApp, secret)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.GracefulStop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := pb.NewShortenerServiceClient(conn)

	_ = store.WriteStorage(storage.Link{
		UUID: 1, ShortURL: "http://localhost/deleted1", OriginalURL: "https://deleted.example", IsDeleted: true,
	})

	_, err = client.ExpandURL(context.Background(), pb.URLExpandRequest_builder{
		Id: proto.String("deleted1"),
	}.Build())
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v, err = %v", status.Code(err), err)
	}
}

func TestGRPC_ShortenURL_internalError(t *testing.T) {
	const secret = "grpc-test-secret"
	store := storage.NewMemoryStorage()
	shortener := service.NewShortener("http://%zz")
	shortenerApp := app.NewShortenerApp(shortener, store, nil)
	grpcSrv := NewGRPCServer(shortenerApp, secret)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.GracefulStop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := pb.NewShortenerServiceClient(conn)

	_, err = client.ShortenURL(context.Background(), pb.URLShortenRequest_builder{
		Url: proto.String("https://example.com/bad-base"),
	}.Build())
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, err = %v", status.Code(err), err)
	}
	if st, ok := status.FromError(err); !ok || st.Message() != "internal error" {
		t.Fatalf("message = %q, want %q", st.Message(), "internal error")
	}
}

func TestGRPC_ListUserURLs_storeError(t *testing.T) {
	store := &listErrorStore{}
	shortener := service.NewShortener("localhost:8080")
	shortenerApp := app.NewShortenerApp(shortener, store, nil)
	srv := NewServer(shortenerApp, "secret")

	ctx := auth.WithUserID(context.Background(), "user-1")
	_, err := srv.ListUserURLs(ctx, pb.ListUserURLsRequest_builder{}.Build())
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, err = %v", status.Code(err), err)
	}
	if st, ok := status.FromError(err); !ok || st.Message() != "internal error" {
		t.Fatalf("message = %q, want %q", st.Message(), "internal error")
	}
}

type listErrorStore struct {
	storage.MemoryStorage
}

func (listErrorStore) GetLinksByUserID(string) ([]storage.Link, error) {
	return nil, errors.New("list failed")
}

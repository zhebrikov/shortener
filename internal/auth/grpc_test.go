package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

func TestContextFromMetadata_emptyAuthorization(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", ""))
	got := ContextFromMetadata(ctx, "secret")
	if !EmptyAuthCookieFromContext(got) {
		t.Fatal("expected empty auth cookie context")
	}
}

func TestContextFromMetadata_missingAuthorization(t *testing.T) {
	got := ContextFromMetadata(context.Background(), "secret")
	if _, ok := UserIDFromContext(got); !ok {
		t.Fatal("expected anonymous user id")
	}
}

func TestContextFromMetadata_validJWT(t *testing.T) {
	const secret = "grpc-auth-secret"
	userID := uuid.NewString()
	token := SignUserID(userID, secret)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", token))
	got := ContextFromMetadata(ctx, secret)
	gotID, ok := UserIDFromContext(got)
	if !ok || gotID != userID {
		t.Fatalf("user id = %q, ok = %v", gotID, ok)
	}
}

func TestContextFromMetadata_bearerPrefix(t *testing.T) {
	const secret = "grpc-auth-secret"
	userID := uuid.NewString()
	token := SignUserID(userID, secret)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token))
	got := ContextFromMetadata(ctx, secret)
	gotID, ok := UserIDFromContext(got)
	if !ok || gotID != userID {
		t.Fatalf("user id = %q, ok = %v", gotID, ok)
	}
}

package auth

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

const metadataAuthorization = "authorization"

// ContextFromMetadata извлекает authorization из gRPC metadata и устанавливает user id в контекст.
// Поведение аналогично HTTP-middleware: пустое значение — 401 на защищённых методах,
// отсутствие или невалидный токен — новый анонимный пользователь.
func ContextFromMetadata(ctx context.Context, secret string) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return WithUserID(ctx, uuid.NewString())
	}
	values := md.Get(metadataAuthorization)
	if len(values) == 0 {
		return WithUserID(ctx, uuid.NewString())
	}
	raw := strings.TrimSpace(values[0])
	if raw == "" {
		return WithEmptyAuthCookie(ctx)
	}
	token := raw
	if strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		token = strings.TrimSpace(raw[7:])
	}
	if userID, ok := ParseAndVerify(token, secret); ok {
		return WithUserID(ctx, userID)
	}
	return WithUserID(ctx, uuid.NewString())
}

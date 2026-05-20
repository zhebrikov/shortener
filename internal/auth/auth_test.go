package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignUserID_ParseAndVerify_roundTrip(t *testing.T) {
	const secret = "test-secret-key"
	id := "user-123"
	signed := SignUserID(id, secret)
	got, ok := ParseAndVerify(signed, secret)
	if !ok || got != id {
		t.Fatalf("got %q, ok=%v", got, ok)
	}
}

func TestParseAndVerify_unexpectedAlg(t *testing.T) {
	// Токен с alg=RS256 без валидной подписи — ParseAndVerify должен отклонить алгоритм.
	signed := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1MSJ9.signature"
	if _, ok := ParseAndVerify(signed, "secret"); ok {
		t.Fatal("expected invalid token for non-HS256 alg")
	}
}

func TestParseAndVerify_wrongSecret(t *testing.T) {
	signed := SignUserID("u1", "secret-a")
	if _, ok := ParseAndVerify(signed, "secret-b"); ok {
		t.Fatal("expected invalid token")
	}
}

func TestParseAndVerify_emptySubject(t *testing.T) {
	const secret = "secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, userClaims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "   "},
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ParseAndVerify(signed, secret); ok {
		t.Fatal("expected invalid token for empty subject")
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := WithUserID(t.Context(), "uid-1")
	got, ok := UserIDFromContext(ctx)
	if !ok || got != "uid-1" {
		t.Fatalf("UserIDFromContext = %q, %v", got, ok)
	}
	if _, ok := UserIDFromContext(t.Context()); ok {
		t.Fatal("expected missing user id")
	}

	emptyCtx := WithEmptyAuthCookie(t.Context())
	if !EmptyAuthCookieFromContext(emptyCtx) {
		t.Fatal("expected empty auth cookie flag")
	}
	if EmptyAuthCookieFromContext(t.Context()) {
		t.Fatal("expected false for plain context")
	}
}

func TestMiddleware_newUser(t *testing.T) {
	const secret = "middleware-secret"
	var gotUser string
	h := Middleware(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, _ = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	c := rr.Result().Cookies()
	if len(c) == 0 || c[0].Name != CookieName {
		t.Fatalf("cookies = %+v", c)
	}
	if gotUser == "" {
		t.Fatal("user id not set in context")
	}
	if uid, ok := ParseAndVerify(c[0].Value, secret); !ok || uid != gotUser {
		t.Fatalf("cookie uid = %q, ctx uid = %q", uid, gotUser)
	}
}

func TestMiddleware_validCookie(t *testing.T) {
	const secret = "middleware-secret"
	uid := "existing-user"
	signed := SignUserID(uid, secret)

	var gotUser string
	h := Middleware(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, _ = UserIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: signed})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if gotUser != uid {
		t.Fatalf("got %q, want %q", gotUser, uid)
	}
}

func TestMiddleware_emptyCookieValue(t *testing.T) {
	h := Middleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !EmptyAuthCookieFromContext(r.Context()) {
			t.Fatal("expected empty auth cookie context")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "   "})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestMiddleware_invalidCookieIssuesNew(t *testing.T) {
	h := Middleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uid, ok := UserIDFromContext(r.Context()); !ok || uid == "" {
			t.Fatal("expected new user id")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "bad.token.value"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if len(rr.Result().Cookies()) == 0 {
		t.Fatal("expected new cookie")
	}
}

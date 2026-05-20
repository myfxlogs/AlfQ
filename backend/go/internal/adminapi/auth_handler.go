// Package adminapi — AuthService handler with JWT, argon2id, TOTP, backed by PostgreSQL.
package adminapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"connectrpc.com/connect"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/db/pg"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// AuthHandler implements alfqv1connect.AuthServiceHandler, backed by PG users table.
type AuthHandler struct {
	kp  *auth.KeyPair
	pg  *pg.Pool
	rdb redis.UniversalClient
}

// NewAuthHandler creates an AuthService handler backed by PG and Redis.
func NewAuthHandler(pgPool *pg.Pool, rdb redis.UniversalClient) (*AuthHandler, error) {
	kp, err := auth.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("auth handler: keypair: %w", err)
	}
	return &AuthHandler{kp: kp, pg: pgPool, rdb: rdb}, nil
}

// dbUser is the row shape of the users table.
type dbUser struct {
	UserID       string
	TenantID     string
	Email        string
	PasswordHash string
	Roles        []string
}

// getUserByEmail looks up a user by email from PG.
func (h *AuthHandler) getUserByEmail(ctx context.Context, email string) (*dbUser, error) {
	u := &dbUser{}
	err := h.pg.QueryRow(ctx, `
		SELECT id, tenant_id, email, password_hash, roles
		FROM users WHERE email = $1
	`, email).Scan(&u.UserID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Roles)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: query user: %w", err)
	}
	return u, nil
}

// getUserByID looks up a user by ID from PG.
func (h *AuthHandler) getUserByID(ctx context.Context, userID string) (*dbUser, error) {
	u := &dbUser{}
	err := h.pg.QueryRow(ctx, `
		SELECT id, tenant_id, email, password_hash, roles
		FROM users WHERE id = $1
	`, userID).Scan(&u.UserID, &u.TenantID, &u.Email, &u.PasswordHash, &u.Roles)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: query user: %w", err)
	}
	return u, nil
}

// Login authenticates a user and returns JWT tokens.
func (h *AuthHandler) Login(ctx context.Context, req *connect.Request[pb.LoginRequest]) (*connect.Response[pb.LoginResponse], error) {
	email := req.Msg.Email
	password := req.Msg.Password
	totpCode := req.Msg.TotpCode

	u, err := h.getUserByEmail(ctx, email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("auth: %w", err))
	}
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid credentials"))
	}

	valid, err := auth.VerifyPassword(u.PasswordHash, password)
	if err != nil || !valid {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid credentials"))
	}

	if totpCode != "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("totp not supported"))
	}

	accessToken, err := h.kp.Sign(auth.Claims{
		Sub:      u.UserID,
		TenantID: u.TenantID,
		Email:    u.Email,
		Roles:    u.Roles,
	}, 15*time.Minute)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("token sign: %w", err))
	}

	var refreshToken string
	if h.rdb != nil {
		var err error
		refreshToken, err = generateRefreshToken()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("refresh token: %w", err))
		}
		hash := sha256Hex(refreshToken)
		if err := h.rdb.Set(ctx, "refresh:"+hash, u.UserID, 7*24*time.Hour).Err(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("redis: %w", err))
		}
	}

	return connect.NewResponse(&pb.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
	}), nil
}

// RefreshToken validates a refresh token and issues new tokens.
// Returns an error when Redis is unavailable (refresh tokens require Redis).
func (h *AuthHandler) RefreshToken(ctx context.Context, req *connect.Request[pb.RefreshTokenRequest]) (*connect.Response[pb.LoginResponse], error) {
	if h.rdb == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("refresh tokens unavailable: redis not configured"))
	}
	hash := sha256Hex(req.Msg.RefreshToken)
	userID, err := h.rdb.Get(ctx, "refresh:"+hash).Result()
	if err == redis.Nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid refresh token"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("redis: %w", err))
	}

	u, err := h.getUserByID(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("user not found"))
	}

	h.rdb.Del(ctx, "refresh:"+hash)

	accessToken, err := h.kp.Sign(auth.Claims{
		Sub:      u.UserID,
		TenantID: u.TenantID,
		Email:    u.Email,
		Roles:    u.Roles,
	}, 15*time.Minute)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("token sign: %w", err))
	}

	newRefresh, err := generateRefreshToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("refresh token: %w", err))
	}
	newHash := sha256Hex(newRefresh)
	if err := h.rdb.Set(ctx, "refresh:"+newHash, u.UserID, 7*24*time.Hour).Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("redis: %w", err))
	}

	return connect.NewResponse(&pb.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefresh,
		ExpiresIn:    900,
	}), nil
}

// VerifyTOTP validates a TOTP code for the given user.
func (h *AuthHandler) VerifyTOTP(ctx context.Context, req *connect.Request[pb.VerifyTOTPRequest]) (*connect.Response[pb.LoginResponse], error) {
	u, err := h.getUserByID(ctx, req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if u == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("totp not implemented"))
}

// Logout invalidates the access token by adding it to a blacklist.
// Blacklist is skipped when Redis is unavailable.
func (h *AuthHandler) Logout(ctx context.Context, req *connect.Request[pb.LogoutRequest]) (*connect.Response[pb.LogoutResponse], error) {
	token := req.Msg.AccessToken
	if token == "" {
		return connect.NewResponse(&pb.LogoutResponse{}), nil
	}
	if h.rdb == nil {
		return connect.NewResponse(&pb.LogoutResponse{}), nil
	}
	claims, err := auth.Verify(token, map[string]auth.Ed25519PublicKey{h.kp.Kid: h.kp.PublicKey})
	if err != nil {
		return connect.NewResponse(&pb.LogoutResponse{}), nil
	}
	ttl := time.Until(time.Unix(claims.Exp, 0))
	if ttl > 0 {
		hash := sha256Hex(token)
		h.rdb.Set(ctx, "bl:"+hash, "1", ttl)
	}
	return connect.NewResponse(&pb.LogoutResponse{}), nil
}

// IsTokenBlacklisted checks if a token has been revoked.
// Returns false when Redis is unavailable (blacklist check skipped).
func (h *AuthHandler) IsTokenBlacklisted(ctx context.Context, token string) bool {
	if h.rdb == nil {
		return false
	}
	hash := sha256Hex(token)
	_, err := h.rdb.Get(ctx, "bl:"+hash).Result()
	return err == nil
}

func generateRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

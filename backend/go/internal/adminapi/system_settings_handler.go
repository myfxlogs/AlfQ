// Package adminapi — SystemSettingsService + API key management (R10).
package adminapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/alfq/backend/go/internal/common/auth"
	"github.com/alfq/backend/go/internal/common/crypto"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func (s *Service) GetSystemSettings(ctx context.Context, _ *pb.GetSystemSettingsRequest) (*pb.GetSystemSettingsResponse, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, value, COALESCE(description,'') FROM system_settings ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("get system settings: %w", err)
	}
	defer rows.Close()

	var settings []*pb.SystemSetting
	for rows.Next() {
		st := &pb.SystemSetting{}
		if err := rows.Scan(&st.Key, &st.Value, &st.Description); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		settings = append(settings, st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// R10: Also return user API key entries as system settings
	userID := auth.UserFromContext(ctx)
	if userID != "" {
		keyRows, err := s.pool.Query(ctx,
			`SELECT provider, key_prefix, model, quota_limit_cents FROM user_api_keys WHERE user_id=$1`, userID)
		if err == nil {
			defer keyRows.Close()
			for keyRows.Next() {
				var provider, keyPrefix, model string
				var quotaLimit int
				if err := keyRows.Scan(&provider, &keyPrefix, &model, &quotaLimit); err != nil {
					continue
				}
				settings = append(settings,
					&pb.SystemSetting{Key: "provider:" + provider + ".key_prefix", Value: keyPrefix, Description: "API Key (已加密)"},
					&pb.SystemSetting{Key: "provider:" + provider + ".model", Value: model, Description: "模型"},
					&pb.SystemSetting{Key: "provider:" + provider + ".quota", Value: fmt.Sprintf("%d", quotaLimit), Description: "月度预算(美分)"},
				)
			}
		}
	}

	return &pb.GetSystemSettingsResponse{Settings: settings}, nil
}

func (s *Service) UpdateSystemSetting(ctx context.Context, req *pb.UpdateSystemSettingRequest) (*pb.UpdateSystemSettingResponse, error) {
	key := req.Key
	value := req.Value

	// R10: Route API key settings to user_api_keys table
	if strings.HasPrefix(key, "provider:") {
		return s.updateAPIKeySetting(ctx, key, value)
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO system_settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = now()`,
		key, value,
	)
	if err != nil {
		return nil, fmt.Errorf("update system setting: %w", err)
	}
	return &pb.UpdateSystemSettingResponse{}, nil
}

// updateAPIKeySetting handles provider:* keys by routing to user_api_keys table.
// Supported sub-keys: provider:<name>.key, provider:<name>.model, provider:<name>.quota
func (s *Service) updateAPIKeySetting(ctx context.Context, key, value string) (*pb.UpdateSystemSettingResponse, error) {
	userID := auth.UserFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("未登录，无法保存 API Key")
	}

	// Parse: provider:<name>.<field>
	parts := strings.SplitN(key, ".", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid provider key format: %s", key)
	}
	provider := parts[1] // "openai", "anthropic", "deepseek", "groq", "ollama", "custom"
	field := parts[2]    // "key", "model", "quota", "endpoint"

	// Ensure encryption key is available
	if s.encCipher == nil {
		return nil, fmt.Errorf("encryption not configured")
	}

	switch field {
	case "key":
		// AES encrypt the key
		keyCipher, err := s.encCipher.Encrypt(value)
		if err != nil {
			return nil, fmt.Errorf("encrypt key: %w", err)
		}
		keyPrefix := crypto.MaskKey(value)
		_, err = s.pool.Exec(ctx,
			`INSERT INTO user_api_keys (user_id, provider, key_cipher, key_prefix)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (user_id, provider) DO UPDATE
			 SET key_cipher=$3, key_prefix=$4, updated_at=now()`,
			userID, provider, keyCipher, keyPrefix,
		)
		if err != nil {
			return nil, fmt.Errorf("save api key: %w", err)
		}
		s.log.Info("api key saved")

	case "model":
		_, err := s.pool.Exec(ctx,
			`INSERT INTO user_api_keys (user_id, provider, model, key_cipher, key_prefix)
			 VALUES ($1, $2, $3, '', '')
			 ON CONFLICT (user_id, provider) DO UPDATE SET model=$3, updated_at=now()`,
			userID, provider, value,
		)
		if err != nil {
			return nil, fmt.Errorf("save model: %w", err)
		}

	case "quota":
		_, err := s.pool.Exec(ctx,
			`INSERT INTO user_api_keys (user_id, provider, quota_limit_cents, key_cipher, key_prefix)
			 VALUES ($1, $2, $3::int, '', '')
			 ON CONFLICT (user_id, provider) DO UPDATE SET quota_limit_cents=$3::int, updated_at=now()`,
			userID, provider, value,
		)
		if err != nil {
			return nil, fmt.Errorf("save quota: %w", err)
		}

	case "endpoint":
		// R10-ext: store custom API endpoint in system_settings (not user_api_keys)
		_, err := s.pool.Exec(ctx,
			`INSERT INTO system_settings (key, value) VALUES ($1, $2)
			 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = now()`,
			key, value,
		)
		if err != nil {
			return nil, fmt.Errorf("save endpoint: %w", err)
		}

	default:
		return nil, fmt.Errorf("unknown provider field: %s", field)
	}

	return &pb.UpdateSystemSettingResponse{}, nil
}

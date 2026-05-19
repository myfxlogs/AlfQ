// Package adminapi — SystemSettingsService RPC handler implementations.
package adminapi

import (
	"context"
	"fmt"

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
	return &pb.GetSystemSettingsResponse{Settings: settings}, rows.Err()
}

func (s *Service) UpdateSystemSetting(ctx context.Context, req *pb.UpdateSystemSettingRequest) (*pb.UpdateSystemSettingResponse, error) {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO system_settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = now()`,
		req.Key, req.Value,
	)
	if err != nil {
		return nil, fmt.Errorf("update system setting: %w", err)
	}
	return &pb.UpdateSystemSettingResponse{}, nil
}

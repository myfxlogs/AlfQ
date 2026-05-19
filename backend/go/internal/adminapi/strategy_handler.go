// Package adminapi — StrategyService RPC handler implementations.
package adminapi

import (
	"context"
	"fmt"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func (s *Service) CreateStrategy(ctx context.Context, req *pb.CreateStrategyRequest) (*pb.Strategy, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	st := &pb.Strategy{}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO strategies (tenant_id, name, description, spec, status)
		VALUES ($1, $2, $3, $4, 'draft')
		RETURNING id, tenant_id, name, description, spec::text, status
	`, req.TenantId, req.Name, req.Description, req.SpecJson).Scan(
		&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("create strategy: %w", err)
	}
	return st, nil
}

func (s *Service) GetStrategy(ctx context.Context, req *pb.GetStrategyRequest) (*pb.Strategy, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	st := &pb.Strategy{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, COALESCE(description,''), spec::text, status
		FROM strategies WHERE id = $1
	`, req.Id).Scan(&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status)
	if err != nil {
		return nil, fmt.Errorf("get strategy: %w", err)
	}
	return st, nil
}

func (s *Service) ListStrategies(ctx context.Context, req *pb.ListStrategiesRequest) (*pb.ListStrategiesResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	tenantID := effectiveTenantID(ctx, req.TenantId)
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, COALESCE(description,''), spec::text, status
		FROM strategies WHERE tenant_id = $1 ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list strategies: %w", err)
	}
	defer rows.Close()

	var strategies []*pb.Strategy
	for rows.Next() {
		st := &pb.Strategy{}
		if err := rows.Scan(&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status); err != nil {
			return nil, fmt.Errorf("scan strategy: %w", err)
		}
		strategies = append(strategies, st)
	}
	return &pb.ListStrategiesResponse{Strategies: strategies}, rows.Err()
}

func (s *Service) DeployStrategy(ctx context.Context, req *pb.DeployStrategyRequest) (*pb.Strategy, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	st := &pb.Strategy{}
	err := s.pool.QueryRow(ctx, `
		UPDATE strategies SET status = 'deployed'
		WHERE id = $1
		RETURNING id, tenant_id, name, COALESCE(description,''), spec::text, status
	`, req.Id).Scan(&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status)
	if err != nil {
		return nil, fmt.Errorf("deploy strategy: %w", err)
	}
	return st, nil
}

func (s *Service) StopStrategy(ctx context.Context, req *pb.StopStrategyRequest) (*pb.Strategy, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	st := &pb.Strategy{}
	err := s.pool.QueryRow(ctx, `
		UPDATE strategies SET status = 'stopped'
		WHERE id = $1
		RETURNING id, tenant_id, name, COALESCE(description,''), spec::text, status
	`, req.Id).Scan(&st.Id, &st.TenantId, &st.Name, &st.Description, &st.SpecJson, &st.Status)
	if err != nil {
		return nil, fmt.Errorf("stop strategy: %w", err)
	}
	return st, nil
}

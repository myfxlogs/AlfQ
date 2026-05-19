// Package adminapi — BrokerService RPC handler implementations.
package adminapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/alfq/backend/go/internal/mdgateway/adapter/mtapi"

	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

func (s *Service) CreateBroker(ctx context.Context, req *pb.CreateBrokerRequest) (*pb.Broker, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	b := &pb.Broker{}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO brokers (tenant_id, code, name, platform, mtapi_endpoint, default_server)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, tenant_id, code, name, platform, mtapi_endpoint, default_server
	`, req.TenantId, req.Code, req.Name, req.Platform, req.MtapiEndpoint, "").Scan(
		&b.Id, &b.TenantId, &b.Code, &b.Name, &b.Platform, &b.MtapiEndpoint, &b.DefaultServer,
	)
	if err != nil {
		return nil, fmt.Errorf("create broker: %w", err)
	}
	return b, nil
}

func (s *Service) GetBroker(ctx context.Context, req *pb.GetBrokerRequest) (*pb.Broker, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	b := &pb.Broker{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, code, name, platform, mtapi_endpoint, COALESCE(default_server,'')
		FROM brokers WHERE id = $1
	`, req.Id).Scan(&b.Id, &b.TenantId, &b.Code, &b.Name, &b.Platform, &b.MtapiEndpoint, &b.DefaultServer)
	if err != nil {
		return nil, fmt.Errorf("get broker: %w", err)
	}
	return b, nil
}

func (s *Service) ListBrokers(ctx context.Context, req *pb.ListBrokersRequest) (*pb.ListBrokersResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	tenantID := effectiveTenantID(ctx, req.TenantId)
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, code, name, platform, mtapi_endpoint, COALESCE(default_server,'')
		FROM brokers WHERE tenant_id = $1 ORDER BY code
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list brokers: %w", err)
	}
	defer rows.Close()

	var brokers []*pb.Broker
	for rows.Next() {
		b := &pb.Broker{}
		if err := rows.Scan(&b.Id, &b.TenantId, &b.Code, &b.Name, &b.Platform, &b.MtapiEndpoint, &b.DefaultServer); err != nil {
			return nil, fmt.Errorf("scan broker: %w", err)
		}
		brokers = append(brokers, b)
	}
	return &pb.ListBrokersResponse{Brokers: brokers}, rows.Err()
}

func (s *Service) UpdateBroker(ctx context.Context, req *pb.Broker) (*pb.Broker, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	b := &pb.Broker{}
	err := s.pool.QueryRow(ctx, `
		UPDATE brokers SET code=$1, name=$2, platform=$3, mtapi_endpoint=$4, default_server=$5
		WHERE id = $6
		RETURNING id, tenant_id, code, name, platform, mtapi_endpoint, COALESCE(default_server,'')
	`, req.Code, req.Name, req.Platform, req.MtapiEndpoint, req.DefaultServer, req.Id).Scan(
		&b.Id, &b.TenantId, &b.Code, &b.Name, &b.Platform, &b.MtapiEndpoint, &b.DefaultServer,
	)
	if err != nil {
		return nil, fmt.Errorf("update broker: %w", err)
	}
	return b, nil
}

func (s *Service) SearchBroker(ctx context.Context, req *pb.SearchBrokerRequest) (*pb.SearchBrokerResponse, error) {
	// 1. Try DB first
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, code, name, platform, mtapi_endpoint, COALESCE(default_server,'')
		 FROM brokers WHERE platform = $1 AND (name ILIKE '%' || $2 || '%' OR code ILIKE '%' || $2 || '%')
		 ORDER BY name LIMIT 20`,
		req.Platform, req.Keyword,
	)
	if err == nil {
		defer rows.Close()
		var matches []*pb.BrokerMatch
		for rows.Next() {
			b := &pb.Broker{}
			if err := rows.Scan(&b.Id, &b.TenantId, &b.Code, &b.Name, &b.Platform, &b.MtapiEndpoint, &b.DefaultServer); err != nil {
				continue
			}
			matches = append(matches, &pb.BrokerMatch{
				Company: b.Name,
				Servers: []*pb.BrokerServer{{Name: b.DefaultServer, Access: b.MtapiEndpoint}},
			})
		}
		if len(matches) > 0 {
			return &pb.SearchBrokerResponse{Matches: matches}, nil
		}
	}

	// 2. Try online gateway search (MT4 and MT5)
	gw := s.mt5Gateway
	if strings.EqualFold(req.Platform, "MT4") {
		gw = s.mt4Gateway
	}
	kw := req.Keyword
	if kw == "" {
		kw = "tr" // gateway requires ≥2 chars; "tr" yields broad coverage (~450 brokers)
	}
	if onlineMatches, err := mtapi.SearchBrokersOnline(ctx, gw, req.Platform, kw); err == nil && len(onlineMatches) > 0 {
		var matches []*pb.BrokerMatch
		for _, m := range onlineMatches {
			var servers []*pb.BrokerServer
			for _, s := range m.Servers {
				servers = append(servers, &pb.BrokerServer{Name: s.Name, Access: s.Access})
			}
			matches = append(matches, &pb.BrokerMatch{
				Company: m.Company,
				Servers: servers,
			})
		}
		return &pb.SearchBrokerResponse{Matches: matches}, nil
	}

	// 3. No results
	return &pb.SearchBrokerResponse{}, nil
}

func (s *Service) DeleteBroker(ctx context.Context, req *pb.DeleteBrokerRequest) (*pb.DeleteBrokerResponse, error) {
	if err := s.setRLS(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM brokers WHERE id = $1`, req.Id)
	if err != nil {
		return nil, fmt.Errorf("delete broker: %w", err)
	}
	return &pb.DeleteBrokerResponse{}, nil
}

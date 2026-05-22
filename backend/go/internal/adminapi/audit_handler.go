// Package adminapi — AuditService handler.
package adminapi

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
	"github.com/alfq/backend/go/internal/common/auth"
)

func (s *Service) ListAuditLogs(ctx context.Context, req *pb.ListAuditLogsRequest) (*pb.ListAuditLogsResponse, error) {
	if err := RequireTenant(ctx); err != nil {
		return nil, fmt.Errorf("rls: %w", err)
	}
	if s.pool == nil {
		return &pb.ListAuditLogsResponse{}, nil
	}

	tenantID := effectiveTenantID(ctx, req.TenantId)
	limit := int32(100)
	if req.Limit > 0 && req.Limit < 1000 {
		limit = req.Limit
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, event_type, account_id, symbol, rule_id, reason, by_user, ts_unix_ms, severity
		 FROM risk_events
		 WHERE tenant_id = $1::uuid
		 ORDER BY ts_unix_ms DESC
		 LIMIT $2`,
		tenantID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]*pb.AuditLog, 0)
	for rows.Next() {
		var (
			id        string
			tID       string
			eventType string
			accountID *string
			symbol    *string
			ruleID    *string
			reason    *string
			byUser    *string
			tsMs      int64
			severity  *string
		)
		if err := rows.Scan(&id, &tID, &eventType, &accountID, &symbol, &ruleID, &reason, &byUser, &tsMs, &severity); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		resource := ""
		if accountID != nil {
			resource = "account:" + *accountID
		}
		action := eventType
		if ruleID != nil {
			action = eventType + ":" + *ruleID
		}

		userID := ""
		if byUser != nil {
			userID = *byUser
		}
		logs = append(logs, &pb.AuditLog{
			Id:       id,
			TenantId: tID,
			UserId:   userID,
			Action:   action,
			Resource: resource,
			TsUnixMs: tsMs,
		})
	}
	return &pb.ListAuditLogsResponse{Logs: logs}, rows.Err()
}

// StreamAuditLogs polls risk_events and streams new entries to the client.
func (s *Service) StreamAuditLogs(ctx context.Context, req *pb.StreamAuditLogsRequest, stream *connect.ServerStream[pb.AuditLog]) error {
	if err := RequireTenant(ctx); err != nil {
		return fmt.Errorf("rls: %w", err)
	}
	if s.pool == nil {
		return nil
	}

	tenantID := auth.TenantFromContext(ctx)
	if tenantID == "" && req.TenantId != "" {
		tenantID = req.TenantId
	}
	if tenantID == "" {
		return fmt.Errorf("stream audit logs: no tenant")
	}

	// Start from now; poll every 5 seconds for new events.
	lastMs := time.Now().UnixMilli()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		rows, err := s.pool.Query(ctx,
			`SELECT id, tenant_id, event_type, account_id, symbol, rule_id, reason, by_user, ts_unix_ms, severity
			 FROM risk_events
			 WHERE tenant_id = $1::uuid AND ts_unix_ms > $2
			 ORDER BY ts_unix_ms ASC
			 LIMIT 50`,
			tenantID, lastMs,
		)
		if err != nil {
			continue
		}

		for rows.Next() {
			var (
				id        string
				tID       string
				eventType string
				accountID *string
				symbol    *string
				ruleID    *string
				reason    *string
				byUser    *string
				tsMs      int64
				severity  *string
			)
			if err := rows.Scan(&id, &tID, &eventType, &accountID, &symbol, &ruleID, &reason, &byUser, &tsMs, &severity); err != nil {
				rows.Close()
				return fmt.Errorf("scan audit log: %w", err)
			}

			resource := ""
			if accountID != nil {
				resource = "account:" + *accountID
			}
			action := eventType
			if ruleID != nil {
				action = eventType + ":" + *ruleID
			}

			userID := ""
			if byUser != nil {
				userID = *byUser
			}
			if err := stream.Send(&pb.AuditLog{
				Id:       id,
				TenantId: tID,
				UserId:   userID,
				Action:   action,
				Resource: resource,
				TsUnixMs: tsMs,
			}); err != nil {
				rows.Close()
				return err
			}
			if tsMs > lastMs {
				lastMs = tsMs
			}
		}
		rows.Close()
	}
}


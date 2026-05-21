// Package adminapi provides Connect RPC adapter wrappers.
package adminapi

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/alfq/backend/go/gen/alfq/v1"
)

// Adapter wraps Service to implement Connect handler interfaces.
type Adapter struct{ svc *Service }

// NewAdapter creates a Connect adapter.
func NewAdapter(svc *Service) *Adapter { return &Adapter{svc: svc} }

// -- BrokerService --

func (a *Adapter) CreateBroker(ctx context.Context, req *connect.Request[pb.CreateBrokerRequest]) (*connect.Response[pb.Broker], error) {
	b, err := a.svc.CreateBroker(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(b), nil
}
func (a *Adapter) GetBroker(ctx context.Context, req *connect.Request[pb.GetBrokerRequest]) (*connect.Response[pb.Broker], error) {
	b, err := a.svc.GetBroker(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(b), nil
}
func (a *Adapter) ListBrokers(ctx context.Context, req *connect.Request[pb.ListBrokersRequest]) (*connect.Response[pb.ListBrokersResponse], error) {
	resp, err := a.svc.ListBrokers(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
func (a *Adapter) UpdateBroker(ctx context.Context, req *connect.Request[pb.Broker]) (*connect.Response[pb.Broker], error) {
	b, err := a.svc.UpdateBroker(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(b), nil
}
func (a *Adapter) DeleteBroker(ctx context.Context, req *connect.Request[pb.DeleteBrokerRequest]) (*connect.Response[pb.DeleteBrokerResponse], error) {
	resp, err := a.svc.DeleteBroker(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
func (a *Adapter) SearchBroker(ctx context.Context, req *connect.Request[pb.SearchBrokerRequest]) (*connect.Response[pb.SearchBrokerResponse], error) {
	resp, err := a.svc.SearchBroker(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// -- AccountService --

func (a *Adapter) CreateAccount(ctx context.Context, req *connect.Request[pb.CreateAccountRequest]) (*connect.Response[pb.Account], error) {
	acc, err := a.svc.CreateAccount(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(acc), nil
}
func (a *Adapter) GetAccount(ctx context.Context, req *connect.Request[pb.GetAccountRequest]) (*connect.Response[pb.Account], error) {
	acc, err := a.svc.GetAccount(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(acc), nil
}
func (a *Adapter) ListAccounts(ctx context.Context, req *connect.Request[pb.ListAccountsRequest]) (*connect.Response[pb.ListAccountsResponse], error) {
	resp, err := a.svc.ListAccounts(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
func (a *Adapter) UpdateAccount(ctx context.Context, req *connect.Request[pb.Account]) (*connect.Response[pb.Account], error) {
	acc, err := a.svc.UpdateAccount(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(acc), nil
}
func (a *Adapter) DeleteAccount(ctx context.Context, req *connect.Request[pb.DeleteAccountRequest]) (*connect.Response[pb.DeleteAccountResponse], error) {
	resp, err := a.svc.DeleteAccount(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) ConnectAccount(ctx context.Context, req *connect.Request[pb.ConnectAccountRequest]) (*connect.Response[pb.ConnectAccountResponse], error) {
	resp, err := a.svc.ConnectAccount(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
func (a *Adapter) DisconnectAccount(ctx context.Context, req *connect.Request[pb.DisconnectAccountRequest]) (*connect.Response[pb.DisconnectAccountResponse], error) {
	resp, err := a.svc.DisconnectAccount(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) ListAccountOrders(ctx context.Context, req *connect.Request[pb.ListAccountOrdersRequest]) (*connect.Response[pb.ListAccountOrdersResponse], error) {
	resp, err := a.svc.ListAccountOrders(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) ListAccountPositions(ctx context.Context, req *connect.Request[pb.ListAccountPositionsRequest]) (*connect.Response[pb.ListAccountPositionsResponse], error) {
	resp, err := a.svc.ListAccountPositions(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) SyncAccountHistory(ctx context.Context, req *connect.Request[pb.SyncAccountHistoryRequest]) (*connect.Response[pb.SyncAccountHistoryResponse], error) {
	resp, err := a.svc.SyncAccountHistory(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) GetSyncStatus(ctx context.Context, req *connect.Request[pb.GetSyncStatusRequest]) (*connect.Response[pb.GetSyncStatusResponse], error) {
	resp, err := a.svc.GetSyncStatus(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// -- StrategyService --

func (a *Adapter) CreateStrategy(ctx context.Context, req *connect.Request[pb.CreateStrategyRequest]) (*connect.Response[pb.Strategy], error) {
	st, err := a.svc.CreateStrategy(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(st), nil
}
func (a *Adapter) GetStrategy(ctx context.Context, req *connect.Request[pb.GetStrategyRequest]) (*connect.Response[pb.Strategy], error) {
	st, err := a.svc.GetStrategy(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(st), nil
}
func (a *Adapter) ListStrategies(ctx context.Context, req *connect.Request[pb.ListStrategiesRequest]) (*connect.Response[pb.ListStrategiesResponse], error) {
	resp, err := a.svc.ListStrategies(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
func (a *Adapter) DeployStrategy(ctx context.Context, req *connect.Request[pb.DeployStrategyRequest]) (*connect.Response[pb.Strategy], error) {
	st, err := a.svc.DeployStrategy(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(st), nil
}
func (a *Adapter) StopStrategy(ctx context.Context, req *connect.Request[pb.StopStrategyRequest]) (*connect.Response[pb.Strategy], error) {
	st, err := a.svc.StopStrategy(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(st), nil
}
func (a *Adapter) StreamSignals(ctx context.Context, req *connect.Request[pb.StreamSignalsRequest], stream *connect.ServerStream[pb.Signal]) error {
	// Placeholder: no real signal stream yet.
	return nil
}

// -- BacktestService --
// RunBacktest and ListBacktests are now implemented in backtest_handler.go.

// -- SystemSettingsService --

func (a *Adapter) GetSystemSettings(ctx context.Context, req *connect.Request[pb.GetSystemSettingsRequest]) (*connect.Response[pb.GetSystemSettingsResponse], error) {
	resp, err := a.svc.GetSystemSettings(ctx, req.Msg)
	if err != nil { return nil, err }
	return connect.NewResponse(resp), nil
}
func (a *Adapter) UpdateSystemSetting(ctx context.Context, req *connect.Request[pb.UpdateSystemSettingRequest]) (*connect.Response[pb.UpdateSystemSettingResponse], error) {
	resp, err := a.svc.UpdateSystemSetting(ctx, req.Msg)
	if err != nil { return nil, err }
	return connect.NewResponse(resp), nil
}

// -- ServiceManagementService --

func (a *Adapter) GetServiceStatus(ctx context.Context, req *connect.Request[pb.GetServiceStatusRequest]) (*connect.Response[pb.GetServiceStatusResponse], error) {
	resp, err := a.svc.GetServiceStatus(ctx, req.Msg)
	if err != nil { return nil, err }
	return connect.NewResponse(resp), nil
}
func (a *Adapter) RestartService(ctx context.Context, req *connect.Request[pb.RestartServiceRequest]) (*connect.Response[pb.RestartServiceResponse], error) {
	resp, err := a.svc.RestartService(ctx, req.Msg)
	if err != nil { return nil, err }
	return connect.NewResponse(resp), nil
}
func (a *Adapter) GetServiceLogs(ctx context.Context, req *connect.Request[pb.GetServiceLogsRequest]) (*connect.Response[pb.GetServiceLogsResponse], error) {
	resp, err := a.svc.GetServiceLogs(ctx, req.Msg)
	if err != nil { return nil, err }
	return connect.NewResponse(resp), nil
}

// -- AuditService --

func (a *Adapter) ListAuditLogs(ctx context.Context, req *connect.Request[pb.ListAuditLogsRequest]) (*connect.Response[pb.ListAuditLogsResponse], error) {
	resp, err := a.svc.ListAuditLogs(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Adapter) StreamAuditLogs(ctx context.Context, req *connect.Request[pb.StreamAuditLogsRequest], stream *connect.ServerStream[pb.AuditLog]) error {
	return nil // stub
}

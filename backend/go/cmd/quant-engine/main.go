// quant-engine merges factor-svc and strategy-svc into a single process.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alfq/backend/go/internal/common/bootstrap"
	"github.com/alfq/backend/go/internal/quantengine"
	"go.uber.org/zap"
)

func main() {
	if err := bootstrap.Run("quant-engine", register,
		bootstrap.WithoutRedis(),
	); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		os.Exit(1)
	}
}

func register(adapter *bootstrap.ServeMuxAdapter, d *bootstrap.Deps) error {
	// Wire signal→order bridge: on each signal, insert an order into PG.
	onSignal := func(symbol string, side string, qty float64, reason string) {
		if d.PG == nil {
			d.Log.Warn("signal→order: no PG, skipping")
			return
		}

		sideInt := int32(1)
		mtSide := "buy"
		if side == "short" {
			sideInt = 2
			mtSide = "sell"
		}

		accountID := "51b8fe22-1561-4027-802d-32af80d17f6d" // MT5 demo
		nowMs := time.Now().UTC().UnixMilli()
		stateSubmitted := int32(4)

		// Resolve canonical → broker-specific symbol_raw via accounts → broker_symbols.
		// Falls back to canonical if no mapping (e.g., bar already broker-native or unknown symbol).
		brokerSymbol := symbol
		var resolved string
		if err := d.PG.QueryRow(context.Background(),
			`SELECT bs.symbol_raw
			   FROM accounts a
			   JOIN broker_symbols bs ON bs.broker_id = a.broker_id
			  WHERE a.id = $1
			    AND (bs.canonical = $2 OR bs.symbol_raw = $2)
			  LIMIT 1`,
			accountID, symbol,
		).Scan(&resolved); err == nil && resolved != "" {
			brokerSymbol = resolved
		} else if err != nil {
			d.Log.Debug("signal→order: symbol resolve fallback",
				zap.String("canonical", symbol), zap.Error(err))
		}

		_, err := d.PG.Exec(context.Background(),
			`INSERT INTO orders (tenant_id, account_id, strategy_id, client_order_id, symbol, side, type, qty, price, state, created_ts_ms, updated_ts_ms)
			 VALUES ($1, $2, $3, $4, $5, $6, 0, $7, 0, $8, $9, $9)
			 ON CONFLICT (account_id, client_order_id) DO NOTHING`,
			"00000000-0000-0000-0000-000000000001",
			accountID,
			"70846f7a-9873-492f-82f7-7ac48d26551e",
			fmt.Sprintf("qe-%s-%d", brokerSymbol, nowMs),
			brokerSymbol,
			sideInt,
			qty,
			stateSubmitted,
			nowMs,
		)
		if err != nil {
			d.Log.Warn("signal→order: insert failed", zap.Error(err), zap.String("symbol", brokerSymbol))
		} else {
			d.Log.Info("signal→order: created",
				zap.String("canonical", symbol),
				zap.String("broker_symbol", brokerSymbol),
				zap.String("side", mtSide), zap.Float64("qty", qty))
		}
	}

	// Start order submitter: polls for SUBMITTED orders without broker_ticket and submits to MT.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				d.Log.Error("order submitter panicked", zap.Any("panic", r))
			}
		}()
		runOrderSubmitter(d)
	}()

	return quantengine.RunQuantEngineWithSignalHandler(adapter.Mux, d, onSignal)
}

// runOrderSubmitter polls PG for unsubmitted orders and sends them to mthub via Connect RPC.
func runOrderSubmitter(d *bootstrap.Deps) {
	d.Log.Info("order submitter: starting")
	if d.PG == nil {
		d.Log.Warn("order submitter: PG is nil, exiting")
		return
	}
	mthubURL := os.Getenv("MTHUB_ADDR")
	if mthubURL == "" {
		mthubURL = "http://md-gateway:9001"
	}
	orderSendURL := mthubURL + "/alfq.mthub.v1.MtHubService/OrderSend"

	httpClient := &http.Client{Timeout: 15 * time.Second}

	// Poll every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	d.Log.Info("order submitter: entering poll loop")

	for range ticker.C {
		d.Log.Debug("order submitter: polling for unsubmitted orders")

		// Acquire dedicated connection to bypass RLS (set_config must stay on same conn)
		ctx := context.Background()
		conn, err := d.PG.Acquire(ctx)
		if err != nil {
			d.Log.Warn("order submitter: acquire conn failed", zap.Error(err))
			continue
		}

		if _, err := conn.Exec(ctx, "SELECT set_config('app.tenant_id', '00000000-0000-0000-0000-000000000001', true)"); err != nil {
			d.Log.Warn("order submitter: set tenant_id failed", zap.Error(err))
			conn.Release()
			continue
		}

		rows, err := conn.Query(ctx,
			`SELECT order_id, account_id, symbol, side, qty
			 FROM orders
			 WHERE state = 4 AND broker_ticket IS NULL
			 ORDER BY created_ts_ms
			 LIMIT 5`)
		if err != nil {
			d.Log.Warn("order submitter: query failed", zap.Error(err))
			continue
		}

		type pendingOrder struct {
			orderID, accountID, symbol string
			side                       int32
			qty                        float64
		}
		var pending []pendingOrder
		for rows.Next() {
			var po pendingOrder
			var qtyFloat float64
			if err := rows.Scan(&po.orderID, &po.accountID, &po.symbol, &po.side, &qtyFloat); err != nil {
				continue
			}
			po.qty = qtyFloat
			pending = append(pending, po)
		}
		rows.Close()
		conn.Release()

		if len(pending) > 0 {
			d.Log.Info("order submitter: found pending orders", zap.Int("count", len(pending)))
		}

		for _, po := range pending {
			po := po // capture
			go func() {
				mtSide := "buy"
				if po.side == 2 {
					mtSide = "sell"
				}

				reqBody := map[string]interface{}{
					"account_id": po.accountID,
					"symbol":     po.symbol,
					"side":       mtSide,
					"lots":       po.qty,
					"comment":    "qe-auto",
				}
				bodyBytes, _ := json.Marshal(reqBody)

				resp, err := httpClient.Post(orderSendURL, "application/json", bytes.NewReader(bodyBytes))
				if err != nil {
					d.Log.Warn("order submitter: mthub call failed", zap.Error(err), zap.String("order_id", po.orderID))
					return
				}

				respBody, _ := io.ReadAll(resp.Body)
				resp.Body.Close()

				if resp.StatusCode != 200 {
					d.Log.Warn("order submitter: mthub non-200",
						zap.Int("status", resp.StatusCode),
						zap.String("body", string(respBody)),
					)
					return
				}

				// ConnectRPC/protojson encodes int64 as a JSON string per proto3 spec,
				// so we decode into string and parse manually.
				var mthubResp struct {
					Ticket json.Number `json:"ticket"`
					Error  string      `json:"error"`
				}
				if err := json.Unmarshal(respBody, &mthubResp); err != nil {
					d.Log.Warn("order submitter: parse mthub response failed",
						zap.Error(err), zap.String("body", string(respBody)))
					return
				}
				ticket, _ := mthubResp.Ticket.Int64()

				if ticket > 0 {
					// Write needs tenant context (RLS); acquire dedicated conn.
					upCtx := context.Background()
					upConn, err := d.PG.Acquire(upCtx)
					if err != nil {
						d.Log.Warn("order submitter: acquire conn for update failed", zap.Error(err))
						return
					}
					defer upConn.Release()
					if _, err := upConn.Exec(upCtx,
						"SELECT set_config('app.tenant_id', '00000000-0000-0000-0000-000000000001', true)"); err != nil {
						d.Log.Warn("order submitter: set tenant for update failed", zap.Error(err))
						return
					}
					_, err = upConn.Exec(upCtx,
						`UPDATE orders SET broker_ticket = $1, updated_ts_ms = $2 WHERE order_id = $3`,
						fmt.Sprintf("%d", ticket),
						time.Now().UTC().UnixMilli(),
						po.orderID,
					)
					if err != nil {
						d.Log.Warn("order submitter: ticket update failed", zap.Error(err))
					} else {
						d.Log.Info("order submitter: ticket assigned",
							zap.String("order_id", po.orderID),
							zap.Int64("ticket", ticket),
						)
					}
				} else if mthubResp.Error != "" {
					d.Log.Warn("order submitter: mthub error",
						zap.String("order_id", po.orderID),
						zap.String("error", mthubResp.Error),
					)
				}
			}()
		}
	}
}

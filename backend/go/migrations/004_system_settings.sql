-- 004: System settings for gateway configuration
CREATE TABLE IF NOT EXISTS system_settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    description TEXT DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Default gateway addresses
INSERT INTO system_settings (key, value, description) VALUES
    ('mt4_gateway_addr', 'mt4grpc3.mtapi.io:443', 'MT4 gRPC 网关地址'),
    ('mt4_gateway_tls', 'true', 'MT4 网关是否启用 TLS'),
    ('mt5_gateway_addr', 'mt5grpc3.mtapi.io:443', 'MT5 gRPC 网关地址'),
    ('mt5_gateway_tls', 'true', 'MT5 网关是否启用 TLS')
ON CONFLICT (key) DO NOTHING;

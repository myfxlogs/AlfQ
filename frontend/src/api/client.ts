import { createConnectTransport } from "@connectrpc/connect-web";
import { createClient, type Interceptor } from "@connectrpc/connect";
import { AuthService, AuditService, TenantService, UserService } from "../gen/alfq/v1/auth_pb";
import { BrokerService, AccountService } from "../gen/alfq/v1/broker_pb";
import { StrategyService, BacktestService } from "../gen/alfq/v1/strategy_pb";

interface ImportMetaEnv {
  VITE_API_BASE_URL?: string;
}

function env(key: string, fallback: string): string {
  return (import.meta as { env?: ImportMetaEnv }).env?.[
    key as keyof ImportMetaEnv
  ] ?? fallback;
}

// Auth interceptor: attach Bearer token to every outgoing request
const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getToken();
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  return next(req);
};

const transport = createConnectTransport({
  baseUrl: env("VITE_API_BASE_URL", "/api"),
  interceptors: [authInterceptor],
});

export const authClient = createClient(AuthService, transport);
export const brokerClient = createClient(BrokerService, transport);
export const accountClient = createClient(AccountService, transport);
export const strategyClient = createClient(StrategyService, transport);
export const auditClient = createClient(AuditService, transport);
export const backtestClient = createClient(BacktestService, transport);
export const tenantClient = createClient(TenantService, transport);
export const userClient = createClient(UserService, transport);

// Helper to get auth token from localStorage
export function getToken(): string | null {
  return localStorage.getItem("alfq_token");
}

export function setToken(token: string): void {
  localStorage.setItem("alfq_token", token);
}

export function clearToken(): void {
  localStorage.removeItem("alfq_token");
}

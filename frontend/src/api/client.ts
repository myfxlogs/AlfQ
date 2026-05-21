import { createConnectTransport } from "@connectrpc/connect-web";
import { createClient, ConnectError, Code, type Interceptor } from "@connectrpc/connect";
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

// Auth interceptor: attach Bearer token to every outgoing request.
const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getToken();
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  return next(req);
};

// Session-expiry interceptor: globally handle auth or server-side failures by
// clearing the local token and redirecting to the login page. This avoids
// every page having to duplicate the same error-handling logic.
// We deliberately ignore Auth* endpoints so the login screen can display real
// validation errors instead of bouncing to itself.
const sessionExpiryInterceptor: Interceptor = (next) => async (req) => {
  try {
    return await next(req);
  } catch (e) {
    if (e instanceof ConnectError) {
      const isAuthRpc = req.service.typeName.endsWith(".AuthService");
      const shouldRedirect =
        e.code === Code.Unauthenticated ||
        e.code === Code.PermissionDenied ||
        e.code === Code.Internal ||
        e.code === Code.Unavailable;
      if (shouldRedirect && !isAuthRpc) {
        clearToken();
        if (!window.location.hash.startsWith("#/login")) {
          window.location.hash = "#/login";
        }
      }
    }
    throw e;
  }
};

const transport = createConnectTransport({
  baseUrl: env("VITE_API_BASE_URL", "/api"),
  interceptors: [authInterceptor, sessionExpiryInterceptor],
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

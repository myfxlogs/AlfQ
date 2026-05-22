import { createConnectTransport } from "@connectrpc/connect-web";
import { createClient, ConnectError, Code, type Interceptor } from "@connectrpc/connect";
import { AuthService, AuditService, TenantService, UserService } from "../gen/alfq/v1/auth_pb";
import { BrokerService, AccountService, SystemSettingsService } from "../gen/alfq/v1/broker_pb";
import { StrategyService, BacktestService } from "../gen/alfq/v1/strategy_pb";

interface ImportMetaEnv {
  VITE_API_BASE_URL?: string;
}

function env(key: string, fallback: string): string {
  return (import.meta as { env?: ImportMetaEnv }).env?.[
    key as keyof ImportMetaEnv
  ] ?? fallback;
}

// ── Token helpers ──

export function getToken(): string | null {
  return localStorage.getItem("alfq_token");
}

export function setToken(token: string): void {
  localStorage.setItem("alfq_token", token);
}

export function clearToken(): void {
  localStorage.removeItem("alfq_token");
  localStorage.removeItem("alfq_refresh");
  localStorage.removeItem("alfq_expires");
}

function getRefreshToken(): string | null {
  return localStorage.getItem("alfq_refresh");
}

function getExpiresAt(): number {
  return Number(localStorage.getItem("alfq_expires") || "0");
}

export function saveAuth(accessToken: string, refreshToken: string, expiresIn: number): void {
  setToken(accessToken);
  localStorage.setItem("alfq_refresh", refreshToken);
  localStorage.setItem("alfq_expires", String(Date.now() + expiresIn * 1000));
}

// ── Auto-refresh ──
// Refresh 5 minutes before expiry (only if ≥ 10 min remaining after login).

let refreshTimer: ReturnType<typeof setTimeout> | null = null;

function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer);
  const expiresAt = getExpiresAt();
  if (!expiresAt) return;
  const delay = expiresAt - Date.now() - 5 * 60 * 1000; // 5 min before expiry
  if (delay <= 0) return; // too late, let the interceptor handle it
  refreshTimer = setTimeout(doRefresh, delay);
}

async function doRefresh() {
  const rt = getRefreshToken();
  if (!rt) return;
  try {
    const transport = createConnectTransport({ baseUrl: env("VITE_API_BASE_URL", "/api") });
    const authClient = createClient(AuthService, transport);
    const res = await authClient.refreshToken({ refreshToken: rt });
    saveAuth(res.accessToken, res.refreshToken, Number(res.expiresIn));
    scheduleRefresh();
  } catch {
    clearToken();
    if (!window.location.hash.startsWith("#/login")) {
      window.location.hash = "#/login";
    }
  }
}

scheduleRefresh(); // kick off on page load

// ── Interceptors ──

// Auth interceptor: attach Bearer token to every outgoing request.
const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getToken();
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  return next(req);
};

// Session-expiry interceptor: globally handle auth or server-side failures.
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
export const settingsClient = createClient(SystemSettingsService, transport);

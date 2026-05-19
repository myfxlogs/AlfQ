import { createConnectTransport } from "@connectrpc/connect-web";
import { createClient } from "@connectrpc/connect";
import { AuthService } from "../gen/alfq/v1/auth_pb";
import { BrokerService, AccountService } from "../gen/alfq/v1/broker_pb";
import { StrategyService } from "../gen/alfq/v1/strategy_pb";

interface ImportMetaEnv {
  VITE_API_BASE_URL?: string;
  VITE_REST_BASE_URL?: string;
}

function env(key: string, fallback: string): string {
  return (import.meta as { env?: ImportMetaEnv }).env?.[
    key as keyof ImportMetaEnv
  ] ?? fallback;
}

const transport = createConnectTransport({
  baseUrl: env("VITE_API_BASE_URL", "/api"),
});

export const authClient = createClient(AuthService, transport);
export const brokerClient = createClient(BrokerService, transport);
export const accountClient = createClient(AccountService, transport);
export const strategyClient = createClient(StrategyService, transport);

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

// Simple REST fetch helper for services without connect handlers (e.g. trading-core internal components)
export async function apiFetch<T = unknown>(
  path: string,
  opts?: RequestInit
): Promise<T> {
  const base = env("VITE_REST_BASE_URL", "/api");
  const res = await fetch(`${base}${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...(getToken() ? { Authorization: `Bearer ${getToken()}` } : {}),
      ...(opts?.headers || {}),
    },
    ...opts,
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${await res.text()}`);
  return res.json() as Promise<T>;
}

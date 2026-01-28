// src/api/backend.ts

import { getBackendSrv } from '@grafana/runtime';
import { API_ENDPOINTS, getBackendCandidates } from '../constants';

let cachedBaseUrl: string | null = null;

type GrafanaUserCtx = {
  login: string;
  email: string;
  orgId: number;
};

async function resolveBackendBaseUrl(): Promise<string> {
  if (cachedBaseUrl) {
    return cachedBaseUrl;
  }

  const candidates = getBackendCandidates();

  for (const base of candidates) {
    try {
      const url = `${base}${API_ENDPOINTS.HEALTH}`;
      const res = await fetch(url, { method: 'GET' });
      if (res.ok) {
        cachedBaseUrl = base;
        return base;
      }
    } catch {
      // next
    }
  }

  throw new Error(`Backend não encontrado. Testei: ${candidates.join(', ')}`);
}

export async function getGrafanaUserCtx(): Promise<GrafanaUserCtx> {
  try {
    const u = await getBackendSrv().get('/api/user');
    return {
      login: (u?.login || '').toString(),
      email: (u?.email || '').toString(),
      orgId: Number(u?.orgId || 1),
    };
  } catch {
    return { login: '', email: '', orgId: 1 };
  }
}

export async function apiGet<T>(path: string): Promise<T> {
  const base = await resolveBackendBaseUrl();
  const clean = path.startsWith('/') ? path : `/${path}`;
  const url = `${base}${clean}`;

  const ctx = await getGrafanaUserCtx();

  const res = await fetch(url, {
    method: 'GET',
    headers: {
      'X-Grafana-Org-Id': String(ctx.orgId || 1),
    },
  });

  const text = await res.text();
  if (!res.ok) {
    throw new Error(`GET ${url} -> ${res.status}: ${text}`);
  }
  return JSON.parse(text) as T;
}

export async function apiPost<T>(path: string, body: any): Promise<T> {
  const base = await resolveBackendBaseUrl();
  const clean = path.startsWith('/') ? path : `/${path}`;
  const url = `${base}${clean}`;

  const ctx = await getGrafanaUserCtx();

  const res = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Grafana-Org-Id': String(ctx.orgId || 1),
    },
    body: JSON.stringify(body),
  });

  const text = await res.text();
  if (!res.ok) {
    throw new Error(`POST ${url} -> ${res.status}: ${text}`);
  }
  return text ? (JSON.parse(text) as T) : ({} as T);
}

// Mantém compatível com teu ImportPage
export async function getGrafanaLoggedUser(): Promise<string> {
  const ctx = await getGrafanaUserCtx();
  return (ctx.login || ctx.email || '').toString();
}

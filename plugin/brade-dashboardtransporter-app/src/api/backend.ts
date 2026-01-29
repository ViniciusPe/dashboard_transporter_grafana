// src/api/backend.ts

import { getBackendSrv } from '@grafana/runtime';
import { API_ENDPOINTS, getBackendCandidates } from '../constants';

let cachedBaseUrl: string | null = null;

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

function stringifyErrorBody(text: string): string {
  // tenta JSON -> pega message
  try {
    const j = JSON.parse(text);
    if (j?.message) return String(j.message);
    return JSON.stringify(j);
  } catch {
    return text;
  }
}

export async function apiGet<T>(path: string): Promise<T> {
  const base = await resolveBackendBaseUrl();
  const clean = path.startsWith('/') ? path : `/${path}`;
  const url = `${base}${clean}`;

  const res = await fetch(url, { method: 'GET' });
  const text = await res.text();

  if (!res.ok) {
    const body = stringifyErrorBody(text);
    throw new Error(`GET ${url} -> ${res.status}: ${body}`);
  }

  return JSON.parse(text) as T;
}

export async function apiPost<T>(path: string, body: any): Promise<T> {
  const base = await resolveBackendBaseUrl();
  const clean = path.startsWith('/') ? path : `/${path}`;
  const url = `${base}${clean}`;

  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });

  const text = await res.text();

  if (!res.ok) {
    const bodyText = stringifyErrorBody(text);
    throw new Error(`POST ${url} -> ${res.status}: ${bodyText}`);
  }

  return text ? (JSON.parse(text) as T) : ({} as T);
}

// pega user logado (Viewer também)
export async function getGrafanaLoggedUser(): Promise<string> {
  try {
    const u = await getBackendSrv().get('/api/user');
    return (u?.login || u?.email || '').toString();
  } catch {
    return '';
  }
}

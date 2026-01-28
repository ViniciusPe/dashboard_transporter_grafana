// src/utils/backend_srv.ts

import { getBackendSrv, config } from '@grafana/runtime';
import type { FetchResponse } from '@grafana/runtime';
import { lastValueFrom } from 'rxjs';
import { PLUGIN_ID as FALLBACK_PLUGIN_ID } from '../constants';

function resolvePluginId(): string {
  // Grafana injeta settings do plugin aqui
  // (na maioria das versões 8+ isso existe)
  const s: any = (config as any)?.bootData?.settings;
  if (s && s[FALLBACK_PLUGIN_ID]) {
    return FALLBACK_PLUGIN_ID;
  }

  // Se por algum motivo o id do plugin mudou, tenta achar pelo "name" do include
  // (fallback defensivo)
  if (s) {
    const keys = Object.keys(s);
    const found = keys.find((k) => (s[k]?.type === 'app' && s[k]?.name === 'Dashboard Transporter') || s[k]?.id === FALLBACK_PLUGIN_ID);
    if (found) {
      return found;
    }
  }

  return FALLBACK_PLUGIN_ID;
}

function pluginProxyUrl(path: string) {
  const pluginId = resolvePluginId();
  const clean = path.startsWith('/') ? path.slice(1) : path;
  // route path no plugin.json: "api"
  return `/api/plugin-proxy/${pluginId}/api/${clean}`;
}

export async function apiGet<T>(path: string): Promise<T> {
  const obs = getBackendSrv().fetch<T>({
    url: pluginProxyUrl(path),
    method: 'GET',
  });

  const res: FetchResponse<T> = await lastValueFrom(obs);
  return res.data as T;
}

export async function apiPost<T>(path: string, body: any): Promise<T> {
  const obs = getBackendSrv().fetch<T>({
    url: pluginProxyUrl(path),
    method: 'POST',
    data: body,
    headers: { 'Content-Type': 'application/json' },
  });

  const res: FetchResponse<T> = await lastValueFrom(obs);
  return res.data as T;
}

// src/constants.ts

export const PLUGIN_ID = 'brade-dashboardtransporter-app';

// Endpoints do teu backend Go
export const API_ENDPOINTS = {
  HEALTH: '/health',
  ENVIRONMENTS: '/environments',
  DASHBOARDS: '/dashboards',
  FOLDERS: '/folders',
  IMPORT_BATCH: '/dashboards/import/batch',
} as const;

/**
 * Base URL do backend (tentativas).
 *
 * Ordem:
 * 1) window.__DASHBOARD_TRANSPORTER_BACKEND__  (você pode setar no ARO sem rebuild)
 * 2) /dashboard-transporter                   (se você publicar backend no mesmo host via ingress/rewrite)
 * 3) http://localhost:8080                    (dev docker)
 */
export function getBackendCandidates(): string[] {
  const w = window as any;
  const fromWindow = (w?.__DASHBOARD_TRANSPORTER_BACKEND__ || '').toString().trim();

  const candidates = [
    fromWindow,
    '/dashboard-transporter',
    'http://localhost:8080',
  ].filter(Boolean);

  // normaliza sem barra no final
  return candidates.map((c) => c.replace(/\/+$/, ''));
}

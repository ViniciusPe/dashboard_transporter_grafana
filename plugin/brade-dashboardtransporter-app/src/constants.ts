// src/constants.ts

export const PLUGIN_ID = 'brade-dashboardtransporter-app';

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
 * 1) window.__DASHBOARD_TRANSPORTER_BACKEND__
 * 2) /dashboard-transporter
 * 3) http://localhost:8080
 */
export function getBackendCandidates(): string[] {
  const w = window as any;
  const fromWindow = (w?.__DASHBOARD_TRANSPORTER_BACKEND__ || '').toString().trim();

  const candidates = [
    fromWindow,
    '/dashboard-transporter',
    'http://localhost:8080',
  ].filter(Boolean);

  return candidates.map((c) => c.replace(/\/+$/, ''));
}

// src/utils/utils_routing.ts

import { PLUGIN_ID } from '../constants';

// Prefixa rotas internas do plugin (grafana app path)
export function prefixRoute(route: string): string {
  const clean = route.startsWith('/') ? route.slice(1) : route;
  return `/a/${PLUGIN_ID}/${clean}`;
}

// src/pages/ImportPage.tsx

import React, { useEffect, useMemo, useState } from 'react';
import { InlineField, InlineFieldRow, Select, Button, Spinner, Input, Alert } from '@grafana/ui';
import { API_ENDPOINTS } from '../constants';
import { apiGet, apiPost, getGrafanaLoggedUser } from '../api/backend';

type Environment = { id: string; name: string; url: string };
type Dashboard = { id: number; uid: string; title: string };
type Folder = { uid: string; title: string };

type ImportBatchPayload = {
  sourceEnv: string;
  targetEnv: string;
  folderUid: string;
  uids: string[];
  requestedBy?: string; // backend recebe string; vamos mandar normalizado (csv)
};

type ImportBatchResult = {
  sourceUid: string;
  targetUid?: string;
  status: 'ok' | 'warning' | 'error';
  message?: string;
};

function normalizeRequestedBy(input: string, fallbackSingleUser: string): { csv: string; users: string[] } {
  const raw = (input || '').trim();

  // se vazio, usa user logado (comportamento atual)
  const base = raw.length ? raw : (fallbackSingleUser || '').trim();
  if (!base) return { csv: '', users: [] };

  // aceita separadores: vírgula, ponto-e-vírgula, newline, tab
  const parts = base
    .split(/[,\n;\t]+/g)
    .map((s) => s.trim())
    .filter(Boolean);

  // unique (mantém ordem)
  const seen = new Set<string>();
  const users: string[] = [];
  for (const p of parts) {
    const k = p.toLowerCase();
    if (!seen.has(k)) {
      seen.add(k);
      users.push(p);
    }
  }

  return { csv: users.join(', '), users };
}

export const ImportPage = () => {
  const [loading, setLoading] = useState(false);

  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [sourceEnv, setSourceEnv] = useState<Environment | null>(null);
  const [targetEnv, setTargetEnv] = useState<Environment | null>(null);

  const [folders, setFolders] = useState<Folder[]>([]);
  const [selectedFolder, setSelectedFolder] = useState<Folder | null>(null);

  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [selectedUids, setSelectedUids] = useState<Set<string>>(new Set());

  const [loggedUser, setLoggedUser] = useState<string>('');
  const [requestedBy, setRequestedBy] = useState<string>('');

  const [msg, setMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // DEBUG: mostra o payload e a resposta do backend na tela
  const [debugOpen, setDebugOpen] = useState<boolean>(true);
  const [lastPayload, setLastPayload] = useState<any>(null);
  const [lastResponse, setLastResponse] = useState<any>(null);

  const envOptions = useMemo(
    () => environments.map((e) => ({ label: `${e.name} (${e.id})`, value: e.id, env: e })),
    [environments]
  );

  const folderOptions = useMemo(
    () => folders.map((f) => ({ label: f.title || 'General', value: f.uid, folder: f })),
    [folders]
  );

  function toggleUid(uid: string) {
    setSelectedUids((prev) => {
      const next = new Set(prev);
      next.has(uid) ? next.delete(uid) : next.add(uid);
      return next;
    });
  }

  function selectAll() {
    setSelectedUids(new Set(dashboards.map((d) => d.uid)));
  }

  async function loadEnvironments() {
    return apiGet<Environment[]>(API_ENDPOINTS.ENVIRONMENTS);
  }

  async function loadDashboards(envId: string) {
    return apiGet<Dashboard[]>(`${API_ENDPOINTS.DASHBOARDS}?env=${encodeURIComponent(envId)}`);
  }

  async function loadFolders(envId: string) {
    return apiGet<Folder[]>(`${API_ENDPOINTS.FOLDERS}?env=${encodeURIComponent(envId)}`);
  }

  // init
  useEffect(() => {
    (async () => {
      setLoading(true);
      setMsg(null);
      setLastPayload(null);
      setLastResponse(null);

      try {
        const u = await getGrafanaLoggedUser();
        setLoggedUser(u);

        const envs = await loadEnvironments();
        setEnvironments(envs);

        // defaults: dev/hml se existirem
        const dev = envs.find((e) => e.id === 'dev') || envs[0] || null;
        const hml = envs.find((e) => e.id === 'hml') || (envs.length > 1 ? envs[1] : null);

        setSourceEnv(dev);
        setTargetEnv(hml);

        // IMPORTANTE: não “pré-carrega” dashboards/pastas fora do ambiente selecionado
        // (vai cair nos useEffect abaixo)
      } catch (e: any) {
        setMsg({ type: 'error', text: e?.message || 'Erro ao carregar dados iniciais' });
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  // quando muda ambiente de ORIGEM, carrega dashboards daquele env
  useEffect(() => {
    if (!sourceEnv?.id) return;
    (async () => {
      setLoading(true);
      setMsg(null);
      setDashboards([]);
      setSelectedUids(new Set());
      try {
        const dbs = await loadDashboards(sourceEnv.id);
        setDashboards(dbs);
      } catch (e: any) {
        setMsg({ type: 'error', text: e?.message || `Erro ao carregar dashboards do ambiente ${sourceEnv.id}` });
      } finally {
        setLoading(false);
      }
    })();
  }, [sourceEnv?.id]);

  // quando muda ambiente de DESTINO, carrega pastas daquele env
  useEffect(() => {
    if (!targetEnv?.id) return;
    (async () => {
      setLoading(true);
      setMsg(null);
      setFolders([]);
      setSelectedFolder(null);
      try {
        const fs = await loadFolders(targetEnv.id);
        setFolders(fs);
        setSelectedFolder(fs[0] || { uid: '', title: 'General' });
      } catch (e: any) {
        setMsg({ type: 'error', text: e?.message || `Erro ao carregar pastas do ambiente ${targetEnv.id}` });
      } finally {
        setLoading(false);
      }
    })();
  }, [targetEnv?.id]);

  async function doImport() {
    setMsg(null);
    setLastPayload(null);
    setLastResponse(null);

    if (!sourceEnv || !targetEnv) {
      setMsg({ type: 'error', text: 'Selecione ambiente de origem e destino.' });
      return;
    }
    if (!selectedFolder) {
      setMsg({ type: 'error', text: 'Selecione a pasta de destino.' });
      return;
    }

    const uids = Array.from(selectedUids);
    if (!uids.length) {
      setMsg({ type: 'error', text: 'Selecione pelo menos 1 dashboard.' });
      return;
    }

    const norm = normalizeRequestedBy(requestedBy, loggedUser);
    if (!norm.users.length) {
      setMsg({ type: 'error', text: 'requestedBy vazio e não foi possível detectar usuário logado.' });
      return;
    }

    setLoading(true);
    try {
      const payload: ImportBatchPayload = {
        sourceEnv: sourceEnv.id,
        targetEnv: targetEnv.id,
        folderUid: selectedFolder.uid ?? '',
        uids,
        requestedBy: norm.csv, // manda como CSV normalizado
      };

      setLastPayload(payload);

      const res = await apiPost<ImportBatchResult[]>(API_ENDPOINTS.IMPORT_BATCH, payload);
      setLastResponse(res);

      // monta mensagem rica (pra não ficar “ok” e você às cegas)
      const summary = Array.isArray(res)
        ? res
            .map((r) => {
              const m = r.message ? ` - ${r.message}` : '';
              return `${r.sourceUid} -> ${r.targetUid || r.sourceUid}: ${r.status}${m}`;
            })
            .join('\n')
        : JSON.stringify(res);

      const hasError = Array.isArray(res) && res.some((r) => r.status === 'error');
      const hasWarn = Array.isArray(res) && res.some((r) => r.status === 'warning');

      setMsg({
        type: hasError ? 'error' : 'success',
        text:
          (hasWarn ? 'Importação concluída com avisos.\n' : 'Importação concluída.\n') +
          `RBAC para: ${norm.users.join(', ')}\n\n` +
          summary,
      });
    } catch (e: any) {
      setMsg({ type: 'error', text: e?.message || 'Falha no import' });
    } finally {
      setLoading(false);
    }
  }

  const previewUsers = useMemo(() => normalizeRequestedBy(requestedBy, loggedUser).users, [requestedBy, loggedUser]);

  return (
    <div style={{ padding: 16, maxWidth: 1100 }}>
      {msg && (
        <Alert severity={msg.type} title={msg.type === 'success' ? 'Sucesso' : 'Erro'}>
          <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{msg.text}</pre>
        </Alert>
      )}

      <h2>Dashboards Transporter</h2>

      <InlineFieldRow style={{ marginBottom: 16 }}>
        <InlineField label="Ambiente de Origem" labelWidth={22}>
          <Select
            width={28}
            options={envOptions}
            value={sourceEnv ? { label: `${sourceEnv.name} (${sourceEnv.id})`, value: sourceEnv.id } : undefined}
            onChange={(v) => setSourceEnv((v as any)?.env ?? null)}
          />
        </InlineField>

        <InlineField label="Ambiente de Destino" labelWidth={22}>
          <Select
            width={28}
            options={envOptions}
            value={targetEnv ? { label: `${targetEnv.name} (${targetEnv.id})`, value: targetEnv.id } : undefined}
            onChange={(v) => setTargetEnv((v as any)?.env ?? null)}
          />
        </InlineField>

        <InlineField label="Pasta no Destino" labelWidth={22}>
          <Select
            width={28}
            options={folderOptions}
            value={selectedFolder ? { label: selectedFolder.title || 'General', value: selectedFolder.uid } : undefined}
            onChange={(v) => setSelectedFolder((v as any)?.folder ?? null)}
          />
        </InlineField>
      </InlineFieldRow>

      <InlineFieldRow style={{ marginBottom: 12 }}>
        <InlineField label="Usuário logado" labelWidth={22}>
          <Input value={loggedUser || 'Não identificado'} readOnly width={60} />
        </InlineField>
      </InlineFieldRow>

      <InlineFieldRow style={{ marginBottom: 8 }}>
        <InlineField label="requestedBy (múltiplos)" labelWidth={22}>
          <Input
            value={requestedBy}
            onChange={(e) => setRequestedBy(e.currentTarget.value)}
            placeholder="Ex: user1@..., user2@... (se vazio = usa usuário logado)"
            width={60}
          />
        </InlineField>
      </InlineFieldRow>

      <div style={{ marginBottom: 24, fontSize: 12, opacity: 0.9 }}>
        <div>
          <b>Preview RBAC:</b> {previewUsers.length ? previewUsers.join(', ') : '(nenhum)'}
        </div>
        <div style={{ opacity: 0.75 }}>
          Separadores aceitos: vírgula, ponto-e-vírgula, quebra de linha.
        </div>
      </div>

      <h3>Dashboards ({dashboards.length})</h3>

      <div style={{ marginBottom: 12 }}>
        <Button size="sm" variant="secondary" onClick={selectAll} disabled={!dashboards.length}>
          Selecionar todos
        </Button>

        <Button
          size="sm"
          variant="primary"
          onClick={doImport}
          style={{ marginLeft: 8 }}
          disabled={!selectedUids.size || loading}
        >
          Importar selecionados ({selectedUids.size})
        </Button>

        <Button
          size="sm"
          variant="secondary"
          onClick={() => setDebugOpen((v) => !v)}
          style={{ marginLeft: 8 }}
        >
          {debugOpen ? 'Ocultar debug' : 'Mostrar debug'}
        </Button>
      </div>

      {loading && <Spinner />}

      {debugOpen && (
        <div style={{ marginBottom: 16 }}>
          <div style={{ fontSize: 12, opacity: 0.9, marginBottom: 6 }}>
            <b>Último payload enviado:</b>
          </div>
          <pre style={{ margin: 0, padding: 12, border: '1px solid #333', borderRadius: 6, overflowX: 'auto' }}>
            {lastPayload ? JSON.stringify(lastPayload, null, 2) : '(nenhum ainda)'}
          </pre>

          <div style={{ fontSize: 12, opacity: 0.9, marginTop: 12, marginBottom: 6 }}>
            <b>Última resposta do backend:</b>
          </div>
          <pre style={{ margin: 0, padding: 12, border: '1px solid #333', borderRadius: 6, overflowX: 'auto' }}>
            {lastResponse ? JSON.stringify(lastResponse, null, 2) : '(nenhuma ainda)'}
          </pre>
        </div>
      )}

      <table style={{ width: '100%' }}>
        <thead>
          <tr>
            <th />
            <th style={{ textAlign: 'left' }}>Título</th>
            <th style={{ textAlign: 'left' }}>UID</th>
          </tr>
        </thead>
        <tbody>
          {dashboards.map((d) => (
            <tr key={d.uid}>
              <td>
                <input type="checkbox" checked={selectedUids.has(d.uid)} onChange={() => toggleUid(d.uid)} />
              </td>
              <td>{d.title}</td>
              <td>{d.uid}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

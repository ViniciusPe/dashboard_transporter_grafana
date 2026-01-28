// src/pages/ImportPage.tsx

import React, { useEffect, useMemo, useState } from 'react';
import {
  InlineField,
  InlineFieldRow,
  Select,
  Button,
  Spinner,
  Input,
  Alert,
} from '@grafana/ui';
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
  requestedBy?: string;
};

type Msg = { type: 'success' | 'error'; text: string; details?: string };

export const ImportPage = () => {
  const [loading, setLoading] = useState(false);

  // carga “parcial” (pra não parecer travado/chumbado)
  const [loadingDashboards, setLoadingDashboards] = useState(false);
  const [loadingFolders, setLoadingFolders] = useState(false);

  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [sourceEnv, setSourceEnv] = useState<Environment | null>(null);
  const [targetEnv, setTargetEnv] = useState<Environment | null>(null);

  const [folders, setFolders] = useState<Folder[]>([]);
  const [selectedFolder, setSelectedFolder] = useState<Folder | null>(null);

  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [selectedUids, setSelectedUids] = useState<Set<string>>(new Set());

  const [loggedUser, setLoggedUser] = useState<string>('');
  const [requestedBy, setRequestedBy] = useState<string>('');

  const [msg, setMsg] = useState<Msg | null>(null);

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

  function normalizeErr(e: any): { message: string; details?: string } {
    const message = e?.message ? String(e.message) : 'Erro desconhecido';
    // se teu backend mandar stack/texto bruto, cai aqui
    const details =
      e?.stack ? String(e.stack) :
      e?.details ? String(e.details) :
      undefined;
    return { message, details };
  }

  // =========================
  // 1) Carga inicial: user + environments + defaults
  // =========================
  useEffect(() => {
    (async () => {
      setLoading(true);
      setMsg(null);

      try {
        const u = await getGrafanaLoggedUser();
        setLoggedUser(u);

        const envs = await loadEnvironments();
        setEnvironments(envs);

        // defaults (mantém seu comportamento atual)
        const dev = envs[0] || null;
        const hml = envs[1] || null;

        setSourceEnv(dev);
        setTargetEnv(hml);

        // IMPORTANTE:
        // não carrega dashboards/pastas aqui direto,
        // quem faz isso são os useEffects de sourceEnv/targetEnv
      } catch (e: any) {
        const { message, details } = normalizeErr(e);
        setMsg({ type: 'error', text: message, details });
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  // =========================
  // 2) Sempre que muda Ambiente de Origem: recarrega dashboards
  // =========================
  useEffect(() => {
    if (!sourceEnv?.id) {
      // se limpou seleção, limpa lista
      setDashboards([]);
      setSelectedUids(new Set());
      return;
    }

    let cancelled = false;

    (async () => {
      setMsg(null);

      // limpa estado antigo pra não parecer “chumbado”
      setDashboards([]);
      setSelectedUids(new Set());
      setLoadingDashboards(true);

      try {
        const dbs = await loadDashboards(sourceEnv.id);
        if (cancelled) return;
        setDashboards(dbs);
      } catch (e: any) {
        if (cancelled) return;
        const { message, details } = normalizeErr(e);
        setMsg({
          type: 'error',
          text: `Erro ao carregar dashboards do ambiente "${sourceEnv.id}": ${message}`,
          details,
        });
      } finally {
        if (!cancelled) setLoadingDashboards(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [sourceEnv?.id]);

  // =========================
  // 3) Sempre que muda Ambiente de Destino: recarrega pastas
  // =========================
  useEffect(() => {
    if (!targetEnv?.id) {
      setFolders([]);
      setSelectedFolder(null);
      return;
    }

    let cancelled = false;

    (async () => {
      setMsg(null);

      // limpa estado antigo
      setFolders([]);
      setSelectedFolder(null);
      setLoadingFolders(true);

      try {
        const fs = await loadFolders(targetEnv.id);
        if (cancelled) return;

        setFolders(fs);

        // tenta “General” (uid vazio) se existir, senão primeira
        const general = fs.find((f) => (f.uid ?? '') === '') || fs[0] || { uid: '', title: 'General' };
        setSelectedFolder(general);
      } catch (e: any) {
        if (cancelled) return;
        const { message, details } = normalizeErr(e);
        setMsg({
          type: 'error',
          text: `Erro ao carregar pastas do ambiente "${targetEnv.id}": ${message}`,
          details,
        });
      } finally {
        if (!cancelled) setLoadingFolders(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [targetEnv?.id]);

  async function doImport() {
    setMsg(null);

    if (!sourceEnv || !targetEnv) {
      setMsg({ type: 'error', text: 'Selecione Ambiente de Origem e Ambiente de Destino.' });
      return;
    }
    if (!selectedFolder) {
      setMsg({ type: 'error', text: 'Selecione a Pasta no Destino.' });
      return;
    }
    if (!selectedUids.size) {
      setMsg({ type: 'error', text: 'Selecione pelo menos 1 dashboard.' });
      return;
    }

    setLoading(true);
    try {
      const payload: ImportBatchPayload = {
        sourceEnv: sourceEnv.id,
        targetEnv: targetEnv.id,
        folderUid: selectedFolder.uid ?? '',
        uids: Array.from(selectedUids),
        requestedBy: (requestedBy || loggedUser || '').trim() || undefined,
      };

      const res = await apiPost<any>(API_ENDPOINTS.IMPORT_BATCH, payload);
      setMsg({ type: 'success', text: res?.message || 'Importação concluída.' });
    } catch (e: any) {
      const { message, details } = normalizeErr(e);
      setMsg({ type: 'error', text: message, details });
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{ padding: 16, maxWidth: 1100 }}>
      {msg && (
        <Alert severity={msg.type} title={msg.type === 'success' ? 'Sucesso' : 'Erro'}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            <div>{msg.text}</div>
            {msg.type === 'error' && msg.details && (
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap', opacity: 0.9 }}>
                {msg.details}
              </pre>
            )}
          </div>
        </Alert>
      )}

      <h2>Dashboards Transporter</h2>

      {/* === AMBIENTES === */}
      <InlineFieldRow style={{ marginBottom: 16 }}>
        <InlineField label="Ambiente de Origem" labelWidth={22}>
          <Select
            width={28}
            options={envOptions}
            value={
              sourceEnv ? { label: `${sourceEnv.name} (${sourceEnv.id})`, value: sourceEnv.id } : undefined
            }
            onChange={(v) => setSourceEnv(((v as any)?.env as Environment) ?? null)}
            isClearable
          />
        </InlineField>

        <InlineField label="Ambiente de Destino" labelWidth={22}>
          <Select
            width={28}
            options={envOptions}
            value={
              targetEnv ? { label: `${targetEnv.name} (${targetEnv.id})`, value: targetEnv.id } : undefined
            }
            onChange={(v) => setTargetEnv(((v as any)?.env as Environment) ?? null)}
            isClearable
          />
        </InlineField>

        <InlineField label="Pasta no Destino" labelWidth={22}>
          <Select
            width={28}
            options={folderOptions}
            value={
              selectedFolder ? { label: selectedFolder.title || 'General', value: selectedFolder.uid } : undefined
            }
            onChange={(v) => setSelectedFolder(((v as any)?.folder as Folder) ?? null)}
            isDisabled={!targetEnv || loadingFolders}
            placeholder={!targetEnv ? 'Selecione o destino' : loadingFolders ? 'Carregando...' : 'Escolha'}
          />
        </InlineField>
      </InlineFieldRow>

      {/* === USUÁRIO LOGADO (LINHA SOZINHA) === */}
      <InlineFieldRow style={{ marginBottom: 12 }}>
        <InlineField label="Usuário logado" labelWidth={22}>
          <Input value={loggedUser || 'Não identificado'} readOnly width={60} />
        </InlineField>
      </InlineFieldRow>

      {/* === REQUESTED BY (LINHA SOZINHA) === */}
      <InlineFieldRow style={{ marginBottom: 24 }}>
        <InlineField label="requestedBy (opcional)" labelWidth={22}>
          <Input
            value={requestedBy}
            onChange={(e) => setRequestedBy(e.currentTarget.value)}
            placeholder="(vazio = usa usuário logado)"
            width={60}
          />
        </InlineField>
      </InlineFieldRow>

      {/* === DASHBOARDS === */}
      <h3>
        Dashboards ({dashboards.length})
        {loadingDashboards && <span style={{ marginLeft: 10, opacity: 0.8 }}>(carregando...)</span>}
      </h3>

      <div style={{ marginBottom: 12 }}>
        <Button size="sm" variant="secondary" onClick={selectAll} disabled={!dashboards.length || loadingDashboards}>
          Selecionar todos
        </Button>

        <Button
          size="sm"
          variant="primary"
          onClick={doImport}
          style={{ marginLeft: 8 }}
          disabled={!selectedUids.size || loading || loadingDashboards || loadingFolders}
        >
          Importar selecionados ({selectedUids.size})
        </Button>
      </div>

      {(loading || loadingDashboards || loadingFolders) && <Spinner />}

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
                <input
                  type="checkbox"
                  checked={selectedUids.has(d.uid)}
                  onChange={() => toggleUid(d.uid)}
                />
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

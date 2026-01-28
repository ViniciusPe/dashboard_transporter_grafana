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

  useEffect(() => {
    (async () => {
      setLoading(true);
      try {
        const u = await getGrafanaLoggedUser();
        setLoggedUser(u);

        const envs = await loadEnvironments();
        setEnvironments(envs);

        const dev = envs[0] || null;
        const hml = envs[1] || null;

        setSourceEnv(dev);
        setTargetEnv(hml);

        if (dev) setDashboards(await loadDashboards(dev.id));
        if (hml) {
          const fs = await loadFolders(hml.id);
          setFolders(fs);
          setSelectedFolder(fs[0] || { uid: '', title: 'General' });
        }
      } catch (e: any) {
        setMsg({ type: 'error', text: e.message });
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  async function doImport() {
    if (!sourceEnv || !targetEnv || !selectedFolder || !selectedUids.size) return;

    setLoading(true);
    try {
      const payload: ImportBatchPayload = {
        sourceEnv: sourceEnv.id,
        targetEnv: targetEnv.id,
        folderUid: selectedFolder.uid ?? '',
        uids: Array.from(selectedUids),
        requestedBy: requestedBy || loggedUser,
      };

      const res = await apiPost<any>(API_ENDPOINTS.IMPORT_BATCH, payload);
      setMsg({ type: 'success', text: res?.message || 'Importação concluída.' });
    } catch (e: any) {
      setMsg({ type: 'error', text: e.message });
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{ padding: 16, maxWidth: 1100 }}>
      {msg && (
        <Alert severity={msg.type} title={msg.type === 'success' ? 'Sucesso' : 'Erro'}>
          {msg.text}
        </Alert>
      )}

      <h2>Dashboards Transporter</h2>

      {/* === AMBIENTES === */}
      <InlineFieldRow style={{ marginBottom: 16 }}>
        <InlineField label="Ambiente de Origem" labelWidth={22}>
          <Select width={28} options={envOptions} onChange={(v) => setSourceEnv((v as any)?.env)} />
        </InlineField>

        <InlineField label="Ambiente de Destino" labelWidth={22}>
          <Select width={28} options={envOptions} onChange={(v) => setTargetEnv((v as any)?.env)} />
        </InlineField>

        <InlineField label="Pasta no Destino" labelWidth={22}>
          <Select width={28} options={folderOptions} onChange={(v) => setSelectedFolder((v as any)?.folder)} />
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
      <h3>Dashboards ({dashboards.length})</h3>

      <div style={{ marginBottom: 12 }}>
        <Button size="sm" variant="secondary" onClick={selectAll}>
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
      </div>

      {loading && <Spinner />}

      <table style={{ width: '100%' }}>
        <thead>
          <tr>
            <th />
            <th>Título</th>
            <th>UID</th>
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

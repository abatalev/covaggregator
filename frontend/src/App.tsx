import React, { useState, useEffect } from 'react';
import { Button, Table, TextInput, Card, Text } from '@gravity-ui/uikit';
import './App.css';

interface CoverageData {
  instruction?: number;
  line?: number;
  branch?: number;
}

interface StatusRow {
  service: string;
  version: string;
  hasSources: boolean;
  hasClasses: boolean;
  hasUnitCoverage: boolean;
  unitCoverage?: CoverageData | null;
  unitReportUrl: string | null;
  hasRuntimeCoverage: boolean;
  runtimeCoverage?: CoverageData | null;
  runtimeReportUrl: string | null;
  hasFullCoverage: boolean;
  fullCoverage?: CoverageData | null;
  fullReportUrl: string | null;
  lastUpdated: string;
}

interface InstanceStatus {
  instance_id: string;
  host: string;
  port: number;
  service_id: string;
  version: string;
  status: string;
  last_poll: string;
  last_error: string;
}

interface Instance {
  id: string;
  service_id: string;
  host: string;
  port: number;
  version: string;
  created_at: string;
}

interface AddInstanceForm {
  service_id: string;
  host: string;
  port: string;
  version: string;
}

const API_BASE = import.meta.env.VITE_API_BASE_URL || '/api';

interface NexusStatus {
  url: string;
  enabled: boolean;
  reachable: boolean;
}

const getCoverageColor = (value: number): string => {
  if (value >= 80) return '#27ae60';
  if (value >= 50) return '#f39c12';
  return '#e74c3c';
};

function App() {
  const [statuses, setStatuses] = useState<StatusRow[]>([]);
  const [instanceStatuses, setInstanceStatuses] = useState<InstanceStatus[]>([]);
  const [instances, setInstances] = useState<Instance[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [usingMock, setUsingMock] = useState(false);
  const [showInstancePanel, setShowInstancePanel] = useState(false);
  const [addForm, setAddForm] = useState<AddInstanceForm>({ service_id: '', host: '', port: '', version: '' });
  const [addError, setAddError] = useState<string | null>(null);
  const [addSuccess, setAddSuccess] = useState(false);
  const [version, setVersion] = useState<string>('');
  const [nexusStatus, setNexusStatus] = useState<NexusStatus | null>(null);

  useEffect(() => {
    const fetchVersion = async () => {
      try {
        const response = await fetch(`${API_BASE}/version`);
        if (response.ok) {
          const data = await response.json();
          setVersion(data.version);
        }
      } catch (err) {
        console.error('Failed to fetch version:', err);
      }
    };

    const fetchData = async () => {
      try {
        const response = await fetch(`${API_BASE}/status/table`);
        if (!response.ok) {
          throw new Error(`Backend unavailable (${response.status})`);
        }
        const data = await response.json();
        setStatuses(data);
        setUsingMock(false);
        setError(null);
      } catch (err) {
        try {
          const mockResponse = await fetch('http://localhost:3001/status');
          if (!mockResponse.ok) throw new Error('Mock also failed');
          const mockData = await mockResponse.json();
          setStatuses(mockData);
          setUsingMock(true);
          setError(null);
        } catch (mockErr) {
          setError(
            `Failed to load data: ${
              err instanceof Error ? err.message : 'Unknown error'
            }`
          );
        }
      } finally {
        setLoading(false);
      }
    };

    const fetchInstanceData = async () => {
      try {
        const response = await fetch(`${API_BASE}/instances/status`);
        if (response.ok) {
          const data = await response.json();
          setInstanceStatuses(data);
        }
      } catch (err) {
        console.error('Failed to fetch instance statuses:', err);
      }

      try {
        const response = await fetch(`${API_BASE}/instances`);
        if (response.ok) {
          const data = await response.json();
          setInstances(data);
        }
      } catch (err) {
        console.error('Failed to fetch instances:', err);
      }
    };

    const fetchNexusStatus = async () => {
      try {
        const response = await fetch(`${API_BASE}/nexus/status`);
        if (response.ok) {
          const data = await response.json();
          setNexusStatus(data);
        }
      } catch (err) {
        console.error('Failed to fetch nexus status:', err);
      }
    };

    fetchData();
    fetchInstanceData();
    fetchVersion();
    fetchNexusStatus();
    const interval = setInterval(() => {
      fetchData();
      fetchInstanceData();
      fetchNexusStatus();
    }, 30000);
    return () => clearInterval(interval);
  }, []);

  const handleAddInstance = async (e: React.FormEvent) => {
    e.preventDefault();
    setAddError(null);
    setAddSuccess(false);

    const port = parseInt(addForm.port, 10);
    if (isNaN(port) || port <= 0 || port > 65535) {
      setAddError('Invalid port');
      return;
    }

    try {
      const response = await fetch(`${API_BASE}/instances`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          service_id: addForm.service_id,
          host: addForm.host,
          port: port,
          version: addForm.version || undefined,
        }),
      });

      if (!response.ok) {
        const err = await response.json();
        throw new Error(err.error || 'Failed to add instance');
      }

      setAddSuccess(true);
      setAddForm({ service_id: '', host: '', port: '', version: '' });
      
      const instancesRes = await fetch(`${API_BASE}/instances`);
      if (instancesRes.ok) {
        setInstances(await instancesRes.json());
      }
    } catch (err) {
      setAddError(err instanceof Error ? err.message : 'Unknown error');
    }
  };

  const getStatusBadge = (status: string) => {
    const colors: Record<string, string> = {
      success: '#27ae60',
      error: '#e74c3c',
      polling: '#f39c12',
    };
    const color = colors[status] || '#95a5a6';
    return (
      <span style={{ 
        color: color, 
        fontWeight: 'bold',
        padding: '2px 8px',
        borderRadius: '4px',
        backgroundColor: color + '20'
      }}>
        {status}
      </span>
    );
  };

  const formatLastPoll = (timestamp: string) => {
    if (!timestamp) return '-';
    const date = new Date(timestamp);
    const now = new Date();
    const diff = Math.floor((now.getTime() - date.getTime()) / 1000);
    if (diff < 60) return `${diff}s ago`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    return date.toLocaleString();
  };

  const renderCoverageBar = (value: number | undefined, label: string) => {
    if (value === undefined) return <span>-</span>;
    const color = getCoverageColor(value);
    return (
      <div style={{ width: '80px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.75rem', marginBottom: '2px' }}>
          <span>{label}</span>
          <span style={{ color, fontWeight: 'bold' }}>{value.toFixed(1)}%</span>
        </div>
        <div style={{ width: '100%', height: '6px', background: '#e0e0e0', borderRadius: '3px', overflow: 'hidden' }}>
          <div style={{ width: `${value}%`, height: '100%', background: color, borderRadius: '3px', transition: 'width 0.3s' }} />
        </div>
      </div>
    );
  };

  const renderCoverageCell = (coverage: CoverageData | null | undefined) => {
    if (!coverage) return <span>-</span>;
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
        {coverage.instruction !== undefined && renderCoverageBar(coverage.instruction, 'Instr')}
        {coverage.line !== undefined && renderCoverageBar(coverage.line, 'Line')}
        {coverage.branch !== undefined && renderCoverageBar(coverage.branch, 'Branch')}
      </div>
    );
  };

  const renderLink = (url: string | null, label: string) => {
    if (!url) return null;
    return (
      <a 
        href={url} 
        target="_blank" 
        rel="noopener noreferrer"
        style={{ fontSize: '0.75rem', color: '#0066cc', textDecoration: 'none' }}
      >
        {label}
      </a>
    );
  };

  const calculateAvgCoverage = (): number => {
    if (statuses.length === 0) return 0;
    const total = statuses.reduce((sum, s) => {
      return sum + (s.fullCoverage?.instruction || s.unitCoverage?.instruction || 0);
    }, 0);
    return total / statuses.length;
  };

  const columnsStatus = [
    { id: 'service', name: 'Service / Version', width: '25%', verticalAlign: 'top' as const },
    { id: 'unitCoverage', name: 'Unit Coverage', width: '10%', verticalAlign: 'top' as const },
    { id: 'runtimeCoverage', name: 'Runtime Coverage', width: '10%', verticalAlign: 'top' as const },
    { id: 'fullCoverage', name: 'Full Coverage', width: '10%', verticalAlign: 'top' as const },
  ];

  const columnsInstances = [
    { id: 'service', name: 'Service' },
    { id: 'host', name: 'Host:Port' },
    { id: 'version', name: 'Version' },
    { id: 'status', name: 'Status' },
    { id: 'lastPoll', name: 'Last Poll' },
    { id: 'error', name: 'Error' },
  ];

  const getRowsStatus = () => {
    return statuses.map((row, idx) => {
      const serviceHosts = instanceStatuses.filter(s => s.service_id === row.service);
      const activeHosts = serviceHosts.filter(s => s.status === 'success').length;
      const inactiveHosts = serviceHosts.filter(s => s.status !== 'success').length;
      const totalHosts = serviceHosts.length;

      return {
        key: `${row.service}-${row.version}-${idx}`,
        service: (
          <div style={{ verticalAlign: 'top' }}>
            <strong>{row.service} v{row.version}</strong>
            <br />
            <span style={{ color: '#999', fontSize: '0.75rem' }}>
              {row.lastUpdated ? new Date(row.lastUpdated).toLocaleString() : '-'}
            </span>
            <br />
            <span style={{ fontSize: '0.7rem' }}>
              <span style={{ color: '#27ae60' }}>{activeHosts}▲</span>
              {' '}
              <span style={{ color: '#e74c3c' }}>{inactiveHosts}▼</span>
              {' '}
              <span style={{ color: '#666' }}>({totalHosts})</span>
            </span>
          </div>
        ),
        unitCoverage: (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            {renderCoverageCell(row.unitCoverage)}
            {renderLink(row.unitReportUrl, 'Details')}
          </div>
        ),
        runtimeCoverage: (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            {renderCoverageCell(row.runtimeCoverage)}
            {renderLink(row.runtimeReportUrl, 'Details')}
          </div>
        ),
        fullCoverage: (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            {renderCoverageCell(row.fullCoverage)}
            {renderLink(row.fullReportUrl, 'Details')}
          </div>
        ),
      };
    });
  };

  const getRowsInstances = () => instanceStatuses.map((inst) => ({
    key: inst.instance_id,
    service: inst.service_id,
    host: `${inst.host}:${inst.port}`,
    version: inst.version || '-',
    status: getStatusBadge(inst.status),
    lastPoll: formatLastPoll(inst.last_poll),
    error: inst.last_error || '-',
  }));

  const activeInstances = instanceStatuses.filter(s => s.status === 'success').length;

  if (loading) {
    return (
      <div className="App-container">
        <Card className="header-card">
          <Text variant="header-1">JaCoCo Coverage</Text>
          <Text variant="body-2" color="secondary">Loading...</Text>
        </Card>
      </div>
    );
  }

  return (
    <div className="App-container">
      {error && (
        <div style={{
          background: '#fee2e2',
          border: '1px solid #ef4444',
          borderRadius: '8px',
          padding: '12px 16px',
          marginBottom: '16px',
          color: '#dc2626'
        }}>
          <strong>Connection error:</strong> {error}
        </div>
      )}

      <Card className="header-card">
        <div className="header-content">
          <div className="header-text">
            <Text variant="display-1" className="header-title">JaCoCo Coverage</Text>
            <Text variant="body-2" className="header-subtitle">
              {version && <span className="version-badge">{version}</span>}
              {usingMock && <span className="mock-badge">Mock</span>}
            </Text>
          </div>
          <Button 
            onClick={() => setShowInstancePanel(!showInstancePanel)}
            variant="normal"
            size="l"
          >
            {showInstancePanel ? 'Hide Hosts' : 'Show Hosts'}
          </Button>
        </div>
      </Card>

      <div className="stats-row">
        <Card className="stat-card">
          <Text variant="header-2">{statuses.length}</Text>
          <Text variant="body-2" color="secondary">Services</Text>
        </Card>
        <Card className="stat-card">
          <Text variant="header-2" style={{ color: getCoverageColor(calculateAvgCoverage()) }}>
            {calculateAvgCoverage().toFixed(1)}%
          </Text>
          <Text variant="body-2" color="secondary">Avg Coverage</Text>
        </Card>
        <Card className="stat-card">
          <Text variant="header-2">
            <span style={{ color: '#27ae60' }}>{activeInstances}</span>
            {' / '}
            <span>{instances.length}</span>
          </Text>
          <Text variant="body-2" color="secondary">Active / Total</Text>
        </Card>
        {nexusStatus && nexusStatus.enabled && (
          <Card className="stat-card">
            <a href={nexusStatus.url} target="_blank" rel="noopener noreferrer" style={{ textDecoration: 'none' }}>
              <Text variant="header-2" style={{ color: nexusStatus.reachable ? '#27ae60' : '#e74c3c' }}>
                ●
              </Text>
              <Text variant="body-2" color="secondary">Nexus</Text>
            </a>
          </Card>
        )}
      </div>

      <Card className="table-card">
        <Table columns={columnsStatus} data={getRowsStatus()} />
      </Card>

      {showInstancePanel && (
        <Card className="table-card">
          <Text variant="header-2" className="section-title">Hosts</Text>
          
          <form onSubmit={handleAddInstance} className="add-form">
            <TextInput
              placeholder="service_id"
              value={addForm.service_id}
              onUpdate={(value) => setAddForm({...addForm, service_id: value})}
            />
            <TextInput
              placeholder="host"
              value={addForm.host}
              onUpdate={(value) => setAddForm({...addForm, host: value})}
            />
            <TextInput
              placeholder="port"
              value={addForm.port}
              onUpdate={(value) => setAddForm({...addForm, port: value})}
            />
            <TextInput
              placeholder="version"
              value={addForm.version}
              onUpdate={(value) => setAddForm({...addForm, version: value})}
            />
            <Button type="submit">Add</Button>
          </form>
          {addError && <Text color="danger" className="form-message">{addError}</Text>}
          {addSuccess && <Text color="positive" className="form-message">Instance added!</Text>}

          {instanceStatuses.length > 0 ? (
            <Table columns={columnsInstances} data={getRowsInstances()} />
          ) : (
            <Text color="secondary">No instances</Text>
          )}
        </Card>
      )}

      <footer className="footer">
        <Text variant="body-2" color="secondary">
          Auto-updates every 30s · Data from <code>/api/status/table</code>
        </Text>
      </footer>
    </div>
  );
}

export default App;

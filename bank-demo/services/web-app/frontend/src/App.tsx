import React, { useState, useEffect } from 'react';
import { PreviewControls } from './components/PreviewControls';
import { AccountBalance } from './components/AccountBalance';
import { Topology } from './components/Topology';
import { ModuleLoader } from './components/ModuleLoader';
import { useApi } from './hooks/useApi';
import { AccountData, RegistryResponse, TopologyNode } from './types';

export const App: React.FC = () => {
  const [previewId, setPreviewId] = useState('');
  
  // Read preview ID from URL params on load and on browser navigation
  useEffect(() => {
    const readFromURL = () => {
      const params = new URLSearchParams(window.location.search);
      const pid = params.get('x-preview-id') || '';
      setPreviewId(pid);
    };
    readFromURL();
    window.addEventListener('popstate', readFromURL);
    return () => window.removeEventListener('popstate', readFromURL);
  }, []);

  const handleApplyPreview = (id: string) => {
    setPreviewId(id);
    const url = new URL(window.location.href);
    if (id) {
      url.searchParams.set('x-preview-id', id);
    } else {
      url.searchParams.delete('x-preview-id');
    }
    window.history.pushState({}, '', url.toString());
    // Dispatch popstate so any listeners (including this component) re-sync
    window.dispatchEvent(new PopStateEvent('popstate'));
  };

  const { data: accountsData, loading: accountsLoading } = useApi<AccountData>('/api/accounts', previewId);
  const { data: registryData } = useApi<RegistryResponse>('/api/module-registry', previewId);

  const { data: topologyData } = useApi<Record<string, string>>('/topology', previewId);

  // Build topology from real API data
  const serviceOrder = ['web-app', 'gateway', 'accounts-api', 'payments-api', 'payments-module'];
  const topologyNodes: TopologyNode[] = serviceOrder.map(name => {
    const version = topologyData?.[name] || 'baseline';
    return {
      id: name,
      name,
      version,
      isPreview: version !== 'baseline' && version !== 'unavailable',
    };
  });

  const isPreviewActive = topologyNodes.some(n => n.isPreview);
  const paymentsModuleInfo = registryData?.modules?.['payments'];

  return (
    <div style={{
      minHeight: '100vh',
      background: '#0a0a0a',
      color: '#fff',
      fontFamily: 'Inter, system-ui, sans-serif'
    }}>
      <header style={{
        background: '#1a1a1a',
        padding: '1rem 2rem',
        borderBottom: '1px solid #333',
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <span style={{ fontSize: '1.5rem', fontWeight: 'bold' }}>🏦</span>
          <h1 style={{ margin: 0, fontSize: '1.25rem', color: '#bb86fc' }}>Diverge Bank</h1>
        </div>
        <div style={{
          padding: '6px 16px',
          borderRadius: '20px',
          fontSize: '0.875rem',
          fontWeight: 'bold',
          background: isPreviewActive ? 'rgba(3, 218, 198, 0.15)' : '#2a2a2a',
          color: isPreviewActive ? '#03dac6' : '#aaa',
          border: isPreviewActive ? '1px solid rgba(3, 218, 198, 0.5)' : '1px solid #444'
        }}>
          {isPreviewActive ? `PREVIEW ${previewId}` : 'BASELINE'}
        </div>
      </header>

      <main style={{ padding: '2rem', maxWidth: '1200px', margin: '0 auto' }}>
        <PreviewControls currentPreviewId={previewId} onApply={handleApplyPreview} />
        
        <Topology nodes={topologyNodes} />
        
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: '1.5rem' }}>
          <div>
            <AccountBalance balance={accountsData?.accounts?.[0]?.balance ?? null} loading={accountsLoading} />
          </div>
          <div>
            <ModuleLoader 
              moduleUrl={paymentsModuleInfo?.url || null} 
              previewId={previewId} 
            />
          </div>
        </div>
      </main>
    </div>
  );
};

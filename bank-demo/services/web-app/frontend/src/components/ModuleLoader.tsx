import React, { useEffect, useState } from 'react';

interface Props {
  moduleUrl: string | null;
  previewId: string;
}

export const ModuleLoader: React.FC<Props> = ({ moduleUrl, previewId }) => {
  const [RemoteComponent, setRemoteComponent] = useState<React.ComponentType<any> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!moduleUrl) {
      setLoading(false);
      return;
    }

    const loadModule = async () => {
      try {
        // @module-federation/vite registers the remote during hostInit.
        // The remote 'paymentsModule' is declared in vite.config.ts,
        // so dynamic import resolves through the MF runtime.
        // @ts-ignore - MF virtual module resolved at runtime
        const module = await import('paymentsModule/PaymentsPanel');
        if (module) {
          const Component = module.default || module.PaymentsPanel;
          setRemoteComponent(() => Component);
        } else {
          setError('Remote module returned null');
        }
      } catch (err: any) {
        console.error('Module Federation load error:', err);
        setError(`Failed to load payments module: ${err.message}`);
      } finally {
        setLoading(false);
      }
    };

    loadModule();
  }, [moduleUrl]);

  if (!moduleUrl) {
    return (
      <div style={{ padding: '2rem', textAlign: 'center', background: '#1a1a1a', borderRadius: '8px', border: '1px dashed #444', color: '#aaa' }}>
        No module registry data available
      </div>
    );
  }

  if (loading) {
    return (
      <div style={{ padding: '3rem', textAlign: 'center', background: '#1a1a1a', borderRadius: '8px', border: '1px solid #333' }}>
        Loading Remote Module...
      </div>
    );
  }

  if (error) {
    return (
      <div style={{ padding: '2rem', background: 'rgba(244, 67, 54, 0.1)', border: '1px solid #f44336', borderRadius: '8px', color: '#f44336' }}>
        <h3>Module Error</h3>
        <p>{error}</p>
      </div>
    );
  }

  if (RemoteComponent) {
    return <RemoteComponent previewId={previewId} />;
  }

  return null;
};

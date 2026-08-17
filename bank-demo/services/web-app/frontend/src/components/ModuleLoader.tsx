import React, { useEffect, useState, useRef } from 'react';

interface Props {
  moduleUrl: string | null;
  previewId: string;
}

export const ModuleLoader: React.FC<Props> = ({ moduleUrl, previewId }) => {
  const [RemoteComponent, setRemoteComponent] = useState<React.ComponentType<any> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const loadedUrlRef = useRef<string | null>(null);

  useEffect(() => {
    if (!moduleUrl) return;

    if (loadedUrlRef.current === moduleUrl) {
      // Already loaded or trying to load this URL
      return;
    }

    loadedUrlRef.current = moduleUrl;
    setLoading(true);
    setError(null);

    const scriptId = `module-${moduleUrl.replace(/[^a-z0-9]/gi, '')}`;
    
    // Remove old script if exists
    const existingScript = document.getElementById(scriptId);
    if (existingScript) {
      existingScript.remove();
    }

    const script = document.createElement('script');
    script.id = scriptId;
    script.src = moduleUrl;
    script.type = 'module';
    script.onload = () => {
      // @ts-ignore
      const module = window.paymentsModule;
      if (module) {
        module.get('./PaymentsPanel').then((factory: any) => {
          const Component = factory();
          setRemoteComponent(() => Component.default);
          setLoading(false);
        }).catch((err: any) => {
          setError(`Failed to instantiate module: ${err.message}`);
          setLoading(false);
        });
      } else {
        // Fallback for demo simplicity (using a global if MF isn't fully working in simple script mode)
        // @ts-ignore
        if (window.__divergeModules && window.__divergeModules.payments) {
          // @ts-ignore
          setRemoteComponent(() => window.__divergeModules.payments.PaymentsPanel);
        } else {
          setError("Module loaded but not registered globally");
        }
        setLoading(false);
      }
    };
    script.onerror = () => {
      setError(`Failed to load script from ${moduleUrl}`);
      setLoading(false);
    };

    document.head.appendChild(script);

  }, [moduleUrl]);

  if (!moduleUrl) {
    return <div style={{ padding: '2rem', textAlign: 'center', background: '#1a1a1a', borderRadius: '8px', border: '1px dashed #444', color: '#aaa' }}>No module registry data available</div>;
  }

  if (loading) {
    return <div style={{ padding: '3rem', textAlign: 'center', background: '#1a1a1a', borderRadius: '8px', border: '1px solid #333' }}>Loading Remote Module...</div>;
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

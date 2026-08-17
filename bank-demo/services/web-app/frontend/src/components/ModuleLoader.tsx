import React, { useEffect, useState, lazy, Suspense, Component, ErrorInfo } from 'react';
import { registerRemotes, loadRemote } from '@module-federation/enhanced/runtime';

interface Props {
  moduleUrl: string | null;
  previewId: string;
}

// Error boundary prevents any MF crash from blanking the entire app
class ModuleErrorBoundary extends Component<
  { children: React.ReactNode; fallback: React.ReactNode },
  { hasError: boolean; error: Error | null }
> {
  state = { hasError: false, error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('ModuleErrorBoundary caught:', error, info);
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback;
    }
    return this.props.children;
  }
}

let remotesRegistered = false;

function registerPaymentsRemote(entryUrl: string) {
  if (remotesRegistered) return;
  try {
    registerRemotes([
      {
        name: 'paymentsModule',
        entry: entryUrl,
        type: 'module',
      },
    ]);
    remotesRegistered = true;
  } catch (err) {
    console.warn('Failed to register remote, may already be registered:', err);
    remotesRegistered = true;
  }
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
        // Register the remote dynamically at runtime
        registerPaymentsRemote(moduleUrl);

        // Load the exposed component via MF runtime
        const module = await loadRemote<{ default: React.ComponentType<any> }>(
          'paymentsModule/PaymentsPanel'
        );

        if (module) {
          setRemoteComponent(() => module.default || module);
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

  const errorFallback = (
    <div style={{
      padding: '2rem',
      background: 'rgba(244, 67, 54, 0.1)',
      border: '1px solid #f44336',
      borderRadius: '8px',
      color: '#f44336'
    }}>
      <h3>Module Error</h3>
      <p>{error || 'An unexpected error occurred loading the payments module'}</p>
    </div>
  );

  if (!moduleUrl) {
    return (
      <div style={{
        padding: '2rem', textAlign: 'center', background: '#1a1a1a',
        borderRadius: '8px', border: '1px dashed #444', color: '#aaa'
      }}>
        No module registry data available
      </div>
    );
  }

  if (loading) {
    return (
      <div style={{
        padding: '3rem', textAlign: 'center', background: '#1a1a1a',
        borderRadius: '8px', border: '1px solid #333'
      }}>
        Loading Remote Module...
      </div>
    );
  }

  if (error) {
    return errorFallback;
  }

  if (RemoteComponent) {
    return (
      <ModuleErrorBoundary fallback={errorFallback}>
        <Suspense fallback={<div style={{ padding: '2rem', textAlign: 'center' }}>Loading...</div>}>
          <RemoteComponent previewId={previewId} />
        </Suspense>
      </ModuleErrorBoundary>
    );
  }

  return null;
};

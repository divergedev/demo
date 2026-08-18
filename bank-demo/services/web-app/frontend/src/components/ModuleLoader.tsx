import React, { useEffect, useState, Component, ErrorInfo } from 'react';

interface Props {
  moduleUrl: string | null;
  previewId: string;
}

// Error boundary that shows the ACTUAL error
class ModuleErrorBoundary extends Component<
  { children: React.ReactNode },
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
      return (
        <div style={{
          padding: '2rem',
          background: 'rgba(244, 67, 54, 0.1)',
          border: '1px solid #f44336',
          borderRadius: '8px',
          color: '#f44336',
          fontSize: '0.875rem',
          wordBreak: 'break-word',
        }}>
          <h3 style={{ margin: '0 0 0.5rem' }}>Module Render Error</h3>
          <p>{this.state.error?.message || 'Unknown error'}</p>
          <pre style={{ fontSize: '0.75rem', color: '#aaa', whiteSpace: 'pre-wrap' }}>
            {this.state.error?.stack?.split('\n').slice(0, 5).join('\n')}
          </pre>
        </div>
      );
    }
    return this.props.children;
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
        // Load the remoteEntry as an ES module - it self-registers on the MF global cache
        const entryModule = await import(/* @vite-ignore */ moduleUrl);
        
        // The MF container should be on the imported module
        const container = entryModule.default || entryModule;
        
        if (container && typeof container.get === 'function') {
          // Standard MF container API
          if (typeof container.init === 'function') {
            // Share scope init - pass empty if not sharing
            try {
              await container.init({});
            } catch (e) {
              // May already be initialized
              console.warn('Container init warning:', e);
            }
          }
          const factory = await container.get('./PaymentsPanel');
          const module = factory();
          const Component = module.default || module.PaymentsPanel || module;
          if (typeof Component === 'function') {
            setRemoteComponent(() => Component);
          } else {
            setError(`Module loaded but PaymentsPanel is not a component (got ${typeof Component})`);
          }
        } else {
          // Fallback: check window globals (PaymentsPanel registers on window.__divergeModules)
          await new Promise(r => setTimeout(r, 200));
          const globals = (window as any).__divergeModules?.payments;
          if (globals?.PaymentsPanel) {
            setRemoteComponent(() => globals.PaymentsPanel);
          } else {
            // Show what we got for debugging
            const keys = Object.keys(entryModule || {}).join(', ');
            setError(`Container not found on remote entry. Module exports: [${keys}]. Container type: ${typeof container}, has .get: ${typeof container?.get}`);
          }
        }
      } catch (err: any) {
        console.error('Module load error:', err);
        setError(`Load error: ${err.message}`);
      } finally {
        setLoading(false);
      }
    };

    loadModule();
  }, [moduleUrl]);

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
    return (
      <div style={{
        padding: '2rem', background: 'rgba(244, 67, 54, 0.1)',
        border: '1px solid #f44336', borderRadius: '8px', color: '#f44336',
        fontSize: '0.875rem', wordBreak: 'break-word',
      }}>
        <h3 style={{ margin: '0 0 0.5rem' }}>Module Error</h3>
        <p>{error}</p>
      </div>
    );
  }

  if (RemoteComponent) {
    return (
      <ModuleErrorBoundary>
        <RemoteComponent previewId={previewId} />
      </ModuleErrorBoundary>
    );
  }

  return null;
};

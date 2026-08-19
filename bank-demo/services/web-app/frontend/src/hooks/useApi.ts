import { useState, useEffect } from 'react';

export function useApi<T>(url: string, previewId: string) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    let mounted = true;
    const fetchData = async () => {
      setLoading(true);
      setError(null);
      try {
        const headers: Record<string, string> = {};
        if (previewId) {
          headers['x-preview-id'] = previewId;
        }
        const res = await fetch(url, { headers, cache: 'no-store' });
        if (!res.ok) {
          throw new Error(`API error: ${res.status}`);
        }
        const json = await res.json();
        if (mounted) {
          setData(json);
        }
      } catch (err) {
        if (mounted) {
          setError(err instanceof Error ? err : new Error(String(err)));
        }
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    };
    fetchData();
    return () => { mounted = false; };
  }, [url, previewId]);

  return { data, loading, error };
}

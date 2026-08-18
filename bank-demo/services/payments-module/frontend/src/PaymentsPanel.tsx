import React, { useEffect, useState } from 'react';
import { PaymentSummary, Transaction } from './types';

interface PaymentsPanelProps {
  previewId?: string;
}

const PaymentsPanel: React.FC<PaymentsPanelProps> = ({ previewId }) => {
  const [summary, setSummary] = useState<PaymentSummary | null>(null);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      try {
        const headers: Record<string, string> = {};
        if (previewId) {
          headers['x-preview-id'] = previewId;
        }

        const [summaryRes, txRes] = await Promise.all([
          fetch('/api/payments', { headers, cache: 'no-store' }),
          fetch('/api/payments/transactions', { headers, cache: 'no-store' })
        ]);

        if (summaryRes.ok) {
          setSummary(await summaryRes.json());
        }
        if (txRes.ok) {
          const txData = await txRes.json();
          setTransactions(Array.isArray(txData) ? txData : txData.transactions || []);
        }
      } catch (err) {
        console.error("Failed to fetch payments data", err);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [previewId]);

  if (loading) {
    return <div style={{ padding: '2rem', color: '#fff', textAlign: 'center' }}>Loading payments...</div>;
  }

  // Determine if a preview is actually active based on backend data,
  // not just whether a preview ID was typed. If the backend returned
  // fraud scores or fees, a real preview service is responding.
  const hasPreviewData = transactions.some(tx => tx.fraud_score !== undefined || tx.fee !== undefined)
    || (summary?.total_fees !== undefined && summary.total_fees > 0)
    || (summary?.flagged_count !== undefined && summary.flagged_count > 0);
  const isPreview = Boolean(previewId) && hasPreviewData;
  const hasFraud = (summary?.flagged_count ?? 0) > 0 || transactions.some(tx => tx.fraud_score !== undefined);

  return (
    <div style={{
      background: '#1e1e1e',
      color: '#fff',
      padding: '1.5rem',
      borderRadius: '8px',
      fontFamily: 'Inter, system-ui, sans-serif',
      boxShadow: '0 4px 6px rgba(0,0,0,0.3)',
      transition: 'all 0.3s ease'
    }}>
      <style>{`
        @keyframes pulseGlow {
          0% { box-shadow: 0 0 0 0 rgba(244, 67, 54, 0.4); }
          70% { box-shadow: 0 0 0 10px rgba(244, 67, 54, 0); }
          100% { box-shadow: 0 0 0 0 rgba(244, 67, 54, 0); }
        }
        @keyframes slideIn {
          from { opacity: 0; transform: translateY(10px); }
          to { opacity: 1; transform: translateY(0); }
        }
      `}</style>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.5rem' }}>
        <h2 style={{ margin: 0, fontSize: '1.5rem' }}>Payments</h2>
        <span style={{
          padding: '4px 12px',
          borderRadius: '16px',
          fontSize: '0.875rem',
          fontWeight: 'bold',
          background: isPreview ? 'rgba(3, 218, 198, 0.2)' : 'rgba(255, 255, 255, 0.1)',
          color: isPreview ? '#03dac6' : '#aaa'
        }}>
          {isPreview ? `PREVIEW ${previewId} (v${summary?.version || '1.0.0'})` : `BASELINE (v${summary?.version || '1.0.0'})`}
        </span>
      </div>

      {hasFraud && isPreview && (
        <div style={{
          background: 'linear-gradient(135deg, rgba(244,67,54,0.15) 0%, rgba(255,171,0,0.15) 100%)',
          border: '1px solid #f44336',
          color: '#fff',
          padding: '1.5rem',
          borderRadius: '8px',
          marginBottom: '1.5rem',
          animation: 'pulseGlow 2s infinite'
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '1rem' }}>
            <span style={{ fontSize: '2rem' }}>🚨</span>
            <div>
              <div style={{ fontWeight: 'bold', fontSize: '1.2rem', color: '#f44336' }}>Fraud Detection Active</div>
              <div style={{ fontSize: '0.9rem', color: '#ffab00' }}>{summary?.flagged_count || 0} suspicious transactions intercepted</div>
            </div>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {transactions.filter(t => t.fraud_flag).map(tx => (
              <div key={tx.id} style={{
                background: 'rgba(0,0,0,0.2)',
                padding: '8px 12px',
                borderRadius: '4px',
                display: 'flex',
                justifyContent: 'space-between',
                fontSize: '0.85rem'
              }}>
                <span style={{ fontFamily: 'monospace' }}>{tx.id.substring(0, 8)}</span>
                <span style={{ color: '#ffab00' }}>{tx.fraud_reason || 'Suspicious activity'}</span>
                <span>${tx.amount.toLocaleString()}</span>
                <span style={{ color: '#f44336', fontWeight: 'bold' }}>{(tx.fraud_score! * 100).toFixed(0)}% Risk</span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div style={{ display: 'flex', gap: '2rem', marginBottom: '2rem' }}>
        <div>
          <div style={{ fontSize: '0.875rem', color: '#aaa', marginBottom: '4px' }}>Total Payments</div>
          <div style={{ fontSize: '1.5rem', fontWeight: 'bold' }}>{summary?.total_payments || 0}</div>
        </div>
        <div>
          <div style={{ fontSize: '0.875rem', color: '#aaa', marginBottom: '4px' }}>Total Volume</div>
          <div style={{ fontSize: '1.5rem', fontWeight: 'bold' }}>${(summary?.total_volume || 0).toLocaleString()}</div>
        </div>
        {isPreview && summary?.total_fees !== undefined && (
          <div>
            <div style={{ fontSize: '0.875rem', color: '#03dac6', marginBottom: '4px' }}>Total Fees ✨</div>
            <div style={{ fontSize: '1.5rem', fontWeight: 'bold', color: '#03dac6' }}>${summary.total_fees.toLocaleString()}</div>
          </div>
        )}
      </div>

      <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
        <thead>
          <tr style={{ borderBottom: '1px solid #333', color: '#aaa' }}>
            <th style={{ padding: '12px 8px', fontWeight: 'normal' }}>ID</th>
            <th style={{ padding: '12px 8px', fontWeight: 'normal' }}>From</th>
            <th style={{ padding: '12px 8px', fontWeight: 'normal' }}>To</th>
            <th style={{ padding: '12px 8px', fontWeight: 'normal' }}>Amount</th>
            {isPreview && <th style={{ padding: '12px 8px', fontWeight: 'normal', color: '#03dac6' }}>Fee ✨</th>}
            <th style={{ padding: '12px 8px', fontWeight: 'normal' }}>Status</th>
            {isPreview && <th style={{ padding: '12px 8px', fontWeight: 'normal', color: '#f44336' }}>Fraud Risk</th>}
          </tr>
        </thead>
        <tbody>
          {transactions.map((tx, index) => (
            <tr key={tx.id} style={{ 
              borderBottom: '1px solid #2a2a2a',
              background: tx.fraud_flag ? 'rgba(244, 67, 54, 0.05)' : 'transparent',
              animation: 'slideIn 0.3s ease-out forwards',
              animationDelay: `${index * 0.05}s`,
              opacity: 0
            }}>
              <td style={{ padding: '12px 8px', fontFamily: 'monospace' }}>{tx.id.substring(0, 8)}</td>
              <td style={{ padding: '12px 8px' }}>{tx.from}</td>
              <td style={{ padding: '12px 8px' }}>{tx.to}</td>
              <td style={{ padding: '12px 8px' }}>${tx.amount.toLocaleString()}</td>
              {isPreview && (
                <td style={{ padding: '12px 8px', color: '#03dac6' }}>
                  {tx.fee !== undefined ? `$${tx.fee.toLocaleString()}` : '-'}
                </td>
              )}
              <td style={{ padding: '12px 8px' }}>
                <span style={{
                  padding: '2px 8px',
                  borderRadius: '12px',
                  fontSize: '0.8rem',
                  background: tx.status === 'completed' ? 'rgba(76, 175, 80, 0.2)' : 'rgba(255, 171, 0, 0.2)',
                  color: tx.status === 'completed' ? '#4caf50' : '#ffab00'
                }}>
                  {tx.status}
                </span>
              </td>
              {isPreview && (
                <td style={{ padding: '12px 8px' }}>
                  {tx.fraud_score !== undefined ? (
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <div style={{
                        width: '8px',
                        height: '8px',
                        borderRadius: '50%',
                        backgroundColor: tx.fraud_score >= 0.7 ? '#f44336' : (tx.fraud_score >= 0.3 ? '#ffab00' : '#4caf50')
                      }} />
                      <span>{(tx.fraud_score * 100).toFixed(0)}%</span>
                    </div>
                  ) : '-'}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

// Expose on window for simpler demo loading
if (typeof window !== 'undefined') {
  // @ts-ignore
  window.__divergeModules = window.__divergeModules || {};
  // @ts-ignore
  window.__divergeModules.payments = window.__divergeModules.payments || {};
  // @ts-ignore
  window.__divergeModules.payments.PaymentsPanel = PaymentsPanel;
}

export default PaymentsPanel;

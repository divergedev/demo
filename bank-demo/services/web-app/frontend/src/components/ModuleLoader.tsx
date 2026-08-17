import React, { useEffect, useState } from 'react';
import { useApi } from '../hooks/useApi';

interface Props {
  moduleUrl: string | null;
  previewId: string;
}

interface Payment {
  id: string;
  from: string;
  to: string;
  amount: number;
  fraud_score: number;
  fraud_flag: boolean;
  fee: number;
  status: string;
}

interface PaymentsData {
  payments: Payment[];
  flagged_count: number;
  total_fees: number;
  version: string;
  service: string;
}

// Inline payments panel — renders the same data the MFE remote would
const InlinePaymentsPanel: React.FC<{ previewId: string }> = ({ previewId }) => {
  const { data, loading } = useApi<PaymentsData>('/api/payments', previewId);

  if (loading) {
    return <div style={{ padding: '2rem', textAlign: 'center', color: '#aaa' }}>Loading payments...</div>;
  }

  if (!data) {
    return <div style={{ padding: '2rem', color: '#f44336' }}>Failed to load payments data</div>;
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h3 style={{ margin: 0, color: '#bb86fc' }}>Payments</h3>
        <div style={{ display: 'flex', gap: '1rem', fontSize: '0.875rem' }}>
          <span style={{ color: '#f44336' }}>🚨 {data.flagged_count} flagged</span>
          <span style={{ color: '#aaa' }}>Fees: ${data.total_fees.toFixed(2)}</span>
          <span style={{
            padding: '2px 8px',
            borderRadius: '4px',
            fontSize: '0.75rem',
            background: data.version === 'baseline' ? '#2a2a2a' : 'rgba(3, 218, 198, 0.15)',
            color: data.version === 'baseline' ? '#aaa' : '#03dac6',
          }}>
            {data.version}
          </span>
        </div>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {data.payments.map((tx) => (
          <div
            key={tx.id}
            style={{
              display: 'grid',
              gridTemplateColumns: '80px 1fr 1fr 100px 80px 70px',
              alignItems: 'center',
              padding: '12px 16px',
              background: tx.fraud_flag ? 'rgba(244, 67, 54, 0.08)' : '#1a1a1a',
              border: tx.fraud_flag ? '1px solid rgba(244, 67, 54, 0.3)' : '1px solid #2a2a2a',
              borderRadius: '8px',
              fontSize: '0.875rem',
            }}
          >
            <span style={{ color: '#666', fontFamily: 'monospace' }}>{tx.id}</span>
            <span style={{ color: '#ccc' }}>{tx.from}</span>
            <span style={{ color: '#ccc' }}>→ {tx.to}</span>
            <span style={{ color: '#fff', fontWeight: 600, textAlign: 'right' }}>
              ${tx.amount.toLocaleString()}
            </span>
            <span style={{
              textAlign: 'center',
              color: tx.fraud_score > 0.7 ? '#f44336' : tx.fraud_score > 0.3 ? '#ff9800' : '#4caf50',
              fontWeight: 600,
            }}>
              {(tx.fraud_score * 100).toFixed(0)}%
            </span>
            <span style={{
              textAlign: 'center',
              padding: '2px 8px',
              borderRadius: '12px',
              fontSize: '0.75rem',
              background: tx.status === 'completed' ? 'rgba(76, 175, 80, 0.15)' : 'rgba(255, 152, 0, 0.15)',
              color: tx.status === 'completed' ? '#4caf50' : '#ff9800',
            }}>
              {tx.status}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};

export const ModuleLoader: React.FC<Props> = ({ moduleUrl, previewId }) => {
  // Render inline payments panel directly — Module Federation remote loading
  // can be enabled once the MF runtime integration is fully configured.
  return <InlinePaymentsPanel previewId={previewId} />;
};

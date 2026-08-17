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
          fetch('/api/payments', { headers }),
          fetch('/api/payments/transactions', { headers })
        ]);

        if (summaryRes.ok) {
          setSummary(await summaryRes.json());
        }
        if (txRes.ok) {
          setTransactions(await txRes.json());
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

  const isPreview = Boolean(previewId);
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
          {isPreview ? `PREVIEW ${previewId}` : 'BASELINE'}
        </span>
      </div>

      {hasFraud && isPreview && (
        <div style={{
          background: 'rgba(255, 171, 0, 0.15)',
          border: '1px solid #ffab00',
          color: '#ffab00',
          padding: '1rem',
          borderRadius: '6px',
          marginBottom: '1.5rem',
          display: 'flex',
          alignItems: 'center',
          gap: '12px'
        }}>
          <span style={{ fontSize: '1.5rem' }}>🛡️</span>
          <div>
            <div style={{ fontWeight: 'bold' }}>Fraud Detection Active</div>
            <div style={{ fontSize: '0.9rem', opacity: 0.9 }}>{summary?.flagged_count || 0} suspicious transactions flagged</div>
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
          </tr>
        </thead>
        <tbody>
          {transactions.map(tx => (
            <tr key={tx.id} style={{ borderBottom: '1px solid #2a2a2a' }}>
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
                {isPreview && tx.fraud_score !== undefined && tx.fraud_score > 0.7 && (
                   <span style={{ marginLeft: '8px', fontSize: '1rem' }} title={`Fraud Score: ${tx.fraud_score}`}>⚠️</span>
                )}
              </td>
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

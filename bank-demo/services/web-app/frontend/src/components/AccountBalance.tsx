import React from 'react';

interface Props {
  balance: number | null;
  loading: boolean;
}

export const AccountBalance: React.FC<Props> = ({ balance, loading }) => {
  return (
    <div style={{
      background: '#1a1a1a',
      padding: '1.5rem',
      borderRadius: '8px',
      marginBottom: '1.5rem',
      boxShadow: '0 4px 6px rgba(0,0,0,0.3)',
      border: '1px solid #333'
    }}>
      <h3 style={{ margin: '0 0 1rem 0', color: '#fff', fontSize: '1.2rem', fontWeight: 'normal' }}>Total Balance</h3>
      {loading ? (
        <div style={{ fontSize: '2.5rem', fontWeight: 'bold', color: '#666' }}>$...</div>
      ) : (
        <div style={{ fontSize: '2.5rem', fontWeight: 'bold', color: '#fff' }}>
          ${balance?.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }) || '0.00'}
        </div>
      )}
    </div>
  );
};

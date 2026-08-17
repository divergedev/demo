import React, { useState } from 'react';

interface Props {
  currentPreviewId: string;
  onApply: (id: string) => void;
}

export const PreviewControls: React.FC<Props> = ({ currentPreviewId, onApply }) => {
  const [inputValue, setInputValue] = useState(currentPreviewId);

  return (
    <div style={{
      background: '#1a1a1a',
      padding: '1.5rem',
      borderRadius: '8px',
      marginBottom: '1.5rem',
      boxShadow: '0 4px 6px rgba(0,0,0,0.3)',
      border: currentPreviewId ? '1px solid #03dac6' : '1px solid #333'
    }}>
      <h3 style={{ margin: '0 0 1rem 0', color: '#bb86fc' }}>Environment Controls</h3>
      <div style={{ display: 'flex', gap: '1rem' }}>
        <input
          type="text"
          value={inputValue}
          onChange={e => setInputValue(e.target.value)}
          placeholder="Enter preview ID (e.g. preview-42)"
          style={{
            flex: 1,
            padding: '0.5rem',
            background: '#2a2a2a',
            border: '1px solid #444',
            color: '#fff',
            borderRadius: '4px'
          }}
        />
        <button
          onClick={() => onApply(inputValue)}
          style={{
            padding: '0.5rem 1rem',
            background: '#bb86fc',
            color: '#000',
            border: 'none',
            borderRadius: '4px',
            fontWeight: 'bold',
            cursor: 'pointer'
          }}
        >
          Apply
        </button>
        <button
          onClick={() => {
            setInputValue('');
            onApply('');
          }}
          style={{
            padding: '0.5rem 1rem',
            background: 'transparent',
            color: '#aaa',
            border: '1px solid #444',
            borderRadius: '4px',
            cursor: 'pointer'
          }}
        >
          Clear
        </button>
      </div>
      <div style={{ marginTop: '1rem', fontSize: '0.9rem', color: currentPreviewId ? '#03dac6' : '#aaa' }}>
        {currentPreviewId ? `Currently viewing preview environment: ${currentPreviewId}` : 'Currently viewing baseline environment'}
      </div>
    </div>
  );
};

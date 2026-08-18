import React from 'react';
import { TopologyNode } from '../types';

interface Props {
  nodes: TopologyNode[];
}

export const Topology: React.FC<Props> = ({ nodes }) => {
  return (
    <div style={{
      background: '#1a1a1a',
      padding: '1.5rem',
      borderRadius: '8px',
      marginBottom: '1.5rem',
      boxShadow: '0 4px 6px rgba(0,0,0,0.3)',
      border: '1px solid #333',
      overflowX: 'auto'
    }}>
      <h3 style={{ margin: '0 0 1.5rem 0', color: '#bb86fc' }}>Service Topology</h3>
      <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', minWidth: 'max-content' }}>
        {nodes.map((node, index) => (
          <React.Fragment key={node.id}>
            <div style={{
              padding: '1rem',
              borderRadius: '8px',
              background: node.isPreview ? 'rgba(3, 218, 198, 0.1)' : '#2a2a2a',
              border: node.isPreview ? '2px solid #03dac6' : '2px solid #444',
              minWidth: '140px',
              textAlign: 'center',
              transition: 'all 0.3s ease'
            }}>
              <div style={{ fontWeight: 'bold', marginBottom: '8px', color: node.isPreview ? '#03dac6' : '#fff' }}>
                {node.name}
              </div>
              <div style={{ fontSize: '0.8rem', color: '#aaa' }}>
                {node.version}
              </div>
            </div>
            {index < nodes.length - 1 && (
              <div style={{ color: '#666', fontWeight: 'bold' }}>→</div>
            )}
          </React.Fragment>
        ))}
      </div>
    </div>
  );
};

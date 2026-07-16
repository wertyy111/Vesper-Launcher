import type { HostSnapshot } from '../../types';

export function LoadingScreen({
  hostSnapshot,
  runtimeError,
  onRetry
}: {
  hostSnapshot: HostSnapshot | null;
  runtimeError: string | null;
  onRetry: () => void;
}) {
  const update = hostSnapshot?.update;
  const isIndeterminate = update?.isIndeterminate ?? true;
  const progressPercent = update?.progressPercent ?? 0;

  return (
    <div className="loading-screen" style={{
      position: 'fixed', inset: 0,
      display: 'flex', flexDirection: 'column',
      alignItems: 'center', justifyContent: 'center',
      color: '#fff', zIndex: 9999,
      padding: '40px'
    }}>
      <div className="glass-panel" style={{ padding: '40px', borderRadius: '16px', minWidth: '400px', textAlign: 'center', background: 'rgba(18, 18, 20, 0.6)' }}>
        <h1 style={{ marginBottom: '10px', fontSize: '24px', fontWeight: 600 }}>Vesper Launcher</h1>
        <h2 style={{ fontSize: '16px', fontWeight: 500, color: '#e2e8f0', marginBottom: '30px' }}>
          {update?.message || 'Запуск...'}
        </h2>
        
        <p style={{ marginBottom: '15px', color: '#94a3b8', fontSize: '13px' }}>
          {update?.detailMessage || 'Загрузка компонентов...'}
        </p>

        <div className="classic-progress" style={{ width: '100%', height: '8px', background: 'rgba(0,0,0,0.3)', borderRadius: '4px', overflow: 'hidden', position: 'relative' }}>
          <div className={`classic-progress-fill ${isIndeterminate ? 'indeterminate' : ''}`} style={{ width: `${progressPercent}%`, height: '100%', background: '#4f46e5', transition: 'width 0.2s ease', position: 'absolute' }} />
        </div>
        
        <p style={{ marginTop: '10px', fontSize: '12px', color: '#64748b' }}>
          {update?.progressText || 'Ожидание...'}
        </p>

        {runtimeError || hostSnapshot?.errorMessage ? (
          <div style={{ marginTop: '20px', padding: '15px', background: 'rgba(220, 38, 38, 0.2)', color: '#f87171', borderRadius: '8px', fontSize: '13px' }}>
            <div>{runtimeError || hostSnapshot?.errorMessage}</div>
            <button
              type="button"
              onClick={onRetry}
              style={{
                marginTop: '12px',
                padding: '9px 18px',
                border: '1px solid rgba(255,255,255,0.18)',
                borderRadius: '8px',
                background: '#4f46e5',
                color: '#fff',
                cursor: 'pointer'
              }}
            >
              Повторить
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
}

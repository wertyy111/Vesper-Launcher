import type { HostSnapshot } from '../../types';

export function LoadingScreen({ hostSnapshot, runtimeError }: { hostSnapshot: HostSnapshot | null; runtimeError: string | null }) {
  return (
    <div className="loading-screen glass-panel">
      <p className="eyebrow">Vesper Launcher</p>
      <h1>Запуск интерфейса</h1>
      <p>Загрузка компонентов React-оболочки...</p>
      <div className="progress-bar-track large">
        <div className="progress-bar-fill indeterminate" style={{ width: '100%' }} />
      </div>
      <div className="loading-meta">
        <span>{hostSnapshot?.phase || 'starting'}</span>
        <span>Ожидание...</span>
      </div>
      {hostSnapshot?.errorMessage || runtimeError ? <p className="error-copy">{hostSnapshot?.errorMessage || runtimeError}</p> : null}
    </div>
  );
}

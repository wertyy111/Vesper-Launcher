import { photinoBridge } from '../../bridge';
import type { PanelRenderProps } from '../../types';
import { PanelHeader } from '../common/PanelHeader';

export function BackgroundPanel({ launcher }: PanelRenderProps) {
  const background = launcher.background;
  const items = (background.items ?? []) as Array<Record<string, any>>;

  return (
    <>
      <PanelHeader title="Фон" subtitle="Выберите фон интерфейса лаунчера или загрузите свой." />

      <section className="background-shell">
        <div className="background-catalog-header" style={{ marginBottom: '16px' }}>
          <button
            className="subtle-button left-liquid-glass-button settings-liquid-glass-button rounded-pill"
            onClick={() => photinoBridge.sendCommand('background.openFolder')}
            type="button"
          >
            <span className="settings-liquid-glass-layer liquid-glass-layer" aria-hidden="true" />
            <span className="left-liquid-glass-content">Папка с фонами</span>
          </button>
        </div>

        <div className="background-catalog-grid">
          {/* Custom & preset backgrounds */}
          {items.map((item) => (
            <div
              key={String(item.fileName)}
              className={`background-catalog-item ${item.isActive ? 'selected' : ''}`}
              onClick={() => photinoBridge.sendCommand('background.setPreset', { fileName: item.fileName })}
              role="button"
              tabIndex={0}
            >
              <img src={item.url || '/assets/bg-day.png'} alt={String(item.label || item.fileName)} />
              {item.isActive && (
                <div className="selected-badge">Выбрано</div>
              )}
              <span>{item.label || item.fileName}</span>
            </div>
          ))}
        </div>
      </section>
    </>
  );
}

import { photinoBridge } from '../../bridge';
import type { LauncherSnapshot } from '../../types';

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 Б';
  const units = ['Б', 'КБ', 'МБ', 'ГБ'];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

export function UpdatesPanel({ launcher }: { launcher: LauncherSnapshot }) {
  const updates = launcher.updates ?? {};
  const isAvailable = Boolean(updates.isAvailable);
  const isInstalling = Boolean(updates.isInstalling);
  const progressPercent = Math.min(Math.max(Number(updates.progressPercent ?? 0), 0), 100);
  const formattedSize = formatBytes(Number(updates.downloadBytes));
  const versionLabel = String(updates.version || 'Новая версия');

  return (
    <section className="wpf-section-shell settings-shell updates-settings-shell">
      <div className="settings-pages-stack updates-pages-stack">
        <div className="wpf-settings-page settings-tab-page active updates-settings-page">
          {isAvailable ? (
            <>
              <article className="updates-release-card" data-testid="updates-release-card">
                <span className="updates-release-card__shape shape-top-left" aria-hidden="true" />
                <span className="updates-release-card__shape shape-top-right" aria-hidden="true" />
                <span className="updates-release-card__shape shape-bottom" aria-hidden="true" />
                <div className="updates-release-card__stage">
                  <div className="updates-release-card__wordmark" aria-label="Vesper">
                    vesper
                  </div>
                </div>
                <footer className="updates-release-card__footer">
                  <div>
                    <strong className="updates-release-version">Версия {versionLabel}</strong>
                    <span>Готова к установке без отдельного установщика</span>
                  </div>
                  <span className="updates-ready-badge">Доступно</span>
                </footer>
              </article>

              <div className="settings-toggle-row updates-setting-row">
                <div>
                  <strong>{updates.usesDelta ? 'Только изменённые файлы' : 'Полный пакет'}</strong>
                  <p>
                    {updates.usesDelta
                      ? 'Velopack применит компактное delta-обновление.'
                      : 'Delta недоступна, поэтому будет загружен полный пакет.'}
                  </p>
                </div>
                <span className="updates-size-pill">{formattedSize}</span>
              </div>

              {isInstalling ? (
                <div className="updates-install-progress" aria-label={`Загружено ${progressPercent}%`}>
                  <div style={{ width: `${progressPercent}%` }} />
                  <span>{progressPercent}%</span>
                </div>
              ) : null}

              {updates.errorMessage ? (
                <p className="updates-error">{String(updates.errorMessage)}</p>
              ) : null}

              <button
                className="primary-button left-liquid-glass-button settings-liquid-glass-button updates-install-button"
                disabled={isInstalling}
                onClick={() => photinoBridge.sendCommand('updates.install')}
                type="button"
              >
                <span className="settings-liquid-glass-layer liquid-glass-layer" aria-hidden="true" />
                <span className="left-liquid-glass-content">
                  {isInstalling ? 'Устанавливаем…' : 'Обновить сейчас'}
                </span>
              </button>
            </>
          ) : (
            <div className="settings-toggle-row updates-setting-row">
              <div>
                <strong>Установлена последняя версия</strong>
                <p>Новые обновления появятся в верхней панели автоматически.</p>
              </div>
              <span className="updates-ready-badge is-current">Актуально</span>
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

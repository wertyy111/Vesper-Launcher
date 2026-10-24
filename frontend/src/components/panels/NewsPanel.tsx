import type { LauncherSnapshot } from '../../types';
import { PanelHeader } from '../common/PanelHeader';

export function NewsPanel({ launcher }: { launcher: LauncherSnapshot }) {
  const languageCode = String(launcher.settings?.minecraftLanguageCode || navigator.language || 'ru').toLowerCase();
  const isRussian = languageCode.startsWith('ru') || languageCode === 'auto';

  return (
    <>
      <PanelHeader
        title={isRussian ? 'Новости' : 'News'}
        subtitle={isRussian ? 'Свежие анонсы и обновления Vesper.' : 'Latest Vesper announcements and updates.'}
      />

      <section className="wpf-section-shell settings-shell news-settings-shell">
        <div className="settings-pages-stack news-pages-stack">
          <div className="wpf-settings-page settings-tab-page active news-settings-page">
            <div className="news-empty-state">
              <p>{isRussian ? 'Новостей пока нет.' : 'No news available yet.'}</p>
            </div>
          </div>
        </div>
      </section>
    </>
  );
}

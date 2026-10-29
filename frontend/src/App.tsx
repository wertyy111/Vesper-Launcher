import { startTransition, useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type MouseEvent } from 'react';
import { photinoBridge } from './bridge';
import type { PointerEvent as ReactPointerEvent } from 'react';
import type { GlassTuningSettings, HostSnapshot, PanelRenderProps } from './types';

import { AccountPanel } from './components/panels/AccountPanel';
import { SettingsPanel } from './components/panels/SettingsPanel';
import { SkinPanel } from './components/panels/SkinPanel';
import { BackgroundPanel } from './components/panels/BackgroundPanel';
import { ModsPanel } from './components/panels/ModsPanel';
import { FriendsPanel } from './components/panels/FriendsPanel';
import { NotificationsPanel } from './components/panels/NotificationsPanel';
import { UpdatesPanel } from './components/panels/UpdatesPanel';
import { NewsPanel } from './components/panels/NewsPanel';
import { EmptyPanel } from './components/panels/EmptyPanel';
import { LoadingScreen } from './components/common/LoadingScreen';
import html2canvasScriptUrl from './vendor/html2canvas.min.js?url';
import liquidGlScriptUrl from './vendor/liquidGL.js?url';

const RESIZE_DIRECTIONS = ['top', 'right', 'bottom', 'left', 'top-left', 'top-right', 'bottom-left', 'bottom-right'];

const DIRECTION_TO_WAILS_EDGE: Record<string, string> = {
  top: 'n-resize',
  bottom: 's-resize',
  left: 'w-resize',
  right: 'e-resize',
  'top-left': 'nw-resize',
  'top-right': 'ne-resize',
  'bottom-left': 'sw-resize',
  'bottom-right': 'se-resize',
};

const BOTTOM_SECTION_ITEMS = [
  { id: 'skin', label: 'Скин' },
  { id: 'background', label: 'Фон' },
  { id: 'mods', label: 'Моды' },
  { id: 'friends', label: 'Друзья' }
];

const WPF_WIDE_PANEL_SECTIONS = new Set(['account', 'settings', 'notifications', 'updates', 'news', ...BOTTOM_SECTION_ITEMS.map((section) => section.id)]);
const SETTINGS_WIDTH_PANEL_SECTIONS = new Set(['account', 'settings', 'notifications', 'updates', 'news', ...BOTTOM_SECTION_ITEMS.map((section) => section.id)]);
const MAX_RENDERED_VERSION_OPTIONS = 120;

const DEFAULT_GLASS_TUNING: GlassTuningSettings = {
  refraction: 0.014,
  bevelDepth: 0.02,
  bevelWidth: 0.15,
  frost: 0,
  resolution: 1.5
};

function App() {
  const [hostSnapshot, setHostSnapshot] = useState<HostSnapshot | null>(null);
  const [runtimeError, setRuntimeError] = useState<string | null>(null);
  const [glassTuning, setGlassTuning] = useState<GlassTuningSettings>(DEFAULT_GLASS_TUNING);
  const [accountForm, setAccountForm] = useState({ mode: 'login', username: '', password: '' });
  const [accountDirty, setAccountDirty] = useState(false);
  const [friendDraft, setFriendDraft] = useState('');
  const [friendDirty, setFriendDirty] = useState(false);
  const [javaPathDraft, setJavaPathDraft] = useState('');
  const [javaPathDirty, setJavaPathDirty] = useState(false);
  const [jvmArgsDraft, setJvmArgsDraft] = useState('');
  const [jvmArgsDirty, setJvmArgsDirty] = useState(false);
  const [versionMenuOpen, setVersionMenuOpen] = useState(false);
  const [versionSearch, setVersionSearch] = useState('');
  const [_versionCategoryFilter] = useState('all');
  const [versionMenuStyle, setVersionMenuStyle] = useState<CSSProperties>({});
  const versionPickerToggleRef = useRef<HTMLDivElement | null>(null);
  const settingsLiquidButtonRef = useRef<HTMLButtonElement | null>(null);
  const liquidGlassBatchRef = useRef(0);
  const liquidGlassResolutionRef = useRef(glassTuning.resolution);
  const clientReadySentRef = useRef(false);

  const launcher = hostSnapshot?.launcher ?? null;
  const theme = launcher?.theme ?? {};
  const main = launcher?.main ?? {};
  const account = launcher?.account ?? {};
  const settings = launcher?.settings ?? {};
  const mods = launcher?.mods ?? {};
  const friends = launcher?.friends ?? {};
  const loadingState = !hostSnapshot || !launcher;
  const activeSection = launcher?.activeSection ?? 'none';

  const setGlassTuningValue = (field: keyof GlassTuningSettings, value: number) => {
    setGlassTuning((prev) => ({ ...prev, [field]: value }));
  };

  const markClientUiReady = () => {
    if (clientReadySentRef.current) {
      return;
    }

    clientReadySentRef.current = true;
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        photinoBridge.sendCommand('client.uiReady');
      });
    });
  };

  useEffect(() => {
    return photinoBridge.subscribe((message) => {
      if (message.type === 'snapshot' && message.data) {
        startTransition(() => {
          setHostSnapshot(message.data ?? null);
          setRuntimeError(null);
        });
        return;
      }

      if (message.type === 'error') {
        setRuntimeError(message.message ?? 'Не удалось обработать команду shell.');
      }
    });
  }, []);

  useEffect(() => {
    const userAgent = window.navigator.userAgent.toLowerCase();
    const platform = window.navigator.platform?.toLowerCase() || '';
    const platformClass = userAgent.includes('mac os') || userAgent.includes('macintosh') || platform.includes('mac')
      ? 'platform-macos'
      : userAgent.includes('linux') || platform.includes('linux')
        ? 'platform-linux'
        : 'platform-windows';

    document.documentElement.classList.add(platformClass);
    return () => document.documentElement.classList.remove(platformClass);
  }, []);

  useEffect(() => {
    photinoBridge.requestSnapshot();
  }, []);

  useEffect(() => {
    if (!loadingState) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      setRuntimeError('Лаунчер не получил данные от внутреннего сервиса. Нажмите «Повторить».');
    }, 12000);

    return () => window.clearTimeout(timeoutId);
  }, [loadingState]);

  useEffect(() => {
    if (loadingState) {
      return;
    }

    let timeoutId = 0;
    let initialLoad = true;
    const publishWindowSize = () => {
      if (initialLoad) {
        return;
      }
      window.clearTimeout(timeoutId);
      timeoutId = window.setTimeout(() => {
        const w = Math.round(window.innerWidth);
        const h = Math.round(window.innerHeight);
        // Do not save maximized, fullscreen, or unconstrained dimensions
        if (
          w < 900 || w > 3840 ||
          h < 580 || h > 2160 ||
          w >= (window.screen.availWidth - 60) ||
          h >= (window.screen.availHeight - 60)
        ) {
          return;
        }
        photinoBridge.sendCommand('host.windowSize', {
          width: w,
          height: h
        });
      }, 500);
    };

    const onWindowResize = () => {
      window.dispatchEvent(new Event('vesper-layout-change'));
      publishWindowSize();
    };

    // Attach listener only after window has fully settled to avoid initial mount sizing glitches
    const settleTimer = window.setTimeout(() => {
      initialLoad = false;
      window.addEventListener('resize', onWindowResize);
    }, 2500);

    return () => {
      window.clearTimeout(settleTimer);
      window.clearTimeout(timeoutId);
      window.removeEventListener('resize', onWindowResize);
    };
  }, [loadingState]);

  useEffect(() => {
    if (!launcher || accountDirty) return;
    setAccountForm((prev) => ({
      mode: account.mode === 'summary' ? prev.mode : account.mode ?? 'login',
      username: account.nicknameInput ?? '',
      password: ''
    }));
  }, [launcher, account.mode, account.nicknameInput, accountDirty]);

  useEffect(() => {
    if (!launcher || friendDirty) return;
    setFriendDraft(friends.friendNicknameInput ?? '');
  }, [launcher, friends.friendNicknameInput, friendDirty]);

  useEffect(() => {
    if (!launcher || javaPathDirty) return;
    setJavaPathDraft(settings.javaPath ?? '');
  }, [launcher, settings.javaPath, javaPathDirty]);

  useEffect(() => {
    if (!launcher || jvmArgsDirty) return;
    setJvmArgsDraft(settings.extraJvmArgs ?? '');
  }, [launcher, settings.extraJvmArgs, jvmArgsDirty]);

  useEffect(() => {
    if (!versionMenuOpen) return;

    const closeOnOutsidePointer = (event: PointerEvent) => {
      if ((event.target as HTMLElement | null)?.closest('.version-picker')) {
        return;
      }

      setVersionMenuOpen(false);
    };

    const closeOnEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') {
        setVersionMenuOpen(false);
      }
    };

    window.addEventListener('pointerdown', closeOnOutsidePointer);
    window.addEventListener('keydown', closeOnEscape);
    return () => {
      window.removeEventListener('pointerdown', closeOnOutsidePointer);
      window.removeEventListener('keydown', closeOnEscape);
    };
  }, [versionMenuOpen]);

  useLayoutEffect(() => {
    if (!versionMenuOpen) return;

    let rafId = 0;
    const computeGeometry = (): CSSProperties | null => {
      const toggle = versionPickerToggleRef.current;
      if (!toggle) {
        return null;
      }

      const rect = toggle.getBoundingClientRect();
      const bottomBar = document.querySelector('.bottom-action-bar');
      const maxBottom = Math.min(window.innerHeight - 12, (bottomBar?.getBoundingClientRect().top ?? window.innerHeight) - 10);
      const targetHeight = Math.min(404, Math.max(rect.height, maxBottom - rect.top));

      return {
        left: `${rect.left}px`,
        top: `${rect.top}px`,
        width: `${rect.width}px`,
        height: `${targetHeight}px`
      };
    };

    const initialGeometry = computeGeometry();
    const toggle = versionPickerToggleRef.current;
    if (initialGeometry && toggle) {
      setVersionMenuStyle({
        ...initialGeometry,
        height: `${toggle.getBoundingClientRect().height}px`
      });
      rafId = window.requestAnimationFrame(() => setVersionMenuStyle(initialGeometry));
    }

    const handleResize = () => {
      const nextGeometry = computeGeometry();
      if (nextGeometry) {
        setVersionMenuStyle(nextGeometry);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => {
      window.cancelAnimationFrame(rafId);
      window.removeEventListener('resize', handleResize);
    };
  }, [versionMenuOpen]);

  useEffect(() => {
    if (loadingState) return;

    let cancelled = false;
    const readyFallbackId = window.setTimeout(() => {
      if (!cancelled) {
        markClientUiReady();
      }
    }, 8000);

    if (!settingsLiquidButtonRef.current) {
      markClientUiReady();
      return () => {
        cancelled = true;
        window.clearTimeout(readyFallbackId);
      };
    }

    const loadScript = (src: string, id: string) => new Promise<void>((resolve, reject) => {
      const existingScript = document.querySelector<HTMLScriptElement>(`script[data-vesper-vendor="${id}"]`);
      if (existingScript) {
        if (existingScript.dataset.loaded === 'true') {
          resolve();
          return;
        }

        existingScript.addEventListener('load', () => resolve(), { once: true });
        existingScript.addEventListener('error', () => reject(new Error(`Failed to load ${id}`)), { once: true });
        return;
      }

      const script = document.createElement('script');
      script.src = src;
      script.async = false;
      script.dataset.vesperVendor = id;
      script.addEventListener('load', () => {
        script.dataset.loaded = 'true';
        resolve();
      }, { once: true });
      script.addEventListener('error', () => reject(new Error(`Failed to load ${id}`)), { once: true });
      document.body.appendChild(script);
    });

    let initTimeout: number | undefined;
    const initializeLiquidSettingsButton = async () => {
      if (cancelled) return;
      try {
        await loadScript(html2canvasScriptUrl, 'html2canvas');
        await loadScript(liquidGlScriptUrl, 'liquidgl');
        await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()));

        if (cancelled || !settingsLiquidButtonRef.current || !window.liquidGL) {
          markClientUiReady();
          return;
        }

        const targets = Array.from(document.querySelectorAll<HTMLElement>('.liquid-glass-layer:not([data-liquid-initialized])'))
          .filter((element) => {
            const style = window.getComputedStyle(element);
            return element.offsetWidth > 0 && element.offsetHeight > 0 && style.visibility !== 'hidden' && style.display !== 'none';
          });

        if (!targets.length) {
          markClientUiReady();
          return;
        }

        const batchId = `vesper-liquid-${++liquidGlassBatchRef.current}`;
        targets.forEach((element) => {
          element.dataset.liquidInitialized = 'true';
          element.dataset.liquidBatch = batchId;
        });

        window.liquidGL({
          target: `[data-liquid-batch="${batchId}"]`,
          snapshot: '.liquid-glass-snapshot-source',
          resolution: glassTuning.resolution,
          refraction: glassTuning.refraction,
          bevelDepth: glassTuning.bevelDepth,
          bevelWidth: glassTuning.bevelWidth,
          frost: glassTuning.frost,
          shadow: false,
          specular: false,
          reveal: 'none',
          tilt: false,
          magnify: 1
        });

        markClientUiReady();
      } catch (error) {
        console.warn('liquidGL controls test failed', error);
        markClientUiReady();
      }
    };

    const debouncedInit = () => {
      window.clearTimeout(initTimeout);
      initTimeout = window.setTimeout(initializeLiquidSettingsButton, 100);
    };

    initializeLiquidSettingsButton();
    window.addEventListener('vesper-layout-change', debouncedInit);
    const observer = new MutationObserver((mutations) => {
      let hasNewLiquidLayer = false;
      for (const m of mutations) {
        for (let i = 0; i < m.addedNodes.length; i++) {
          const node = m.addedNodes[i];
          if (node.nodeType === 1) {
            const el = node as HTMLElement;
            if (el.classList?.contains('liquid-glass-layer') || el.querySelector?.('.liquid-glass-layer:not([data-liquid-initialized])')) {
              hasNewLiquidLayer = true;
              break;
            }
          }
        }
        if (hasNewLiquidLayer) break;
      }
      if (hasNewLiquidLayer) {
        debouncedInit();
      }
    });
    observer.observe(document.body, {
      childList: true,
      subtree: true
    });

    return () => {
      cancelled = true;
      window.clearTimeout(readyFallbackId);
      window.clearTimeout(initTimeout);
      window.removeEventListener('vesper-layout-change', debouncedInit);
      observer.disconnect();
    };
  }, [loadingState]);

  useEffect(() => {
    const renderer = (window as any).__liquidGLRenderer__;
    if (!renderer?.lenses) return;

    renderer.lenses.forEach((lens: any) => {
      lens.options.refraction = glassTuning.refraction;
      lens.options.bevelDepth = glassTuning.bevelDepth;
      lens.options.bevelWidth = glassTuning.bevelWidth;
      lens.options.frost = glassTuning.frost;
    });

    if (liquidGlassResolutionRef.current !== glassTuning.resolution) {
      liquidGlassResolutionRef.current = glassTuning.resolution;
      renderer._snapshotResolution = glassTuning.resolution;
      renderer.captureSnapshot?.();
      return;
    }

    renderer.render?.();
  }, [glassTuning]);

  const pageBackground = theme.backgroundUrl
    ? `url(${theme.backgroundUrl}) center / cover no-repeat`
    : 'radial-gradient(circle at top, rgba(111, 144, 166, 0.18) 0%, rgba(10, 12, 15, 0) 40%), linear-gradient(180deg, #0b0e11 0%, #060709 100%)';

  useEffect(() => {
    let cancelled = false;
    const rootEl = document.getElementById('root');
    if (rootEl) {
      rootEl.style.background = 'transparent';
      const isLinux = document.documentElement.classList.contains('platform-linux');
      rootEl.style.borderRadius = isLinux ? '0px' : '20px';
      rootEl.style.overflow = 'hidden';
    }

    const nextFrame = () => new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()));

    (async () => {
      if (theme.backgroundUrl) {
        const img = new Image();
        img.src = theme.backgroundUrl;
        if (!img.complete) {
          await new Promise<void>((resolve) => {
            img.addEventListener('load', () => resolve(), { once: true });
            img.addEventListener('error', () => resolve(), { once: true });
          });
        }
        try {
          await img.decode();
        } catch {
          // ignore decode errors
        }
      }

      await document.fonts?.ready;
      await nextFrame();
      await nextFrame();
      if (cancelled) return;

      const renderer = (window as any).__liquidGLRenderer__;
      if (renderer?.captureSnapshot) {
        for (let attempt = 0; renderer._capturing && attempt < 80 && !cancelled; attempt += 1) {
          await new Promise<void>((resolve) => window.setTimeout(resolve, 50));
        }
        if (!cancelled) {
          await renderer.captureSnapshot();
          renderer.render?.();
          photinoBridge.sendCommand('host.logJsInfo', {
            message: `Liquid glass snapshot refreshed: ${theme.backgroundUrl || 'procedural'}`
          });
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [pageBackground, theme.backgroundUrl]);

  const openSection = (sectionId: string) => {
    const nextSection = launcher?.activeSection === sectionId ? 'none' : sectionId;
    setHostSnapshot((prev) => {
      if (!prev || !prev.launcher) return prev;
      return {
        ...prev,
        launcher: {
          ...prev.launcher,
          activeSection: nextSection
        }
      };
    });
    if (nextSection === 'none') {
      photinoBridge.sendCommand('shell.closeSection');
    } else {
      photinoBridge.sendCommand('shell.openSection', { section: sectionId });
    }
  };

  const handleActionClick = (action: () => void) => (event: MouseEvent<HTMLElement>) => {
    event.preventDefault();
    event.stopPropagation();
    action();
  };

  const startWindowDrag = (event: ReactPointerEvent<HTMLElement>) => {
    if (event.button !== 0 || event.detail > 1) return;
    if ((event.target as HTMLElement).closest('button, input, select, textarea, a, .titlebar-actions')) return;
    photinoBridge.sendCommand('host.startDrag', {
      screenX: event.screenX,
      screenY: event.screenY
    });
  };

  const continueWindowDrag = (event: ReactPointerEvent<HTMLElement>) => {
    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
      photinoBridge.sendCommand('host.continueDrag', {
        screenX: event.screenX,
        screenY: event.screenY
      });
    }
  };

  const endWindowDrag = (event: ReactPointerEvent<HTMLElement>) => {
    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
      event.currentTarget.releasePointerCapture?.(event.pointerId);
    }
    photinoBridge.sendCommand('host.endDrag');
  };

  const startWindowResize = (event: MouseEvent<HTMLElement>, direction: string) => {
    if (event.button !== 0) return;
    const edge = DIRECTION_TO_WAILS_EDGE[direction];
    const w = window as any;
    if (edge) {
      if (w.wails?.flags) {
        w.wails.flags.resizeEdge = edge;
      }
      if (typeof w.WailsInvoke === 'function') {
        event.preventDefault();
        w.WailsInvoke(`resize:${edge}`);
        return;
      }
    }
    event.preventDefault();
    photinoBridge.sendCommand('host.startResize', { direction });
  };

  const submitAccount = () => {
    setAccountDirty(false);
    photinoBridge.sendCommand('account.submit', {
      mode: accountForm.mode,
      username: accountForm.username,
      password: accountForm.password
    });
    setAccountForm((prev) => ({ ...prev, password: '' }));
  };

  const toggleSelectedMod = (projectId: string) => {
    const selected = new Set<string>((mods.selectedProjectIds ?? []) as string[]);
    if (selected.has(projectId)) selected.delete(projectId);
    else selected.add(projectId);
    photinoBridge.sendCommand('mods.setSelectedProjects', { projectIds: Array.from(selected) });
  };

  const versionOptions = (main.availableVersions ?? main.versions ?? []) as Array<Record<string, any>>;
  const selectedVersionKey = String(main.selectedVersionKey ?? '');
  const selectedVersion = versionOptions.find((entry) => String(entry.key ?? '') === selectedVersionKey);
  const selectedVersionLabel = selectedVersion?.displayName ?? main.inlineVersionLabel ?? 'Версия: не выбрана';
  const filteredVersionOptions = useMemo(() => {
    if (!versionSearch.trim()) return versionOptions;
    const query = versionSearch.trim().toLowerCase();
    return versionOptions.filter((entry) => String(entry.displayName || '').toLowerCase().includes(query));
  }, [versionOptions, versionSearch]);
  const visibleVersionOptions = useMemo(() => {
    const visible = filteredVersionOptions.slice(0, MAX_RENDERED_VERSION_OPTIONS);
    if (!selectedVersion || visible.some((entry) => String(entry.key ?? '') === selectedVersionKey)) {
      return visible;
    }

    return [selectedVersion, ...visible.slice(0, Math.max(0, MAX_RENDERED_VERSION_OPTIONS - 1))];
  }, [filteredVersionOptions, selectedVersion, selectedVersionKey]);
  const hiddenVersionOptionsCount = Math.max(0, filteredVersionOptions.length - visibleVersionOptions.length);
  const progressPercent = main.progressPercent ?? hostSnapshot?.update?.progressPercent ?? 0;
  const isProgressIndeterminate = Boolean(main.isProgressIndeterminate || hostSnapshot?.update?.isIndeterminate);
  const isVersionOperationActive = Boolean(main.isVersionOperationActive);
  const progressFillStyle = isProgressIndeterminate ? undefined : { width: `${progressPercent}%` };
  const rightPanelVisible = !loadingState && activeSection !== 'none';
  const isWideSidePanel = !loadingState && WPF_WIDE_PANEL_SECTIONS.has(activeSection);
  const isSettingsWidthSidePanel = !loadingState && SETTINGS_WIDTH_PANEL_SECTIONS.has(activeSection);
  const isLauncherSettingsPanel = !loadingState && (
    activeSection === 'settings' ||
    activeSection === 'friends' ||
    activeSection === 'notifications' ||
    activeSection === 'updates' ||
    activeSection === 'news'
  );

  const panelProps: PanelRenderProps | null = launcher
    ? {
        launcher, accountForm, setAccountForm, setAccountDirty, submitAccount,
        friendDraft, setFriendDraft, setFriendDirty,
        javaPathDraft, setJavaPathDraft, setJavaPathDirty,
        jvmArgsDraft, setJvmArgsDraft, setJvmArgsDirty,
        toggleSelectedMod,
        glassTuning, setGlassTuningValue
      }
    : null;

  const playerDisplayName = String(account.currentNickname || main.nickname || main.usernameText || '').trim();
  const playerFieldText = playerDisplayName || 'Введите ник';
  const updates = launcher?.updates ?? {};

  return (
    <div className="app-shell" style={{ background: pageBackground }}>
      <div className="liquid-glass-snapshot-source" style={{ background: pageBackground }} aria-hidden="true" />
      <div className="procedural-background" />
      <div className="background-vignette" />

      {loadingState ? (
        <LoadingScreen
          hostSnapshot={hostSnapshot}
          runtimeError={runtimeError}
          onRetry={() => {
            setRuntimeError(null);
            photinoBridge.requestSnapshot();
          }}
        />
      ) : (
        <div className="launcher-frame">
          {RESIZE_DIRECTIONS.map((direction) => (
            <span
              key={direction}
              className={`window-resize-zone resize-${direction}`}
              onMouseDown={(event) => startWindowResize(event, direction)}
              onMouseEnter={() => {
                const w = window as any;
                const edge = DIRECTION_TO_WAILS_EDGE[direction];
                if (w.wails?.flags && edge) {
                  w.wails.flags.resizeEdge = edge;
                }
              }}
              onMouseLeave={() => {
                const w = window as any;
                if (w.wails?.flags) {
                  w.wails.flags.resizeEdge = undefined;
                }
              }}
              aria-hidden="true"
            />
          ))}

          <header
            className="titlebar"
            onPointerDown={startWindowDrag}
            onPointerMove={continueWindowDrag}
            onPointerUp={endWindowDrag}
            onPointerCancel={endWindowDrag}
          >
            <div className="titlebar-brand">
              {theme.logoUrl ? <img className="brand-logo" src={theme.logoUrl} alt="Vesper" /> : null}
              <span>Vesper Launcher</span>
            </div>

            <div className="titlebar-actions">
              {updates.isAvailable ? (
                <button
                  className="launcher-updates-button"
                  onClick={handleActionClick(() => openSection('updates'))}
                  type="button"
                  title={`Доступно обновление ${String(updates.version || '')}`}
                >
                  Updates
                </button>
              ) : null}
              <button className="title-account-button" onClick={handleActionClick(() => openSection('account'))} type="button">
                {account.currentNickname || main.nickname || 'Создайте аккаунт'}
              </button>
              <button className="title-icon-button" onClick={handleActionClick(() => photinoBridge.sendCommand('host.minimize'))} type="button" title="Свернуть">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3.2" strokeLinecap="round">
                  <line x1="4" y1="12" x2="20" y2="12" />
                </svg>
              </button>
              <button className="title-icon-button" onClick={handleActionClick(() => photinoBridge.sendCommand('host.close'))} type="button" title="Закрыть">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3.2" strokeLinecap="round">
                  <line x1="6" y1="6" x2="18" y2="18" />
                  <line x1="18" y1="6" x2="6" y2="18" />
                </svg>
              </button>
            </div>
          </header>

          <div className="classic-workspace">
            <div className="classic-main-grid">
              <aside className={`left-control-surface ${versionMenuOpen ? 'version-menu-active' : ''}`}>
                <div className="left-control-surface-bg glass-panel" />
                <div className="left-panel-content">
                  <div className="left-panel-top">
                    {theme.wordmarkUrl ? <img className="left-wordmark" src={theme.wordmarkUrl} alt="Vesper Launcher" /> : null}
                    <p className="launcher-subtitle">Minecraft Java Launcher</p>

                    <div className="classic-field-block">
                      <label className="classic-label">Игрок</label>
                      <div className="classic-inline-row">
                        <button className="static-glass-control text-left left-liquid-glass-button" data-liquid-label={playerFieldText} onClick={handleActionClick(() => openSection('account'))} type="button">
                          <span className="left-liquid-glass-layer liquid-glass-layer" aria-hidden="true" />
                          <span className="left-liquid-glass-content">{playerFieldText}</span>
                        </button>
                        <button className="input-icon-button left-liquid-glass-button" data-liquid-icon={'@'} onClick={handleActionClick(() => openSection('account'))} type="button" title="Меню игрока">
                          <span className="left-liquid-glass-layer liquid-glass-layer" aria-hidden="true" />
                          <span className="safe-icon left-liquid-glass-content">
                            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.8" strokeLinecap="round" strokeLinejoin="round">
                              <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
                              <circle cx="12" cy="7" r="4" />
                            </svg>
                          </span>
                        </button>
                      </div>
                    </div>

                    <div className="classic-field-block version-block">
                      <div className="classic-inline-row">
                        <div className={`version-picker ${versionMenuOpen ? 'open' : ''}`}>
                          <span
                            className={`version-picker-morph-glass left-liquid-glass-layer liquid-glass-layer ${versionMenuOpen ? 'is-expanded' : ''}`}
                            style={versionMenuOpen ? versionMenuStyle : undefined}
                            aria-hidden="true"
                          />
                          <div
                            ref={versionPickerToggleRef}
                            className="version-picker-toggle left-liquid-glass-button"
                            data-liquid-label={versionMenuOpen ? versionSearch : selectedVersionLabel}
                          >
                            <span className="left-liquid-glass-layer liquid-glass-layer" aria-hidden="true" />
                            <input
                              className="version-picker-main-input left-liquid-glass-content"
                              type="text"
                              value={versionMenuOpen ? versionSearch : selectedVersionLabel}
                              placeholder={selectedVersionLabel}
                              onFocus={() => setVersionMenuOpen(true)}
                              onChange={(event) => {
                                setVersionSearch(event.target.value);
                                if (!versionMenuOpen) {
                                  setVersionMenuOpen(true);
                                }
                              }}
                              onKeyDown={(event) => {
                                if (event.key === 'Escape') {
                                  setVersionMenuOpen(false);
                                  setVersionSearch('');
                                } else if (event.key === 'Enter' && visibleVersionOptions.length > 0) {
                                  const target = visibleVersionOptions[0];
                                  const key = String(target.key ?? '');
                                  const isTargetInstalled = !!target.isInstalled;
                                  setVersionMenuOpen(false);
                                  setVersionSearch('');
                                  setHostSnapshot((prev) => (prev && prev.launcher ? {
                                    ...prev,
                                    launcher: {
                                      ...prev.launcher,
                                      main: {
                                        ...prev.launcher.main,
                                        selectedVersionKey: key,
                                        launchButtonText: isTargetInstalled ? 'Играть' : 'Установить'
                                      }
                                    }
                                  } : prev));
                                  photinoBridge.sendCommand('main.selectVersionKey', { key });
                                }
                              }}
                            />
                            <button
                              className="select-chevron-button left-liquid-glass-content"
                              type="button"
                              aria-label="Открыть меню версий"
                              onClick={handleActionClick(() => {
                                setVersionMenuOpen((open) => {
                                  if (open) setVersionSearch('');
                                  return !open;
                                });
                              })}
                            >
                              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3.6" strokeLinecap="round" strokeLinejoin="round">
                                <polyline points="6 9 12 15 18 9" />
                              </svg>
                            </button>
                          </div>

                          <div
                            className={`version-picker-menu ${versionMenuOpen ? 'is-open' : 'is-closed'}`}
                            role="listbox"
                            aria-label="Выбор версии Minecraft"
                            aria-hidden={!versionMenuOpen}
                            style={versionMenuStyle}
                          >
                            <div className="version-picker-scroll">
                              {filteredVersionOptions.length === 0 ? (
                                <span className="version-picker-empty">Версия «{versionSearch}» не найдена</span>
                              ) : null}
                              {visibleVersionOptions.map((entry) => {
                                const key = String(entry.key ?? '');
                                const isSelected = key === selectedVersionKey;
                                const isInstalled = !!entry.isInstalled;

                                return (
                                  <button
                                    key={key}
                                    className={`version-picker-option ${isSelected ? 'selected' : ''}`}
                                    type="button"
                                    role="option"
                                    aria-selected={isSelected}
                                    onClick={handleActionClick(() => {
                                      setVersionMenuOpen(false);
                                      setVersionSearch('');
                                      setHostSnapshot((prev) => (prev && prev.launcher ? {
                                        ...prev,
                                        launcher: {
                                          ...prev.launcher,
                                          main: {
                                            ...prev.launcher.main,
                                            selectedVersionKey: key,
                                            launchButtonText: isInstalled ? 'Играть' : 'Установить'
                                          }
                                        }
                                      } : prev));
                                      photinoBridge.sendCommand('main.selectVersionKey', { key });
                                    })}
                                  >
                                    <span className="version-picker-option-label">
                                      <span>{entry.displayName}</span>
                                    </span>
                                    <div className="version-picker-option-actions">
                                      {isInstalled ? (
                                        <button
                                          className="version-delete-button"
                                          type="button"
                                          title="Удалить версию"
                                          aria-label={`Удалить версию ${entry.displayName}`}
                                          onClick={(event) => {
                                            event.stopPropagation();
                                            setHostSnapshot((prev) => {
                                              if (!prev?.launcher?.main?.versions) return prev;
                                              const updated = prev.launcher.main.versions.map((opt: any) =>
                                                String(opt.key) === key ? { ...opt, isInstalled: false } : opt
                                              );
                                              const isCurrent = key === prev.launcher.main.selectedVersionKey;
                                              return {
                                                ...prev,
                                                launcher: {
                                                  ...prev.launcher,
                                                  main: {
                                                    ...prev.launcher.main,
                                                    versions: updated,
                                                    availableVersions: updated,
                                                    launchButtonText: isCurrent ? 'Установить' : prev.launcher.main.launchButtonText
                                                  }
                                                }
                                              };
                                            });
                                            photinoBridge.sendCommand('main.deleteVersion', { key });
                                          }}
                                        >
                                          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
                                            <polyline points="3 6 5 6 21 6" />
                                            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                                            <line x1="10" y1="11" x2="10" y2="17" />
                                            <line x1="14" y1="11" x2="14" y2="17" />
                                          </svg>
                                        </button>
                                      ) : null}
                                      {isSelected ? (
                                        <span className="version-picker-option-check" aria-hidden="true">
                                          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                                            <polyline points="5 12 10 17 19 7" />
                                          </svg>
                                        </span>
                                      ) : null}
                                    </div>
                                  </button>
                                );
                              })}
                              {hiddenVersionOptionsCount > 0 ? (
                                <span className="version-picker-empty">Показано {visibleVersionOptions.length} из {filteredVersionOptions.length}. Уточните поиск.</span>
                              ) : null}
                            </div>
                          </div>
                        </div>

                        <div className="select-shell static-glass-control">
                          <select value={main.selectedVersionKey ?? ''} onChange={(e) => photinoBridge.sendCommand('main.selectVersionKey', { key: e.target.value })}>
                            <option value="">{main.inlineVersionLabel || 'Версия: не выбрана'}</option>
                            {visibleVersionOptions.map((entry) => (
                              <option key={String(entry.key)} value={String(entry.key)}>{entry.displayName}</option>
                            ))}
                          </select>
                          <span className="select-chevron">
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3.6" strokeLinecap="round" strokeLinejoin="round">
                              <polyline points="6 9 12 15 18 9" />
                            </svg>
                          </span>
                        </div>
                        <button className="input-icon-button left-liquid-glass-button" data-liquid-icon={'#'} onClick={handleActionClick(() => photinoBridge.sendCommand('main.openProfileFolder'))} type="button" title="Папка профиля">
                          <span className="left-liquid-glass-layer liquid-glass-layer" aria-hidden="true" />
                          <span className="safe-icon left-liquid-glass-content">
                            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.8" strokeLinecap="round" strokeLinejoin="round">
                              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
                            </svg>
                          </span>
                        </button>
                      </div>

                      <div className="classic-inline-row launch-row-classic">
                        {(() => {
                          const selectedOption = (main.versions || []).find((v: any) => v.key === selectedVersionKey);
                          const isInstalled = selectedOption ? !!selectedOption.isInstalled : false;
                          const launchText = isVersionOperationActive
                            ? (isInstalled ? 'Запуск...' : 'Установка...')
                            : (main.launchButtonText || (isInstalled ? 'Играть' : 'Установить'));

                          return (
                            <button
                              className={`main-glass-button launch-button-classic left-liquid-glass-button ${isVersionOperationActive ? 'is-busy' : ''}`}
                              data-liquid-label={launchText}
                              onClick={handleActionClick(() => photinoBridge.sendCommand('main.launch'))}
                              type="button"
                              disabled={isVersionOperationActive}
                            >
                              <span className="left-liquid-glass-layer liquid-glass-layer" aria-hidden="true" />
                              <span className={`launch-button-progress ${isProgressIndeterminate ? 'indeterminate' : ''}`} style={progressFillStyle} />
                              <span className="left-liquid-glass-content launch-button-content">
                                {isVersionOperationActive ? <span className="launch-operation-spinner" aria-hidden="true" /> : null}
                                <span>{launchText}</span>
                              </span>
                            </button>
                          );
                        })()}
                        <button ref={settingsLiquidButtonRef} className="input-icon-button settings-liquid-button left-liquid-glass-button" data-liquid-ignore="" data-liquid-icon={'*'} onClick={handleActionClick(() => openSection('settings'))} type="button" title="Настройки">
                          <span className="settings-liquid-glass-test left-liquid-glass-layer liquid-glass-layer" aria-hidden="true" />
                          <span className="safe-icon settings-liquid-button-icon left-liquid-glass-content">
                            <svg width="21" height="21" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.8" strokeLinecap="round" strokeLinejoin="round">
                              <circle cx="12" cy="12" r="3" />
                              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
                            </svg>
                          </span>
                        </button>
                      </div>
                    </div>
                  </div>

                  <div className="left-panel-bottom">
                    {(() => {
                      const rawStatus = String(main.statusText || runtimeError || hostSnapshot?.update?.message || '').trim();
                      const displayStatus =
                        rawStatus.includes('host.') ||
                        rawStatus.includes('background.') ||
                        rawStatus.includes('shell.') ||
                        rawStatus.includes('main.') ||
                        rawStatus.includes('сборки для разработки') ||
                        rawStatus === 'Готов к запуску'
                          ? ''
                          : rawStatus;
                      const rawProgress = String(main.progressText || hostSnapshot?.update?.detailMessage || '').trim();
                      const displayProgress =
                        rawProgress.includes('host.') ||
                        rawProgress.includes('background.') ||
                        rawProgress.includes('shell.') ||
                        rawProgress.includes('main.') ||
                        rawProgress.includes('сборки для разработки')
                          ? ''
                          : rawProgress;
                      const showProgressBar = isVersionOperationActive || isProgressIndeterminate || (progressPercent > 0 && progressPercent < 100);
                      const langCode = String(launcher?.settings?.minecraftLanguageCode || navigator.language || 'ru').toLowerCase();
                      const newsButtonLabel = langCode.startsWith('ru') || langCode === 'auto' ? 'Новости' : 'NEWS';
                      const shouldShowStatus = (showProgressBar || runtimeError || isVersionOperationActive) && !!displayStatus;

                      return (
                        <>
                          {shouldShowStatus ? <p className="status-text">{displayStatus}</p> : null}
                          {showProgressBar && displayProgress ? <p className="progress-text">{displayProgress}</p> : null}
                          {showProgressBar ? (
                            <div className="classic-progress">
                              <div className={`classic-progress-fill ${isProgressIndeterminate ? 'indeterminate' : ''}`} style={progressFillStyle} />
                              <span>{main.progressOverlayText || hostSnapshot?.update?.progressText || ''}</span>
                            </div>
                          ) : null}
                          <div className="left-panel-quick-actions">
                            <button
                              className={`quick-news-button ${activeSection === 'news' ? 'active' : ''}`}
                              onClick={handleActionClick(() => openSection('news'))}
                              type="button"
                              title={newsButtonLabel}
                            >
                              <span>{newsButtonLabel}</span>
                            </button>
                            <div className="social-links-group">
                              <button
                                className="social-icon-button social-tg"
                                onClick={handleActionClick(() => {
                                  photinoBridge.sendCommand('shell.openexternalurl', { url: 'https://t.me/VesperLauncher' });
                                  window.open('https://t.me/VesperLauncher', '_blank');
                                })}
                                type="button"
                                title="Telegram Vesper"
                              >
                                <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                                  <path d="M9.78 18.65l.28-4.2 7.63-6.88c.34-.3-.07-.46-.52-.18L7.7 13.47l-4.08-1.28c-.89-.28-.9-.88.19-1.31L19.78 4.63c.74-.28 1.39.18 1.15 1.31L18.1 19.38c-.23 1.07-.86 1.33-1.74.83l-4.88-3.6-2.36 2.27c-.26.26-.48.47-.98.47z" />
                                </svg>
                              </button>
                              <button
                                className="social-icon-button social-discord"
                                onClick={handleActionClick(() => {
                                  photinoBridge.sendCommand('shell.openexternalurl', { url: 'https://discord.gg/P4Ayp2Rsb' });
                                  window.open('https://discord.gg/P4Ayp2Rsb', '_blank');
                                })}
                                type="button"
                                title="Discord Vesper"
                              >
                                <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
                                  <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028c.462-.63.874-1.295 1.226-1.994.021-.041.001-.09-.041-.106a13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.061 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.028zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z" />
                                </svg>
                              </button>
                            </div>
                          </div>
                        </>
                      );
                    })()}
                  </div>
                </div>
              </aside>

              <aside className={`sidepanel classic-side-panel glass-panel ${rightPanelVisible ? 'visible' : ''} ${isWideSidePanel ? 'wide-section-panel' : ''} ${isSettingsWidthSidePanel ? 'settings-section-panel' : ''} ${isLauncherSettingsPanel ? 'launcher-settings-side-panel' : ''}`}>
                {panelProps ? renderPanel(panelProps) : null}
              </aside>

              <nav className="bottom-action-bar">
                <div className="bottom-action-bar-bg glass-panel" />
                {BOTTOM_SECTION_ITEMS.map((section) => (
                  <button
                    key={section.id}
                    className={`main-glass-button ${launcher?.activeSection === section.id ? 'active' : ''} left-liquid-glass-button`}
                    data-liquid-label={section.label}
                    onClick={handleActionClick(() => openSection(section.id))}
                    type="button"
                  >
                    <span className="left-liquid-glass-layer liquid-glass-layer" aria-hidden="true" />
                    <span className="left-liquid-glass-content">{section.label}</span>
                  </button>
                ))}
              </nav>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function renderPanel(props: PanelRenderProps) {
  switch (props.launcher.activeSection) {
    case 'account': return <AccountPanel {...props} />;
    case 'settings': return <SettingsPanel {...props} />;
    case 'skin': return <SkinPanel {...props} />;
    case 'background': return <BackgroundPanel {...props} />;
    case 'mods': return <ModsPanel {...props} />;
    case 'friends': return <FriendsPanel {...props} />;
    case 'notifications': return <NotificationsPanel {...props} />;
    case 'updates': return <UpdatesPanel launcher={props.launcher} />;
    case 'news': return <NewsPanel launcher={props.launcher} />;
    default: return <EmptyPanel />;
  }
}

export default App;

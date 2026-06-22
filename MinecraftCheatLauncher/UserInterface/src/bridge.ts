import type { HostEnvelope } from './types';

type Listener = (message: HostEnvelope) => void;

declare global {
  interface External {
    sendMessage?: (message: string) => void;
    receiveMessage?: (message: string) => void;
  }

  interface Window {
    chrome?: {
      webview?: {
        addEventListener?: (name: string, handler: (event: { data: unknown }) => void) => void;
      };
    };
    photinoReceiveMessage?: (message: string | HostEnvelope) => void;
  }
}

class PhotinoBridge {
  private listeners = new Set<Listener>();
  private initialized = false;

  subscribe(listener: Listener) {
    this.ensureInitialized();
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  requestSnapshot() {
    this.sendCommand('host.requestSnapshot');
  }

  sendCommand(command: string, payload: Record<string, unknown> = {}) {
    this.ensureInitialized();
    const message = JSON.stringify({ type: 'command', command, payload });
    window.external?.sendMessage?.(message);
  }

  private ensureInitialized() {
    if (this.initialized) {
      return;
    }

    this.initialized = true;
    const handleRawMessage = (raw: unknown) => {
      const envelope = this.parseEnvelope(raw);
      if (!envelope) {
        return;
      }

      this.listeners.forEach((listener) => listener(envelope));
    };

    const existingReceive = window.external?.receiveMessage;
    if (window.external) {
      window.external.receiveMessage = (message: string) => {
        existingReceive?.(message);
        handleRawMessage(message);
      };
    }

    window.photinoReceiveMessage = handleRawMessage;
    window.addEventListener('message', (event) => handleRawMessage(event.data));
    document.addEventListener('message', ((event: Event) => {
      const customEvent = event as CustomEvent<unknown>;
      handleRawMessage(customEvent.detail);
    }) as EventListener);
    window.chrome?.webview?.addEventListener?.('message', (event) => handleRawMessage(event.data));
  }

  private parseEnvelope(raw: unknown): HostEnvelope | null {
    if (!raw) {
      return null;
    }

    if (typeof raw === 'object' && raw !== null && 'type' in (raw as Record<string, unknown>)) {
      return raw as HostEnvelope;
    }

    if (typeof raw !== 'string') {
      return null;
    }

    try {
      return JSON.parse(raw) as HostEnvelope;
    } catch {
      return null;
    }
  }
}

export const photinoBridge = new PhotinoBridge();

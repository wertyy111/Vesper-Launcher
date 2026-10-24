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
    if (this.sendNativeWindowCommand(command, message)) {
      return;
    }

    void this.sendMessageToHost(message, command, payload);
  }

  private ensureInitialized() {
    if (this.initialized) {
      return;
    }

    this.initialized = true;
    const existingReceive = window.external?.receiveMessage;
    if (window.external) {
      window.external.receiveMessage = (message: string) => {
        existingReceive?.(message);
        this.dispatchRawMessage(message);
      };
    }

    window.photinoReceiveMessage = (message) => this.dispatchRawMessage(message);
    window.addEventListener('message', (event) => this.dispatchRawMessage(event.data));
    document.addEventListener('message', ((event: Event) => {
      const customEvent = event as CustomEvent<unknown>;
      this.dispatchRawMessage(customEvent.detail);
    }) as EventListener);
    window.chrome?.webview?.addEventListener?.('message', (event) => this.dispatchRawMessage(event.data));

    // Wails v2 runtime integration with retry loop to handle async runtime initialization
    const tryBindWails = () => {
      const wailsRuntime = (window as any).runtime;
      if (wailsRuntime?.EventsOn) {
        wailsRuntime.EventsOn('vesper:message', (data: any) => this.dispatchRawMessage(data));
        wailsRuntime.EventsOn('snapshot', (data: any) => this.dispatchRawMessage({ type: 'snapshot', data }));
        return true;
      }
      return false;
    };

    if (!tryBindWails()) {
      let attempts = 0;
      const interval = setInterval(() => {
        attempts++;
        if (tryBindWails() || attempts > 60) {
          clearInterval(interval);
        }
      }, 50);
    }
  }

  private async sendMessageToHost(message: string, command: string, payload: Record<string, unknown>) {
    const wailsApp = (window as any).go?.main?.App;
    if (wailsApp?.SendCommand) {
      try {
        await wailsApp.SendCommand(command, payload);
        return;
      } catch (err) {
        console.warn('Wails SendCommand failed:', err);
      }
    }
    if (wailsApp?.SendMessage) {
      try {
        await wailsApp.SendMessage(message);
        return;
      } catch (err) {
        console.warn('Wails SendMessage failed:', err);
      }
    }

    const wailsRuntime = (window as any).runtime;
    if (wailsRuntime?.EventsEmit) {
      try {
        const parsed = JSON.parse(message);
        wailsRuntime.EventsEmit('vesper:command', parsed);
        return;
      } catch (err) {
        console.warn('Wails EventsEmit failed:', err);
      }
    }

    if (await this.trySendHttpBridgeMessage(message)) {
      return;
    }

    window.external?.sendMessage?.(message);
  }

  private sendNativeWindowCommand(command: string, message: string) {
    const normalizedCommand = command.trim().toLowerCase();
    if (normalizedCommand !== 'host.startdrag' && normalizedCommand !== 'host.startresize') {
      return false;
    }

    if (!window.external?.sendMessage) {
      return false;
    }

    window.external.sendMessage(message);
    return true;
  }

  private async trySendHttpBridgeMessage(message: string) {
    try {
      const response = await fetch('/bridge-message', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: message
      });

      if (!response.ok) {
        return false;
      }

      const text = await response.text();
      this.dispatchRawMessage(text);
      return true;
    } catch {
      return false;
    }
  }

  private dispatchRawMessage(raw: unknown) {
    const envelope = this.parseEnvelope(raw);
    if (!envelope) {
      return;
    }

    this.listeners.forEach((listener) => listener(envelope));
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

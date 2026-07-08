import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

afterEach(() => cleanup());

// Minimal stubs for the Wails runtime + Go bindings so any module importing
// files under frontend/wailsjs/ can be loaded (and called) under jsdom.
type Listener = (...data: unknown[]) => void;
const listeners = new Map<string, Listener[]>();

(window as unknown as Record<string, unknown>).runtime = {
  EventsOnMultiple(eventName: string, callback: Listener, _maxCallbacks: number) {
    const arr = listeners.get(eventName) ?? [];
    arr.push(callback);
    listeners.set(eventName, arr);
    return () => listeners.delete(eventName);
  },
  EventsOff(eventName: string) {
    listeners.delete(eventName);
  },
  EventsEmit(eventName: string, ...data: unknown[]) {
    for (const cb of listeners.get(eventName) ?? []) cb(...data);
  },
};

(window as unknown as Record<string, unknown>).go = {
  main: {
    // Every bound method resolves to null; tests drive the stores through the
    // exported event handlers instead of real backend calls.
    App: new Proxy({}, { get: () => () => Promise.resolve(null) }),
  },
};

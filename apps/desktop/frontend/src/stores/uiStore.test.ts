import { beforeEach, describe, expect, it } from 'vitest';
import { useUiStore } from './uiStore';

beforeEach(() => {
  useUiStore.setState({ view: 'smart-scan', activeCategoryId: undefined, fda: null, config: undefined });
});

describe('uiStore view transitions', () => {
  it('starts on smart-scan with unknown FDA', () => {
    expect(useUiStore.getState().view).toBe('smart-scan');
    expect(useUiStore.getState().fda).toBeNull();
  });

  it('setView switches view and carries the active category id', () => {
    useUiStore.getState().setView('category', 'trash');
    expect(useUiStore.getState().view).toBe('category');
    expect(useUiStore.getState().activeCategoryId).toBe('trash');
  });

  it('setView without a category clears activeCategoryId', () => {
    useUiStore.getState().setView('category', 'trash');
    useUiStore.getState().setView('settings');
    expect(useUiStore.getState().view).toBe('settings');
    expect(useUiStore.getState().activeCategoryId).toBeUndefined();
  });

  it('can enter and leave first-run', () => {
    useUiStore.getState().setView('first-run');
    expect(useUiStore.getState().view).toBe('first-run');
    useUiStore.getState().setView('smart-scan');
    expect(useUiStore.getState().view).toBe('smart-scan');
  });
});

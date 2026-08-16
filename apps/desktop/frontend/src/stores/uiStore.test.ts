import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../wailsjs/go/main/App', () => ({
  CheckFDA: vi.fn().mockResolvedValue(null),
  GetConfig: vi.fn().mockResolvedValue(null),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  CheckForUpdate: vi.fn(),
}));

import { CheckForUpdate } from '../../wailsjs/go/main/App';
import { useUiStore } from './uiStore';

const CheckForUpdateMock = CheckForUpdate as unknown as ReturnType<typeof vi.fn>;

beforeEach(() => {
  useUiStore.setState({
    view: 'smart-scan',
    activeCategoryId: undefined,
    fda: null,
    config: undefined,
    update: undefined,
  });
  CheckForUpdateMock.mockReset();
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

describe('uiStore update check', () => {
  it('stores the UpdateInfo returned by CheckForUpdate', async () => {
    const info = { current: '1.4.0', latest: '2.0.0', available: true, url: 'https://x' };
    CheckForUpdateMock.mockResolvedValue(info);
    const got = await useUiStore.getState().checkUpdate();
    expect(got).toEqual(info);
    expect(useUiStore.getState().update).toEqual(info);
  });

  it('leaves update untouched when the binding rejects or resolves null', async () => {
    CheckForUpdateMock.mockRejectedValue(new Error('no bridge'));
    expect(await useUiStore.getState().checkUpdate()).toBeNull();
    expect(useUiStore.getState().update).toBeUndefined();

    CheckForUpdateMock.mockResolvedValue(null); // test-harness stub shape
    expect(await useUiStore.getState().checkUpdate()).toBeNull();
    expect(useUiStore.getState().update).toBeUndefined();
  });
});

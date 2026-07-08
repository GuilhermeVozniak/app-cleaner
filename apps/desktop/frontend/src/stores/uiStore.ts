import { create } from 'zustand';
import { CheckFDA, GetConfig, SaveConfig } from '../../wailsjs/go/main/App';
import type { Config } from '../lib/types';

export type View =
  | 'smart-scan'
  | 'category'
  | 'uninstaller'
  | 'maintenance'
  | 'backups'
  | 'settings'
  | 'first-run';

export interface UiState {
  view: View;
  activeCategoryId?: string;
  fda: boolean | null;
  config?: Config;
  setView: (view: View, activeCategoryId?: string) => void;
  loadConfig: () => Promise<void>;
  saveConfig: (c: Config) => Promise<void>;
  refreshFda: () => Promise<boolean | null>;
}

export const useUiStore = create<UiState>((set) => ({
  view: 'smart-scan',
  activeCategoryId: undefined,
  fda: null,
  config: undefined,

  setView: (view, activeCategoryId) => set({ view, activeCategoryId }),

  loadConfig: async () => {
    const config = (await GetConfig()) as Config | null;
    if (config) set({ config });
  },

  saveConfig: async (c) => {
    // SaveConfig's generated param type (wailsjs/go/models config.Config) carries an
    // instance `convertValues` method (added by the Wails generator because Config has
    // a nested ExtraPaths field) that our plain lib/types Config intentionally omits.
    // Wails marshals call args to JSON regardless, so this cast is safe — it exists only
    // to bridge the generated class shape with our plain data type at the call boundary.
    await SaveConfig(c as unknown as Parameters<typeof SaveConfig>[0]);
    set({ config: c });
  },

  refreshFda: async () => {
    const fda = (await CheckFDA()) as boolean | null;
    set({ fda });
    return fda;
  },
}));

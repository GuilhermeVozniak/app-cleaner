import { useEffect } from 'react';
import { CheckFDA } from '../wailsjs/go/main/App';
import Sidebar from './components/Sidebar';
import FirstRun from './views/FirstRun';
import SmartScan from './views/SmartScan';
import { useUiStore } from './stores/uiStore';

// Placeholder panes — swapped for real views by later tasks:
// SmartScan (Task 23), CategoryDetail (Task 24), Uninstaller/Maintenance/Backups/Settings (later tasks).
function Placeholder({ title }: { title: string }) {
  return (
    <div className="flex h-full items-center justify-center text-neutral-400 dark:text-neutral-500">
      {title}
    </div>
  );
}

function App() {
  const view = useUiStore((s) => s.view);

  useEffect(() => {
    void useUiStore.getState().loadConfig();
    // First-run gate (spec §12): shown unless FDA is confirmed granted —
    // false (denied) AND null (unknown) both land on the permission screen.
    // "Continue without" on that screen still lets the user proceed.
    void CheckFDA().then((v) => {
      const fda = v as boolean | null;
      useUiStore.setState({ fda });
      if (fda !== true) useUiStore.getState().setView('first-run');
    });
  }, []);

  if (view === 'first-run') {
    return (
      <div className="h-full bg-white dark:bg-neutral-900">
        <FirstRun />
      </div>
    );
  }

  return (
    <div className="flex h-full bg-white text-neutral-900 dark:bg-neutral-900 dark:text-neutral-100">
      <Sidebar />
      <main className="min-w-0 flex-1 overflow-y-auto">
        {view === 'smart-scan' && <SmartScan />}
        {view === 'category' && <Placeholder title="Category" />}
        {view === 'uninstaller' && <Placeholder title="Uninstaller" />}
        {view === 'maintenance' && <Placeholder title="Maintenance" />}
        {view === 'backups' && <Placeholder title="Backups" />}
        {view === 'settings' && <Placeholder title="Settings" />}
      </main>
    </div>
  );
}

export default App;

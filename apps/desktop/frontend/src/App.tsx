import { useEffect, type CSSProperties } from 'react';
import { CheckFDA } from '../wailsjs/go/main/App';
import { Backdrop } from './components/Backdrop';
import Sidebar from './components/Sidebar';
import { TitleBar } from './components/TitleBar';
import FirstRun from './views/FirstRun';
import Dashboard from './views/Dashboard';
import SmartScan from './views/SmartScan';
import CategoryDetail from './views/CategoryDetail';
import Uninstaller from './views/Uninstaller';
import Maintenance from './views/Maintenance';
import Backups from './views/Backups';
import Settings from './views/Settings';
import MyTools from './views/MyTools';
import SpaceLens from './views/SpaceLens';
import LoginItems from './views/LoginItems';
import { CleanFlow } from './components/CleanFlow';
import { MODULES, identityForView } from './lib/modules';
import { useUiStore } from './stores/uiStore';

function App() {
  const view = useUiStore((s) => s.view);
  const identity = view === 'first-run' ? MODULES[0] : identityForView(view);

  // Dialogs and tooltips portal to <body>, outside the shell's --module scope,
  // so the current module hue is mirrored onto the document root as well.
  useEffect(() => {
    document.documentElement.style.setProperty('--module', identity.hue);
  }, [identity.hue]);

  useEffect(() => {
    void useUiStore.getState().loadConfig();
    void useUiStore.getState().checkUpdate();
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
      <div className="relative h-full text-ink" style={{ '--module': MODULES[0].hue } as CSSProperties}>
        <Backdrop canvas={MODULES[0].canvas} />
        <TitleBar title="" />
        <div className="relative z-10 h-full">
          <FirstRun />
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full text-ink" style={{ '--module': identity.hue } as CSSProperties}>
      <Backdrop canvas={identity.canvas} />
      <TitleBar title={identity.label} />
      <Sidebar />
      <main className="relative z-10 min-w-0 flex-1 overflow-y-auto pt-11">
        {view === 'dashboard' && <Dashboard />}
        {view === 'smart-scan' && <SmartScan />}
        {view === 'category' && <CategoryDetail />}
        {view === 'uninstaller' && <Uninstaller />}
        {view === 'maintenance' && <Maintenance />}
        {view === 'backups' && <Backups />}
        {view === 'settings' && <Settings />}
        {view === 'my-tools' && <MyTools />}
        {view === 'space-lens' && <SpaceLens />}
        {view === 'login-items' && <LoginItems />}
      </main>
      <CleanFlow />
    </div>
  );
}

export default App;

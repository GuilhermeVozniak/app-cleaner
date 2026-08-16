export type SafetyLevel = 'safe' | 'moderate' | 'risky';
export type CategoryGroup =
  | 'System Junk'
  | 'Development'
  | 'Storage'
  | 'Browsers'
  | 'Large Files';

export interface Category {
  id: string;
  name: string;
  group: CategoryGroup;
  description: string;
  safetyLevel: SafetyLevel;
  safetyNote?: string;
  supportsFileSelection?: boolean;
}

export interface CleanableItem {
  path: string;
  size: number;
  name: string;
  isDirectory: boolean;
  modifiedAt?: string; // RFC3339 from Go *time.Time
}

export interface ScanResult {
  category: Category;
  items: CleanableItem[];
  totalSize: number;
  error?: string;
}

export interface ScanSummary {
  results: ScanResult[];
  totalSize: number;
  totalItems: number;
}

export interface CleanResult {
  category: Category;
  cleanedItems: number;
  freedSpace: number;
  errors: string[];
}

export interface CleanSummary {
  results: CleanResult[];
  totalFreedSpace: number;
  totalCleanedItems: number;
  totalErrors: number;
}

export interface DisplayRow {
  type: 'directory-header' | 'file' | 'expand-hint';
  directoryKey: string;
  displayName: string;
  path?: string;
  size?: number;
  name?: string;
  hiddenCount?: number;
  totalFilesInDir: number;
  selectable: boolean;
}

export interface RelatedPath {
  path: string;
  size: number;
}

export interface AppInfo {
  name: string;
  path: string;
  bundleId: string;
  appSize: number;
  relatedPaths: RelatedPath[];
  totalSize: number;
  running: boolean;
}

export interface BackupInfo {
  path: string;
  date: string; // RFC3339 from Go time.Time
  size: number;
}

export interface BackupItem {
  path: string;
  name: string;
  size: number;
}

export interface BackupDetails {
  items: BackupItem[];
  fromManifest: boolean;
  truncated: number;
}

export interface RestoreResult {
  restored: number;
  failed: number;
  errors: string[];
}

export interface MaintenanceResult {
  success: boolean;
  message: string;
  error?: string;
  requiresAdmin: boolean;
}

export interface ExtraPaths {
  nodeModules: string[];
  projects: string[];
}

export interface Config {
  downloadsDaysOld: number;
  largeFilesMinSize: number;
  backupByDefault: boolean;
  backupRetentionDays: number;
  concurrency: number;
  showRisky: boolean;
  keepLanguages: string[];
  extraPaths: ExtraPaths;
}

// ---- Wails event payloads (exact keys from the contract's Events section) ----

export interface ScanProgressEvent {
  completed: number;
  total: number;
  categoryId: string;
  totalSize: number;
  itemCount: number;
  error?: string;
}

export interface ScanDoneEvent {
  summary: ScanSummary;
  cancelled?: boolean;
  error?: string;
}

export interface CleanProgressEvent {
  current: number;
  total: number;
  categoryId: string;
  itemName: string;
}

/** clean:done payload — summary = core.CleanSummary JSON; notBackedUp is a TOP-LEVEL sibling of summary. */
export interface CleanDoneEvent {
  summary: CleanSummary;
  notBackedUp: string[];
  cancelled?: boolean;
  error?: string;
}

export interface UninstallProgressEvent {
  current: number;
  total: number;
  appName: string;
}

export interface UninstallDoneEvent {
  uninstalled: number;
  freedSpace: number;
  errors: string[];
  cancelled?: boolean;
  error?: string;
}

export interface BackupProgressEvent {
  current: number;
  total: number;
  itemName: string;
}

export interface MaintenanceProgressEvent {
  done: number;
  total: number;
  date: string;
  error?: string;
}

export interface MaintenanceDoneEvent {
  result: MaintenanceResult;
}

export interface DiskUsage {
  total: number;
  free: number;
  used: number;
}

export interface ActivityStats {
  totalCleanedBytes: number;
  totalCleanedItems: number;
  cleanRuns: number;
  scanRuns: number;
  appsUninstalled: number;
  lastCleanAt: string; // RFC3339, '' = never
}

export interface SpaceLensNode {
  name: string;
  path: string;
  size: number;
  isDir: boolean;
  children?: SpaceLensNode[];
  truncated?: number;
}

export interface LoginItem {
  label: string;
  path: string;
  program: string;
  kind: 'user-agent' | 'global-agent' | 'daemon';
  runAtLoad: boolean;
  programMissing: boolean;
}

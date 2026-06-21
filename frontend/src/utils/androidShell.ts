export interface AndroidShellPlugin {
  status?: () => Promise<unknown>;
  check?: (options: { manual: boolean }) => Promise<{ message?: string }>;
  refreshWidgets?: () => Promise<{ ok?: boolean }>;
}

interface CapacitorWindow {
  Plugins?: Record<string, unknown>;
  isNativePlatform?: () => boolean;
  getPlatform?: () => string;
}

function capacitorRuntime(): CapacitorWindow | null {
  return (window as unknown as { Capacitor?: CapacitorWindow }).Capacitor ?? null;
}

export function isAndroidShell(): boolean {
  const capacitor = capacitorRuntime();
  if (!capacitor) {
    return false;
  }
  if (typeof capacitor.isNativePlatform === "function") {
    return capacitor.isNativePlatform();
  }
  return typeof capacitor.getPlatform === "function" && capacitor.getPlatform() === "android";
}

export function getAndroidShellPlugin(): AndroidShellPlugin | null {
  if (!isAndroidShell()) {
    return null;
  }
  const plugin = capacitorRuntime()?.Plugins?.AndroidUpdate as AndroidShellPlugin | undefined;
  return plugin ?? null;
}

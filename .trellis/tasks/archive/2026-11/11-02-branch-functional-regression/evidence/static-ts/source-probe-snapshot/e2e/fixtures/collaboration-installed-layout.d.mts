export interface NativeInstalledLayout {
  resources: string;
  asar: string;
  cli: string;
  rendererUrl: string;
  platform: "darwin" | "win32";
  apiOS: "macos" | "windows";
}
export function nativeInstalledLayout(executable: string, platform?: string): NativeInstalledLayout;
export function nativeAcceptanceHome(home: string | undefined, platform?: string): string;

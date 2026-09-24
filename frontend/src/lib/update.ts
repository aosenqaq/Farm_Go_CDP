import type { license } from '../../wailsjs/go/models';

// Wails returns JSON-shaped update data, while its generated class includes an internal converter method.
export type UpdateStateDto = Omit<license.UpdateState, 'convertValues'>;

export type UpdateCheckPreferencesDto = {
  enabled: boolean;
  intervalMinutes: number;
};

export function getValidatedUpdateDownloadURL(update: UpdateStateDto | null | undefined) {
  const value = update?.available ? update.latest?.downloadUrl?.trim() || '' : '';
  try {
    const url = new URL(value);
    return (url.protocol === 'http:' || url.protocol === 'https:') && url.host !== '' ? url.toString() : '';
  } catch {
    return '';
  }
}

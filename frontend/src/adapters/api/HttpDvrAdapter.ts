import {
  MotionDetectionConfig,
  PTZCommand,
  PTZPosition3D,
  PrivacyMaskConfig,
  RecordingDaySpan,
  RecordingSegment,
  SystemOverview,
  VideoChannel,
  VideoOverlayConfig,
} from '../../domain/models';
import { DvrApiPort } from '../../ports/DvrApiPort';

export class HttpDvrAdapter implements DvrApiPort {
  private readonly baseUrl: string;

  constructor(baseUrl = '') {
    this.baseUrl = baseUrl.replace(/\/$/, '');
  }

  private async request<T>(path: string, init?: RequestInit): Promise<T> {
    const resp = await fetch(`${this.baseUrl}${path}`, {
      headers: {
        'Content-Type': 'application/json',
        ...(init?.headers ?? {}),
      },
      ...init,
    });
    if (!resp.ok) {
      let errMsg = `HTTP ${resp.status}`;
      try {
        const body = await resp.json();
        if (body?.error) errMsg = body.error;
      } catch {
        // ignore
      }
      throw new Error(errMsg);
    }
    return resp.json() as Promise<T>;
  }

  getSystemOverview(): Promise<SystemOverview> {
    return this.request<SystemOverview>('/api/system/overview');
  }

  async rebootDevice(): Promise<void> {
    await this.request('/api/system/reboot', { method: 'POST' });
  }

  listChannels(): Promise<VideoChannel[]> {
    return this.request<VideoChannel[]>('/api/channels');
  }

  getLiveStreamUrl(channelId: number, streamType: 1 | 2): string {
    return `${this.baseUrl}/api/channels/${channelId}/live.mp4?stream=${streamType}`;
  }

  getSnapshotUrl(channelId: number, streamType: 1 | 2 = 1, download = false): string {
    const dl = download ? '&download=1' : '';
    return `${this.baseUrl}/api/channels/${channelId}/snapshot?stream=${streamType}${dl}&_t=${Date.now()}`;
  }

  async ptzMove(channelId: number, cmd: PTZCommand): Promise<void> {
    await this.request(`/api/channels/${channelId}/ptz/move`, {
      method: 'POST',
      body: JSON.stringify(cmd),
    });
  }

  async ptzStop(channelId: number): Promise<void> {
    await this.request(`/api/channels/${channelId}/ptz/stop`, { method: 'POST' });
  }

  async ptzPreset(channelId: number, presetId: number, action: 'goto' | 'set'): Promise<void> {
    await this.request(`/api/channels/${channelId}/ptz/presets/${presetId}`, {
      method: 'POST',
      body: JSON.stringify({ action }),
    });
  }

  async ptzZoom3D(channelId: number, pos: PTZPosition3D): Promise<void> {
    await this.request(`/api/channels/${channelId}/ptz/zoom3d`, {
      method: 'POST',
      body: JSON.stringify(pos),
    });
  }

  getMonthlyCalendar(channelId: number, year: number, month: number): Promise<RecordingDaySpan[]> {
    return this.request<RecordingDaySpan[]>(
      `/api/archive/channels/${channelId}/calendar?year=${year}&month=${month}`
    );
  }

  searchRecordings(channelId: number, startIso: string, endIso: string): Promise<RecordingSegment[]> {
    const params = new URLSearchParams({ start: startIso, end: endIso });
    return this.request<RecordingSegment[]>(
      `/api/archive/channels/${channelId}/search?${params.toString()}`
    );
  }

  getArchivePlayUrl(
    channelId: number,
    playbackUri: string,
    startIso?: string,
    endIso?: string
  ): string {
    const params = new URLSearchParams();
    if (playbackUri) params.set('uri', playbackUri);
    if (startIso) params.set('start', startIso);
    if (endIso) params.set('end', endIso);
    return `${this.baseUrl}/api/archive/channels/${channelId}/play.mp4?${params.toString()}`;
  }

  getArchiveDownloadUrl(playbackUri: string, filename: string): string {
    const params = new URLSearchParams({ uri: playbackUri, filename });
    return `${this.baseUrl}/api/archive/download.mp4?${params.toString()}`;
  }

  getChannelOverlays(channelId: number): Promise<VideoOverlayConfig> {
    return this.request<VideoOverlayConfig>(`/api/channels/${channelId}/overlays`);
  }

  async updateChannelOverlays(channelId: number, cfg: VideoOverlayConfig): Promise<void> {
    await this.request(`/api/channels/${channelId}/overlays`, {
      method: 'PUT',
      body: JSON.stringify(cfg),
    });
  }

  getChannelMotion(channelId: number): Promise<MotionDetectionConfig> {
    return this.request<MotionDetectionConfig>(`/api/channels/${channelId}/motion`);
  }

  async updateChannelMotion(channelId: number, cfg: MotionDetectionConfig): Promise<void> {
    await this.request(`/api/channels/${channelId}/motion`, {
      method: 'PUT',
      body: JSON.stringify(cfg),
    });
  }

  getChannelPrivacyMask(channelId: number): Promise<PrivacyMaskConfig> {
    return this.request<PrivacyMaskConfig>(`/api/channels/${channelId}/privacy-mask`);
  }

  async updateChannelPrivacyMask(channelId: number, cfg: PrivacyMaskConfig): Promise<void> {
    await this.request(`/api/channels/${channelId}/privacy-mask`, {
      method: 'PUT',
      body: JSON.stringify(cfg),
    });
  }
}

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
} from '../domain/models';

export interface DvrApiPort {
  // System
  getSystemOverview(): Promise<SystemOverview>;
  rebootDevice(): Promise<void>;

  // Channels & Streams
  listChannels(): Promise<VideoChannel[]>;
  getLiveStreamUrl(channelId: number, streamType: 1 | 2): string;
  getSnapshotUrl(channelId: number, streamType?: 1 | 2, download?: boolean): string;

  // PTZ
  ptzMove(channelId: number, cmd: PTZCommand): Promise<void>;
  ptzStop(channelId: number): Promise<void>;
  ptzPreset(channelId: number, presetId: number, action: 'goto' | 'set'): Promise<void>;
  ptzZoom3D(channelId: number, pos: PTZPosition3D): Promise<void>;

  // Archive Calendar, Search, Playback & Download
  getMonthlyCalendar(channelId: number, year: number, month: number): Promise<RecordingDaySpan[]>;
  searchRecordings(channelId: number, startIso: string, endIso: string): Promise<RecordingSegment[]>;
  getArchivePlayUrl(channelId: number, playbackUri: string, startIso?: string, endIso?: string): string;
  getArchiveDownloadUrl(playbackUri: string, filename: string): string;

  // Channel Visual Configuration
  getChannelOverlays(channelId: number): Promise<VideoOverlayConfig>;
  updateChannelOverlays(channelId: number, cfg: VideoOverlayConfig): Promise<void>;
  getChannelMotion(channelId: number): Promise<MotionDetectionConfig>;
  updateChannelMotion(channelId: number, cfg: MotionDetectionConfig): Promise<void>;
  getChannelPrivacyMask(channelId: number): Promise<PrivacyMaskConfig>;
  updateChannelPrivacyMask(channelId: number, cfg: PrivacyMaskConfig): Promise<void>;
}

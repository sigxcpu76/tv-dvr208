export const MOTION_ROWS = 18;
export const MOTION_COLS = 22;
export const NORMALIZED_WIDTH = 704;
export const NORMALIZED_HEIGHT = 576;

export interface DeviceInfo {
  deviceName: string;
  deviceId: string;
  model: string;
  serialNumber: string;
  macAddress: string;
  firmwareVersion: string;
  firmwareReleasedDate: string;
  encoderVersion: string;
  encoderReleasedDate: string;
  deviceType: string;
  telecontrolId: number;
}

export interface SystemTime {
  timeMode: string;
  localTime: string;
  timeZone: string;
}

export interface StorageVolume {
  id: number;
  name: string;
  type: string;
  status: string;
  capacityMb: number;
  freeSpaceMb: number;
  property: string;
}

export interface NetworkInterface {
  id: number;
  addressingType: string;
  ipAddress: string;
  subnetMask: string;
  defaultGateway: string;
  primaryDns: string;
  secondaryDns: string;
  macAddress: string;
  mtu: number;
}

export interface UserAccount {
  id: number;
  userName: string;
  userLevel: string;
}

export interface SystemOverview {
  deviceInfo: DeviceInfo;
  systemTime: SystemTime;
  storage: StorageVolume[];
  network: NetworkInterface;
  users: UserAccount[];
}

export interface StreamConfig {
  trackId: number;
  channelId: number;
  enabled: boolean;
  videoCodecType: string;
  resolutionWidth: number;
  resolutionHeight: number;
  qualityControlType: string;
  maxBitrateKbps: number;
  maxFrameRate: number;
  audioEnabled: boolean;
  audioCodecType: string;
}

export interface VideoChannel {
  id: number;
  inputPort: number;
  enabled: boolean;
  name: string;
  videoFormat: string;
  resDesc: string;
  mainStream?: StreamConfig;
  subStream?: StreamConfig;
}

export interface RecordingDaySpan {
  dayNumber: number;
  hasRecords: boolean;
  recordType?: string;
}

export interface RecordingSegment {
  trackId: number;
  channelId: number;
  startTime: string;
  endTime: string;
  playbackUri: string;
  fileName: string;
  sizeBytes: number;
  contentType: string;
}

export interface PTZCommand {
  pan: number;
  tilt: number;
  zoom: number;
}

export interface PTZPosition3D {
  startX: number;
  startY: number;
  endX: number;
  endY: number;
}

export interface TextOverlay {
  id: number;
  enabled: boolean;
  positionX: number;
  positionY: number;
  displayText: string;
}

export interface DateTimeOverlay {
  enabled: boolean;
  positionX: number;
  positionY: number;
  dateStyle: string;
  timeStyle: string;
  displayWeek: boolean;
}

export interface ChannelNameOverlay {
  enabled: boolean;
  positionX: number;
  positionY: number;
}

export interface VideoOverlayConfig {
  channelId: number;
  normalizedWidth: number;
  normalizedHeight: number;
  transparent: boolean;
  flashing: boolean;
  fontSize: number;
  dateTime: DateTimeOverlay;
  channelName: ChannelNameOverlay;
  textOverlays: TextOverlay[];
}

export type MotionGridMatrix = boolean[][];

export interface MotionDetectionConfig {
  channelId: number;
  enabled: boolean;
  enableHighlight: boolean;
  samplingInterval: number;
  startTriggerTimeMs: number;
  endTriggerTimeMs: number;
  sensitivityLevel: number;
  rowGranularity: number;
  columnGranularity: number;
  gridMapHex: string;
  grid: MotionGridMatrix;
}

export interface PrivacyMaskRegion {
  id: number;
  enabled: boolean;
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface PrivacyMaskConfig {
  channelId: number;
  enabled: boolean;
  normalizedWidth: number;
  normalizedHeight: number;
  regions: PrivacyMaskRegion[];
}

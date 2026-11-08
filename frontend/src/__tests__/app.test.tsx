import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { App } from '../App';
import { DvrProvider } from '../application/DvrContext';
import {
  MotionDetectionConfig,
  PTZCommand,
  PrivacyMaskConfig,
  VideoOverlayConfig,
} from '../domain/models';
import {
  countActiveCells,
  createMotionGrid,
  motionGridToHex,
  parseMotionGridHex,
  setMotionCell,
} from '../domain/motionGrid';
import {
  computeTimelineBars,
  formatBytes,
  formatDuration,
} from '../domain/timeline';
import { DvrApiPort } from '../ports/DvrApiPort';
import {
  detectAvcMime,
  prefersNativeHls,
  toHlsUrl,
  toLiveWsUrl,
} from '../adapters/ui/ManagedVideo';

function createFakeDvrPort(): DvrApiPort & {
  lastPtzMove: { channelId: number; cmd: PTZCommand } | null;
  lastSavedMotion: MotionDetectionConfig | null;
} {
  const fullGrid = createMotionGrid(true);
  const fake: DvrApiPort & {
    lastPtzMove: { channelId: number; cmd: PTZCommand } | null;
    lastSavedMotion: MotionDetectionConfig | null;
  } = {
    lastPtzMove: null,
    lastSavedMotion: null,

    async getSystemOverview() {
      return {
        deviceInfo: {
          deviceName: 'TV-DVR208',
          deviceId: '48383633-3732-3836-3831-3c8cf8a1c032',
          model: 'TV-DVR208',
          serialNumber: 'TV-DVR2080820170916CCWR863728681WCVU',
          macAddress: '3c:8c:f8:a1:c0:32',
          firmwareVersion: 'V3.4.7',
          firmwareReleasedDate: 'build 170123',
          encoderVersion: 'V5.0',
          encoderReleasedDate: 'build 160912',
          deviceType: 'IPC',
          telecontrolId: 255,
        },
        systemTime: {
          timeMode: 'NTP',
          localTime: '2026-11-07T16:20:00+02:00',
          timeZone: 'CST-2:00:00',
        },
        storage: [
          {
            id: 1,
            name: 'hdda',
            type: 'SATA',
            status: 'ok',
            capacityMb: 953869,
            freeSpaceMb: 0,
            property: 'RW',
          },
        ],
        network: {
          id: 1,
          addressingType: 'static',
          ipAddress: '192.168.27.60',
          subnetMask: '255.255.252.0',
          defaultGateway: '192.168.24.1',
          primaryDns: '192.168.24.1',
          secondaryDns: '0.0.0.0',
          macAddress: '3c:8c:f8:a1:c0:32',
          mtu: 1500,
        },
        users: [{ id: 2, userName: 'operator', userLevel: 'Operator' }],
      };
    },

    rebootDevice: vi.fn().mockResolvedValue(undefined),

    async listChannels() {
      return [
        {
          id: 1,
          inputPort: 1,
          enabled: true,
          name: 'Terasa',
          videoFormat: 'PAL',
          resDesc: '1080P25',
          mainStream: {
            trackId: 101,
            channelId: 1,
            enabled: true,
            videoCodecType: 'H.264',
            resolutionWidth: 1920,
            resolutionHeight: 1080,
            qualityControlType: 'VBR',
            maxBitrateKbps: 2048,
            maxFrameRate: 1200,
            audioEnabled: false,
            audioCodecType: 'G.711ulaw',
          },
        },
        {
          id: 2,
          inputPort: 2,
          enabled: true,
          name: 'Curte',
          videoFormat: 'PAL',
          resDesc: '1080P25',
        },
      ];
    },

    getLiveStreamUrl(channelId, streamType) {
      return `/api/channels/${channelId}/live.mp4?stream=${streamType}`;
    },

    getSnapshotUrl(channelId, streamType = 1, download = false) {
      return `/api/channels/${channelId}/snapshot?stream=${streamType}&download=${download ? 1 : 0}`;
    },

    async ptzMove(channelId, cmd) {
      fake.lastPtzMove = { channelId, cmd };
    },

    ptzStop: vi.fn().mockResolvedValue(undefined),
    ptzPreset: vi.fn().mockResolvedValue(undefined),
    ptzZoom3D: vi.fn().mockResolvedValue(undefined),

    async getMonthlyCalendar() {
      return [{ dayNumber: 7, hasRecords: true, recordType: 'time' }];
    },

    async searchRecordings(channelId) {
      return [
        {
          trackId: channelId * 100 + 1,
          channelId,
          startTime: '2026-11-07T12:00:00Z',
          endTime: '2026-11-07T13:00:00Z',
          playbackUri:
            'rtsp://192.168.27.60/Streaming/tracks/101/?starttime=20261107T120000Z&endtime=20261107T130000Z&name=00000000127000000&size=262040376',
          fileName: '00000000127000000',
          sizeBytes: 262040376,
          contentType: 'video',
        },
      ];
    },

    getArchivePlayUrl(channelId, playbackUri) {
      return `/api/archive/channels/${channelId}/play.mp4?uri=${encodeURIComponent(playbackUri)}`;
    },

    getArchiveDownloadUrl(playbackUri, filename) {
      return `/api/archive/download.mp4?uri=${encodeURIComponent(playbackUri)}&filename=${encodeURIComponent(filename)}`;
    },

    async getChannelOverlays(channelId): Promise<VideoOverlayConfig> {
      return {
        channelId,
        normalizedWidth: 704,
        normalizedHeight: 576,
        transparent: false,
        flashing: false,
        fontSize: 1,
        dateTime: {
          enabled: true,
          positionX: 16,
          positionY: 32,
          dateStyle: 'YYYY-MM-DD',
          timeStyle: '24hour',
          displayWeek: false,
        },
        channelName: {
          enabled: true,
          positionX: 512,
          positionY: 512,
        },
        textOverlays: [],
      };
    },

    updateChannelOverlays: vi.fn().mockResolvedValue(undefined),

    async getChannelMotion(channelId): Promise<MotionDetectionConfig> {
      return {
        channelId,
        enabled: true,
        enableHighlight: false,
        samplingInterval: 5,
        startTriggerTimeMs: 1000,
        endTriggerTimeMs: 1000,
        sensitivityLevel: 2,
        rowGranularity: 18,
        columnGranularity: 22,
        gridMapHex: motionGridToHex(fullGrid),
        grid: fullGrid,
      };
    },

    async updateChannelMotion(_channelId, cfg) {
      fake.lastSavedMotion = cfg;
    },

    async getChannelPrivacyMask(channelId): Promise<PrivacyMaskConfig> {
      return {
        channelId,
        enabled: false,
        normalizedWidth: 704,
        normalizedHeight: 576,
        regions: [],
      };
    },

    updateChannelPrivacyMask: vi.fn().mockResolvedValue(undefined),
  };

  return fake;
}

describe('Domain Logic: Motion Grid & Timeline', () => {
  it('encodes and decodes 22x18 Hikvision motion grid hex accurately', () => {
    const full = createMotionGrid(true);
    const hex = motionGridToHex(full);
    expect(hex).toHaveLength(108);
    expect(hex.slice(0, 6)).toBe('fffffc');

    const modified = setMotionCell(createMotionGrid(false), 0, 0, true);
    const modHex = motionGridToHex(modified);
    expect(modHex.slice(0, 6)).toBe('800000');

    const decoded = parseMotionGridHex(modHex);
    expect(decoded[0][0]).toBe(true);
    expect(countActiveCells(decoded)).toBe(1);
  });

  it('computes 24-hour timeline bar geometry and formats bytes/durations', () => {
    const bars = computeTimelineBars(
      [
        {
          trackId: 101,
          channelId: 1,
          startTime: '2026-11-07T06:00:00Z',
          endTime: '2026-11-07T12:00:00Z',
          playbackUri: 'rtsp://test',
          fileName: 'clip1',
          sizeBytes: 1048576,
          contentType: 'video',
        },
      ],
      '2026-11-07'
    );
    expect(bars).toHaveLength(1);
    expect(bars[0].leftPercent).toBeCloseTo(25, 1);
    expect(bars[0].widthPercent).toBeCloseTo(25, 1);
    expect(formatBytes(262040376)).toBe('249.9 MB');
    expect(formatDuration('2026-11-07T12:00:00Z', '2026-11-07T12:13:10Z')).toBe('13m 10s');
  });

  it('converts progressive fMP4 stream URLs to Apple HLS (.m3u8) URLs for iOS/Safari', () => {
    expect(toHlsUrl('/api/channels/1/live.mp4?stream=2&t=123')).toBe(
      '/api/channels/1/hls/2/index.m3u8?t=123'
    );
    expect(toHlsUrl('/api/channels/3/live.mp4?stream=1')).toBe(
      '/api/channels/3/hls/1/index.m3u8'
    );
    expect(
      toHlsUrl('/api/archive/channels/1/play.mp4?uri=rtsp%3A%2F%2F192.168.27.60%2FStreaming%2Ftracks%2F101%2F')
    ).toBe(
      '/api/archive/channels/1/hls/index.m3u8?uri=rtsp%3A%2F%2F192.168.27.60%2FStreaming%2Ftracks%2F101%2F'
    );
  });

  it('converts live fMP4 URLs to WebSocket URLs and extracts avcC codec strings for MSE', () => {
    expect(toLiveWsUrl('http://localhost:8080/api/channels/1/live.mp4?stream=2')).toBe(
      'ws://localhost:8080/api/channels/1/live.ws?stream=2'
    );
    expect(toLiveWsUrl('https://dvr.example.com/api/channels/4/live.mp4?stream=1')).toBe(
      'wss://dvr.example.com/api/channels/4/live.ws?stream=1'
    );
    expect(toLiveWsUrl('/api/archive/channels/1/play.mp4?uri=rtsp://test')).toBeNull();

    // Simulated ISO BMFF avcC box header: 'avcC' followed by version=1, profile=0x64, compat=0x00, level=0x1f
    const fakeInit = new Uint8Array([0x00, 0x00, 0x00, 0x2c, 0x61, 0x76, 0x63, 0x43, 0x01, 0x64, 0x00, 0x1f]);
    expect(detectAvcMime(fakeInit)).toBe('video/mp4; codecs="avc1.64001f"');
  });

  it('never selects native HLS (.m3u8) on Desktop Chrome even when canPlayType returns maybe', () => {
    const origUA = navigator.userAgent;
    Object.defineProperty(window.navigator, 'userAgent', {
      value:
        'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
      configurable: true,
    });
    const fakeVideo = {
      canPlayType: () => 'maybe',
    } as unknown as HTMLVideoElement;
    expect(prefersNativeHls(fakeVideo)).toBe(false);

    Object.defineProperty(window.navigator, 'userAgent', {
      value:
        'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1',
      configurable: true,
    });
    expect(prefersNativeHls(fakeVideo)).toBe(true);

    Object.defineProperty(window.navigator, 'userAgent', {
      value: origUA,
      configurable: true,
    });
  });
});

describe('Hexagonal UI Integration via DvrApiPort', () => {
  it('renders Live View, switches camera selection, and dispatches PTZ commands', async () => {
    const port = createFakeDvrPort();
    render(
      <DvrProvider adapter={port}>
        <App />
      </DvrProvider>
    );

    await waitFor(() => {
      expect(screen.getByTestId('camera-tile-1')).toBeInTheDocument();
      expect(screen.getByTestId('camera-tile-2')).toBeInTheDocument();
    });

    // Select channel 2 tile and trigger PTZ Right
    fireEvent.click(screen.getByTestId('camera-tile-2'));
    fireEvent.mouseDown(screen.getByLabelText('PTZ Right'));

    await waitFor(() => {
      expect(port.lastPtzMove).toEqual({
        channelId: 2,
        cmd: { pan: 50, tilt: 0, zoom: 0 },
      });
    });
  });

  it('navigates to Archive & Playback, renders 24h timeline segments, and exposes lossless MP4 download', async () => {
    const port = createFakeDvrPort();
    render(
      <DvrProvider adapter={port}>
        <App />
      </DvrProvider>
    );

    await waitFor(() => expect(screen.getByTestId('camera-tile-1')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /Archive & Playback/i }));

    await waitFor(() => {
      expect(screen.getByText('00000000127000000')).toBeInTheDocument();
      expect(screen.getByTestId('active-download-btn')).toHaveAttribute(
        'href',
        expect.stringContaining('/api/archive/download.mp4')
      );
    });
  });

  it('navigates to DVR Configuration, edits the 22x18 SVG Motion Grid, and views System/HDD details', async () => {
    const port = createFakeDvrPort();
    render(
      <DvrProvider adapter={port}>
        <App />
      </DvrProvider>
    );

    await waitFor(() => expect(screen.getByTestId('camera-tile-1')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /DVR Configuration/i }));

    await waitFor(() => {
      expect(screen.getByTestId('motion-svg-grid')).toBeInTheDocument();
    });

    // Toggle cell (0,0) off and save
    fireEvent.mouseDown(screen.getByTestId('motion-cell-0-0'));
    fireEvent.click(screen.getByRole('button', { name: /Save Motion Detection/i }));

    await waitFor(() => {
      expect(port.lastSavedMotion).not.toBeNull();
      expect(port.lastSavedMotion?.grid[0][0]).toBe(false);
      expect(port.lastSavedMotion?.gridMapHex.slice(0, 6)).toBe('7ffffc');
    });

    // Switch to System, Storage & Network sub-tab
    fireEvent.click(screen.getByRole('button', { name: /System, Storage & Network/i }));
    await waitFor(() => {
      expect(screen.getByText(/TV-DVR2080820170916CCWR863728681WCVU/)).toBeInTheDocument();
      expect(screen.getByText(/hdda/)).toBeInTheDocument();
    });
  });

  it('switches to the dedicated Mobile UI and exercises mobile channel strip, PTZ tab, and mobile archive cards', async () => {
    const port = createFakeDvrPort();
    render(
      <DvrProvider adapter={port}>
        <App />
      </DvrProvider>
    );

    await waitFor(() => expect(screen.getByTestId('camera-tile-1')).toBeInTheDocument());

    // Switch to dedicated Mobile Console
    fireEvent.click(screen.getByRole('button', { name: /Mobile View/i }));

    await waitFor(() => {
      expect(screen.getByTestId('mobile-console')).toBeInTheDocument();
      expect(screen.getByTestId('mobile-ch-pill-2')).toBeInTheDocument();
    });

    // Tap channel 2 pill in horizontal strip and open Mobile PTZ tab
    fireEvent.click(screen.getByTestId('mobile-ch-pill-2'));
    fireEvent.click(screen.getByRole('button', { name: /^PTZ$/i }));

    await waitFor(() => {
      expect(screen.getByTestId('mobile-ptz-panel')).toBeInTheDocument();
    });

    fireEvent.pointerDown(screen.getByLabelText('Mobile PTZ Right'));
    await waitFor(() => {
      expect(port.lastPtzMove).toEqual({
        channelId: 2,
        cmd: { pan: 50, tilt: 0, zoom: 0 },
      });
    });

    // Navigate to Mobile Archive tab and verify segment card rendering
    fireEvent.click(screen.getByRole('button', { name: /^Archive$/i }));
    await waitFor(() => {
      expect(screen.getByTestId('mobile-segment-card-0')).toBeInTheDocument();
      expect(screen.getByTestId('mobile-active-download-btn')).toHaveAttribute(
        'href',
        expect.stringContaining('/api/archive/download.mp4')
      );
    });
  });
});

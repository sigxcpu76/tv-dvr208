import React, { useEffect, useState } from 'react';
import {
  Calendar,
  Camera,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Clock,
  Cpu,
  Download,
  Film,
  Grid,
  HardDrive,
  Monitor,
  Move,
  Play,
  Power,
  Radio,
  RefreshCw,
  Settings,
  ShieldCheck,
  Sliders,
  Video,
  Wifi,
  ZoomIn,
  ZoomOut,
} from 'lucide-react';
import { useDvrPort } from '../../../application/DvrContext';
import {
  MOTION_COLS,
  MOTION_ROWS,
  MotionDetectionConfig,
  RecordingDaySpan,
  RecordingSegment,
  SystemOverview,
  VideoChannel,
  VideoOverlayConfig,
} from '../../../domain/models';
import {
  countActiveCells,
  createMotionGrid,
  motionGridToHex,
  setMotionCell,
} from '../../../domain/motionGrid';
import {
  computeTimelineBars,
  formatBytes,
  formatDuration,
  formatTimeUTC,
} from '../../../domain/timeline';
import { ManagedVideo } from '../ManagedVideo';

export type MobileTab = 'live' | 'archive' | 'ptz' | 'config';

interface MobileConsoleProps {
  channels: VideoChannel[];
  overview: SystemOverview | null;
  error: string | null;
  onSwitchToDesktop?: () => void;
  defaultDateIso?: string;
}

export const MobileConsole: React.FC<MobileConsoleProps> = ({
  channels,
  overview,
  error,
  onSwitchToDesktop,
  defaultDateIso = '2026-11-07',
}) => {
  const dvr = useDvrPort();

  const [activeTab, setActiveTab] = useState<MobileTab>('live');
  const [selectedChannelId, setSelectedChannelId] = useState<number>(1);
  const [streamType, setStreamType] = useState<1 | 2>(2);
  const [renderMode, setRenderMode] = useState<'video' | 'snapshot'>('video');
  const [snapshotTick, setSnapshotTick] = useState<number>(0);

  // PTZ state
  const [ptzSpeed, setPtzSpeed] = useState<number>(50);
  const [ptzStatus, setPtzStatus] = useState<string>('Ready');

  // Archive state
  const [selectedDate, setSelectedDate] = useState<string>(defaultDateIso);
  const [calendarDays, setCalendarDays] = useState<RecordingDaySpan[]>([]);
  const [segments, setSegments] = useState<RecordingSegment[]>([]);
  const [activeSegment, setActiveSegment] = useState<RecordingSegment | null>(null);
  const [playbackRate, setPlaybackRate] = useState<number>(1.0);
  const [archiveLoading, setArchiveLoading] = useState<boolean>(false);
  const [archiveError, setArchiveError] = useState<string | null>(null);

  // Config state
  const [configSubTab, setConfigSubTab] = useState<'system' | 'motion' | 'osd'>('system');
  const [motionCfg, setMotionCfg] = useState<MotionDetectionConfig | null>(null);
  const [overlayCfg, setOverlayCfg] = useState<VideoOverlayConfig | null>(null);
  const [configStatus, setConfigStatus] = useState<string | null>(null);

  const activeChannel =
    channels.find((c) => c.id === selectedChannelId) ?? {
      id: selectedChannelId,
      inputPort: selectedChannelId,
      enabled: true,
      name: `Camera ${selectedChannelId}`,
      videoFormat: 'PAL',
      resDesc: '1080P25',
    };

  const stepChannel = (delta: number) => {
    if (channels.length === 0) return;
    const idx = channels.findIndex((c) => c.id === selectedChannelId);
    const nextIdx = (idx + delta + channels.length) % channels.length;
    setSelectedChannelId(channels[nextIdx].id);
  };

  // PTZ handlers (pointer/touch friendly)
  const handlePtzStart = async (pan: number, tilt: number, zoom: number, label: string) => {
    try {
      setPtzStatus(`${label} • CH ${selectedChannelId}`);
      await dvr.ptzMove(selectedChannelId, { pan, tilt, zoom });
    } catch (e) {
      setPtzStatus(`PTZ Error: ${(e as Error).message}`);
    }
  };

  const handlePtzStop = async () => {
    try {
      await dvr.ptzStop(selectedChannelId);
      setPtzStatus(`Stopped • CH ${selectedChannelId}`);
    } catch (e) {
      setPtzStatus(`Stop Error: ${(e as Error).message}`);
    }
  };

  const handlePreset = async (presetId: number, action: 'goto' | 'set') => {
    try {
      await dvr.ptzPreset(selectedChannelId, presetId, action);
      setPtzStatus(`${action === 'goto' ? 'Called' : 'Saved'} Preset #${presetId}`);
    } catch (e) {
      setPtzStatus(`Preset Error: ${(e as Error).message}`);
    }
  };

  // Archive data loading when on Archive tab
  const [yearStr, monthStr] = selectedDate.split('-');
  const year = Number(yearStr) || 2026;
  const month = Number(monthStr) || 11;

  useEffect(() => {
    if (activeTab !== 'archive') return;
    let mounted = true;
    dvr
      .getMonthlyCalendar(selectedChannelId, year, month)
      .then((days) => {
        if (mounted) setCalendarDays(days);
      })
      .catch(() => {
        if (mounted) setCalendarDays([]);
      });
    return () => {
      mounted = false;
    };
  }, [dvr, activeTab, selectedChannelId, year, month]);

  const loadArchiveDay = async (chId: number, dateIso: string) => {
    setArchiveLoading(true);
    setArchiveError(null);
    try {
      const segs = await dvr.searchRecordings(
        chId,
        `${dateIso}T00:00:00Z`,
        `${dateIso}T23:59:59Z`
      );
      setSegments(segs);
      setActiveSegment(segs.length > 0 ? segs[0] : null);
    } catch (e) {
      setArchiveError((e as Error).message);
    } finally {
      setArchiveLoading(false);
    }
  };

  useEffect(() => {
    if (activeTab !== 'archive') return;
    void loadArchiveDay(selectedChannelId, selectedDate);
  }, [activeTab, selectedChannelId, selectedDate]);

  // Config data loading when on Config tab
  useEffect(() => {
    if (activeTab !== 'config') return;
    let mounted = true;
    setConfigStatus(null);
    Promise.all([
      dvr.getChannelMotion(selectedChannelId),
      dvr.getChannelOverlays(selectedChannelId),
    ])
      .then(([m, o]) => {
        if (!mounted) return;
        setMotionCfg(m);
        setOverlayCfg(o);
      })
      .catch((e) => {
        if (!mounted) return;
        setConfigStatus(`Error: ${(e as Error).message}`);
      });
    return () => {
      mounted = false;
    };
  }, [dvr, activeTab, selectedChannelId]);

  const toggleMotionCellAt = (r: number, c: number) => {
    if (!motionCfg) return;
    const nextVal = !motionCfg.grid[r]?.[c];
    const nextGrid = setMotionCell(motionCfg.grid, r, c, nextVal);
    setMotionCfg({
      ...motionCfg,
      grid: nextGrid,
      gridMapHex: motionGridToHex(nextGrid),
    });
  };

  const handleSaveMotion = async () => {
    if (!motionCfg) return;
    try {
      await dvr.updateChannelMotion(selectedChannelId, motionCfg);
      setConfigStatus(`Saved motion grid for CH ${selectedChannelId}`);
    } catch (e) {
      setConfigStatus(`Error: ${(e as Error).message}`);
    }
  };

  const handleSaveOverlays = async () => {
    if (!overlayCfg) return;
    try {
      await dvr.updateChannelOverlays(selectedChannelId, overlayCfg);
      setConfigStatus(`Saved OSD overlays for CH ${selectedChannelId}`);
    } catch (e) {
      setConfigStatus(`Error: ${(e as Error).message}`);
    }
  };

  const liveStreamUrl = dvr.getLiveStreamUrl(selectedChannelId, streamType);
  const activeSnapUrl = `${dvr.getSnapshotUrl(selectedChannelId, 1, false)}&tick=${snapshotTick}`;
  const downloadSnapUrl = dvr.getSnapshotUrl(selectedChannelId, 1, true);

  const recordedDays = calendarDays.filter((d) => d.hasRecords);
  const timelineBars = computeTimelineBars(segments, selectedDate);
  const archivePlayUrl = activeSegment
    ? dvr.getArchivePlayUrl(selectedChannelId, activeSegment.playbackUri)
    : '';

  return (
    <div className="mobile-shell" data-testid="mobile-console">
      {/* Compact Sticky Mobile Header */}
      <header className="mobile-topbar">
        <div className="mobile-brand">
          <span className="online-dot" />
          <div>
            <div className="mobile-brand-title">
              {overview?.deviceInfo?.deviceName ?? 'TV-DVR208'}
            </div>
            <div className="mobile-brand-sub">
              {overview?.network?.ipAddress ?? '192.168.1.64'} • CH {selectedChannelId}{' '}
              {activeChannel.name}
            </div>
          </div>
        </div>

        <div className="mobile-topbar-actions">
          {onSwitchToDesktop && (
            <button
              type="button"
              className="mobile-icon-pill"
              onClick={onSwitchToDesktop}
              title="Switch to Desktop Layout"
              aria-label="Desktop View"
            >
              <Monitor size={15} />
              <span>Desktop</span>
            </button>
          )}
        </div>
      </header>

      {/* Horizontal Touch-Scrollable Camera Selector Strip */}
      <div className="mobile-channel-strip" role="tablist" aria-label="Camera Channels">
        {channels.map((ch) => {
          const isSelected = ch.id === selectedChannelId;
          return (
            <button
              key={ch.id}
              type="button"
              role="tab"
              aria-selected={isSelected}
              data-testid={`mobile-ch-pill-${ch.id}`}
              className={`mobile-ch-pill ${isSelected ? 'active' : ''}`}
              onClick={() => setSelectedChannelId(ch.id)}
            >
              <span className="mobile-ch-num">CH{ch.id}</span>
              <span className="mobile-ch-name">{ch.name}</span>
            </button>
          );
        })}
      </div>

      {error && (
        <div className="error-banner" role="alert">
          {error}
        </div>
      )}

      {/* Main Scrollable Mobile Content Area */}
      <main className="mobile-content">
        {/* ================= TAB 1 & 2: LIVE VIEW & PTZ ================= */}
        {(activeTab === 'live' || activeTab === 'ptz') && (
          <div className="mobile-section">
            {/* Sticky Single-Stream 16:9 Mobile Video Card */}
            <div className="mobile-player-card">
              <div className="mobile-viewport">
                {renderMode === 'video' ? (
                  <ManagedVideo
                    key={`mob-live-${selectedChannelId}-${streamType}`}
                    src={liveStreamUrl}
                    poster={activeSnapUrl}
                    autoPlay
                    muted
                    playsInline
                    controls
                    className="mobile-video"
                  />
                ) : (
                  <img
                    src={activeSnapUrl}
                    alt={activeChannel.name}
                    className="mobile-video"
                  />
                )}

                <button
                  type="button"
                  className="mobile-nav-arrow left"
                  aria-label="Previous Camera"
                  onClick={() => stepChannel(-1)}
                >
                  <ChevronLeft size={20} />
                </button>
                <button
                  type="button"
                  className="mobile-nav-arrow right"
                  aria-label="Next Camera"
                  onClick={() => stepChannel(1)}
                >
                  <ChevronRight size={20} />
                </button>

                <div className="mobile-player-badge">
                  <span className="live-dot" />
                  <span>
                    CH {selectedChannelId} — {activeChannel.name}
                  </span>
                </div>
              </div>

              {/* Quick Stream Controls Bar under Player */}
              <div className="mobile-player-controls">
                <div className="mobile-Segmented">
                  <button
                    type="button"
                    className={`mob-seg-btn ${streamType === 2 ? 'active' : ''}`}
                    onClick={() => setStreamType(2)}
                  >
                    SD (Sub)
                  </button>
                  <button
                    type="button"
                    className={`mob-seg-btn ${streamType === 1 ? 'active' : ''}`}
                    onClick={() => setStreamType(1)}
                  >
                    HD (1080p)
                  </button>
                </div>

                <div className="mobile-Segmented">
                  <button
                    type="button"
                    className={`mob-seg-btn ${renderMode === 'video' ? 'active' : ''}`}
                    onClick={() => setRenderMode('video')}
                  >
                    <Video size={13} /> Live
                  </button>
                  <button
                    type="button"
                    className={`mob-seg-btn ${renderMode === 'snapshot' ? 'active' : ''}`}
                    onClick={() => setRenderMode('snapshot')}
                  >
                    <Camera size={13} /> Snap
                  </button>
                </div>

                <a
                  href={downloadSnapUrl}
                  className="mob-action-btn"
                  title="Save 1080p JPEG"
                  aria-label="Save Snapshot"
                >
                  <Download size={15} />
                </a>
              </div>
            </div>

            {activeTab === 'live' ? (
              /* 2-Column All-Camera Snapshot Matrix (Tap any card to switch active live stream) */
              <div className="mobile-card">
                <div className="mobile-card-header">
                  <h4>
                    <Grid size={15} /> All Cameras (Tap to Stream)
                  </h4>
                  <button
                    type="button"
                    className="mob-mini-btn"
                    onClick={() => setSnapshotTick((t) => t + 1)}
                  >
                    <RefreshCw size={13} /> Refresh Previews
                  </button>
                </div>

                <div className="mobile-thumb-grid">
                  {channels.map((ch) => {
                    const isSelected = ch.id === selectedChannelId;
                    const thumbUrl = `${dvr.getSnapshotUrl(ch.id, 2, false)}&tick=${snapshotTick}`;
                    return (
                      <button
                        key={ch.id}
                        type="button"
                        data-testid={`mobile-thumb-${ch.id}`}
                        className={`mobile-thumb-tile ${isSelected ? 'selected' : ''}`}
                        onClick={() => {
                          setSelectedChannelId(ch.id);
                          setRenderMode('video');
                        }}
                      >
                        <div className="mobile-thumb-img-wrap">
                          <img src={thumbUrl} alt={ch.name} loading="lazy" />
                          <span className="mobile-thumb-tag">CH {ch.id}</span>
                          {isSelected && <span className="mobile-thumb-playing">ACTIVE</span>}
                        </div>
                        <div className="mobile-thumb-meta">
                          <strong>{ch.name}</strong>
                          <span>{ch.resDesc}</span>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </div>
            ) : (
              /* Dedicated Touch PTZ & Presets Console */
              <div className="mobile-card" data-testid="mobile-ptz-panel">
                <div className="mobile-card-header">
                  <h4>
                    <Move size={15} /> Touch PTZ — CH {selectedChannelId} ({activeChannel.name})
                  </h4>
                  <span className="mobile-status-chip" data-testid="mobile-ptz-status">
                    {ptzStatus}
                  </span>
                </div>

                <div className="mobile-ptz-layout">
                  <div className="mobile-ptz-dpad">
                    <button
                      type="button"
                      className="mob-ptz-key"
                      onPointerDown={() => handlePtzStart(-ptzSpeed, ptzSpeed, 0, 'Up-Left')}
                      onPointerUp={handlePtzStop}
                    >
                      ↖
                    </button>
                    <button
                      type="button"
                      className="mob-ptz-key"
                      aria-label="Mobile PTZ Up"
                      onPointerDown={() => handlePtzStart(0, ptzSpeed, 0, 'Tilt Up')}
                      onPointerUp={handlePtzStop}
                    >
                      ▲
                    </button>
                    <button
                      type="button"
                      className="mob-ptz-key"
                      onPointerDown={() => handlePtzStart(ptzSpeed, ptzSpeed, 0, 'Up-Right')}
                      onPointerUp={handlePtzStop}
                    >
                      ↗
                    </button>

                    <button
                      type="button"
                      className="mob-ptz-key"
                      aria-label="Mobile PTZ Left"
                      onPointerDown={() => handlePtzStart(-ptzSpeed, 0, 0, 'Pan Left')}
                      onPointerUp={handlePtzStop}
                    >
                      ◀
                    </button>
                    <button
                      type="button"
                      className="mob-ptz-key stop"
                      aria-label="Mobile PTZ Stop"
                      onClick={handlePtzStop}
                    >
                      ■
                    </button>
                    <button
                      type="button"
                      className="mob-ptz-key"
                      aria-label="Mobile PTZ Right"
                      onPointerDown={() => handlePtzStart(ptzSpeed, 0, 0, 'Pan Right')}
                      onPointerUp={handlePtzStop}
                    >
                      ▶
                    </button>

                    <button
                      type="button"
                      className="mob-ptz-key"
                      onPointerDown={() => handlePtzStart(-ptzSpeed, -ptzSpeed, 0, 'Down-Left')}
                      onPointerUp={handlePtzStop}
                    >
                      ↙
                    </button>
                    <button
                      type="button"
                      className="mob-ptz-key"
                      aria-label="Mobile PTZ Down"
                      onPointerDown={() => handlePtzStart(0, -ptzSpeed, 0, 'Tilt Down')}
                      onPointerUp={handlePtzStop}
                    >
                      ▼
                    </button>
                    <button
                      type="button"
                      className="mob-ptz-key"
                      onPointerDown={() => handlePtzStart(ptzSpeed, -ptzSpeed, 0, 'Down-Right')}
                      onPointerUp={handlePtzStop}
                    >
                      ↘
                    </button>
                  </div>

                  <div className="mobile-ptz-side">
                    <div className="mobile-zoom-pair">
                      <button
                        type="button"
                        className="mob-zoom-btn"
                        onPointerDown={() => handlePtzStart(0, 0, ptzSpeed, 'Zoom In')}
                        onPointerUp={handlePtzStop}
                      >
                        <ZoomIn size={16} /> Zoom +
                      </button>
                      <button
                        type="button"
                        className="mob-zoom-btn"
                        onPointerDown={() => handlePtzStart(0, 0, -ptzSpeed, 'Zoom Out')}
                        onPointerUp={handlePtzStop}
                      >
                        <ZoomOut size={16} /> Zoom -
                      </button>
                    </div>

                    <div className="mobile-speed-box">
                      <label htmlFor="mob-ptz-speed">Speed: {ptzSpeed}%</label>
                      <input
                        id="mob-ptz-speed"
                        type="range"
                        min={10}
                        max={100}
                        step={10}
                        value={ptzSpeed}
                        onChange={(e) => setPtzSpeed(Number(e.target.value))}
                      />
                    </div>

                    <div className="mobile-presets-grid">
                      {[1, 2, 3, 4].map((p) => (
                        <button
                          key={p}
                          type="button"
                          className="mob-preset-btn"
                          onClick={() => handlePreset(p, 'goto')}
                        >
                          Preset #{p}
                        </button>
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {/* ================= TAB 3: ARCHIVE & PLAYBACK ================= */}
        {activeTab === 'archive' && (
          <div className="mobile-section" data-testid="mobile-archive-section">
            {/* Sticky Archive Video Player */}
            <div className="mobile-player-card">
              <div className="mobile-viewport">
                {activeSegment ? (
                  <ManagedVideo
                    key={activeSegment.playbackUri}
                    src={archivePlayUrl}
                    playbackRate={playbackRate}
                    controls
                    autoPlay
                    playsInline
                    className="mobile-video"
                  />
                ) : (
                  <div className="mobile-empty-player">
                    {archiveLoading
                      ? 'Searching recordings...'
                      : 'Tap a recording card below to play'}
                  </div>
                )}
              </div>

              <div className="mobile-player-controls">
                <div className="mobile-Segmented">
                  {[0.5, 1.0, 2.0, 4.0].map((rate) => (
                    <button
                      key={rate}
                      type="button"
                      className={`mob-seg-btn ${playbackRate === rate ? 'active' : ''}`}
                      onClick={() => setPlaybackRate(rate)}
                    >
                      {rate}×
                    </button>
                  ))}
                </div>

                {activeSegment && (
                  <a
                    href={dvr.getArchiveDownloadUrl(
                      activeSegment.playbackUri,
                      `CH${selectedChannelId}_${activeChannel.name}_${selectedDate}_${formatTimeUTC(activeSegment.startTime).replace(/:/g, '')}.mp4`
                    )}
                    className="mob-download-pill"
                    data-testid="mobile-active-download-btn"
                  >
                    <Download size={14} /> Save .MP4
                  </a>
                )}
              </div>
            </div>

            {/* Compact Date Selector + Horizontal Recorded-Day Chips */}
            <div className="mobile-card">
              <div className="mobile-date-row">
                <label htmlFor="mob-archive-date" className="mobile-field-label">
                  <Calendar size={14} /> Date
                </label>
                <input
                  id="mob-archive-date"
                  type="date"
                  value={selectedDate}
                  onChange={(e) => setSelectedDate(e.target.value)}
                  className="mobile-date-input"
                />
                <button
                  type="button"
                  className="mob-mini-btn"
                  onClick={() => loadArchiveDay(selectedChannelId, selectedDate)}
                >
                  <RefreshCw size={13} />
                </button>
              </div>

              {recordedDays.length > 0 && (
                <div className="mobile-recorded-days-strip">
                  <span className="mobile-strip-label">Recorded Days:</span>
                  {recordedDays.map((d) => {
                    const dayIso = `${year}-${String(month).padStart(2, '0')}-${String(d.dayNumber).padStart(2, '0')}`;
                    const isSelected = selectedDate === dayIso;
                    return (
                      <button
                        key={d.dayNumber}
                        type="button"
                        className={`mob-day-chip ${isSelected ? 'active' : ''}`}
                        onClick={() => setSelectedDate(dayIso)}
                      >
                        {String(month).padStart(2, '0')}/{String(d.dayNumber).padStart(2, '0')}
                      </button>
                    );
                  })}
                </div>
              )}

              {/* Touch 24h Scrubber */}
              <div className="mobile-timeline-wrap">
                <div className="timeline-header">
                  <span>
                    <Clock size={12} /> 24h Timeline ({selectedDate})
                  </span>
                  <span>{segments.length} clips</span>
                </div>
                <div className="timeline-track">
                  {timelineBars.map((bar, idx) => {
                    const isActive =
                      activeSegment?.playbackUri === bar.segment.playbackUri;
                    return (
                      <div
                        key={bar.segment.playbackUri || idx}
                        className={`timeline-segment ${isActive ? 'active' : ''}`}
                        style={{
                          left: `${bar.leftPercent}%`,
                          width: `${bar.widthPercent}%`,
                        }}
                        onClick={() => setActiveSegment(bar.segment)}
                      />
                    );
                  })}
                </div>
                <div className="timeline-ruler">
                  <span>00h</span>
                  <span>06h</span>
                  <span>12h</span>
                  <span>18h</span>
                  <span>24h</span>
                </div>
              </div>
            </div>

            {/* Touch-Friendly Segment Cards List (Replaces wide desktop table) */}
            <div className="mobile-card">
              <div className="mobile-card-header">
                <h4>
                  <Film size={15} /> Recorded Clips ({segments.length})
                </h4>
              </div>

              {archiveError && <div className="error-banner">{archiveError}</div>}

              <div className="mobile-segment-list">
                {segments.map((seg, idx) => {
                  const isCurrent = activeSegment?.playbackUri === seg.playbackUri;
                  const dlUrl = dvr.getArchiveDownloadUrl(
                    seg.playbackUri,
                    `CH${selectedChannelId}_${selectedDate}_${formatTimeUTC(seg.startTime).replace(/:/g, '')}.mp4`
                  );
                  return (
                    <div
                      key={seg.playbackUri || idx}
                      data-testid={`mobile-segment-card-${idx}`}
                      className={`mobile-segment-card ${isCurrent ? 'active' : ''}`}
                      onClick={() => setActiveSegment(seg)}
                    >
                      <div className="mobile-segment-info">
                        <div className="mobile-segment-time">
                          <span className="mob-clip-index">#{idx + 1}</span>
                          <strong>
                            {formatTimeUTC(seg.startTime)} – {formatTimeUTC(seg.endTime)}
                          </strong>
                        </div>
                        <div className="mobile-segment-sub">
                          <span>{formatDuration(seg.startTime, seg.endTime)}</span>
                          <span>•</span>
                          <span>{formatBytes(seg.sizeBytes)}</span>
                          {seg.fileName && (
                            <>
                              <span>•</span>
                              <span className="mono">{seg.fileName}</span>
                            </>
                          )}
                        </div>
                      </div>

                      <div
                        className="mobile-segment-actions"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <button
                          type="button"
                          className="mob-seg-play-btn"
                          onClick={() => setActiveSegment(seg)}
                          aria-label={`Play clip ${idx + 1}`}
                        >
                          <Play size={14} />
                        </button>
                        <a
                          href={dlUrl}
                          className="mob-seg-dl-btn"
                          aria-label={`Download clip ${idx + 1}`}
                        >
                          <Download size={14} />
                        </a>
                      </div>
                    </div>
                  );
                })}

                {segments.length === 0 && !archiveLoading && (
                  <div className="mobile-empty-state">
                    No recordings found for CH {selectedChannelId} on {selectedDate}.
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {/* ================= TAB 4: SYSTEM & CONFIGURATION ================= */}
        {activeTab === 'config' && (
          <div className="mobile-section" data-testid="mobile-config-section">
            <div className="mobile-subtab-bar">
              <button
                type="button"
                className={`mob-subtab ${configSubTab === 'system' ? 'active' : ''}`}
                onClick={() => setConfigSubTab('system')}
              >
                <Cpu size={14} /> System & HDD
              </button>
              <button
                type="button"
                className={`mob-subtab ${configSubTab === 'motion' ? 'active' : ''}`}
                onClick={() => setConfigSubTab('motion')}
              >
                <Grid size={14} /> Motion (22×18)
              </button>
              <button
                type="button"
                className={`mob-subtab ${configSubTab === 'osd' ? 'active' : ''}`}
                onClick={() => setConfigSubTab('osd')}
              >
                <Sliders size={14} /> OSD Overlays
              </button>
            </div>

            {configStatus && (
              <div className="status-pill full-width">
                <CheckCircle2 size={14} /> {configStatus}
              </div>
            )}

            {configSubTab === 'system' && (
              <>
                <div className="mobile-card">
                  <div className="mobile-card-header">
                    <h4>
                      <ShieldCheck size={15} /> Device & Firmware
                    </h4>
                  </div>
                  <dl className="kv-list">
                    <div>
                      <dt>Model</dt>
                      <dd>{overview?.deviceInfo?.model ?? 'TV-DVR208'}</dd>
                    </div>
                    <div>
                      <dt>Firmware</dt>
                      <dd>{overview?.deviceInfo?.firmwareVersion ?? 'V3.4.7'}</dd>
                    </div>
                    <div>
                      <dt>Serial</dt>
                      <dd className="mono">{overview?.deviceInfo?.serialNumber ?? '—'}</dd>
                    </div>
                    <div>
                      <dt>System Time</dt>
                      <dd className="mono">
                        {overview?.systemTime?.localTime?.slice(0, 19).replace('T', ' ') ?? '—'}
                      </dd>
                    </div>
                  </dl>
                </div>

                <div className="mobile-card">
                  <div className="mobile-card-header">
                    <h4>
                      <HardDrive size={15} /> SATA Storage
                    </h4>
                  </div>
                  {(overview?.storage ?? []).map((hdd) => (
                    <div key={hdd.id} className="kv-list">
                      <div>
                        <dt>
                          Disk #{hdd.id} ({hdd.name})
                        </dt>
                        <dd>
                          <span className="status-badge ok">{hdd.status.toUpperCase()}</span>
                        </dd>
                      </div>
                      <div>
                        <dt>Capacity</dt>
                        <dd>{(hdd.capacityMb / 1024).toFixed(1)} GB</dd>
                      </div>
                      <div>
                        <dt>Mode</dt>
                        <dd>{hdd.property}</dd>
                      </div>
                    </div>
                  ))}
                </div>

                <div className="mobile-card">
                  <div className="mobile-card-header">
                    <h4>
                      <Wifi size={15} /> Network & Maintenance
                    </h4>
                  </div>
                  <dl className="kv-list">
                    <div>
                      <dt>IPv4 Address</dt>
                      <dd className="mono">{overview?.network?.ipAddress ?? '—'}</dd>
                    </div>
                    <div>
                      <dt>Gateway</dt>
                      <dd className="mono">{overview?.network?.defaultGateway ?? '—'}</dd>
                    </div>
                    <div>
                      <dt>MAC Address</dt>
                      <dd className="mono">{overview?.network?.macAddress ?? '—'}</dd>
                    </div>
                  </dl>
                  <button
                    type="button"
                    className="btn-danger full-width"
                    style={{ marginTop: 12 }}
                    onClick={() => dvr.rebootDevice()}
                  >
                    <Power size={14} /> Reboot TV-DVR208
                  </button>
                </div>
              </>
            )}

            {configSubTab === 'motion' && motionCfg && (
              <div className="mobile-card">
                <div className="mobile-card-header">
                  <h4>
                    <Grid size={15} /> CH {selectedChannelId} Motion Grid (
                    {countActiveCells(motionCfg.grid)} / {MOTION_ROWS * MOTION_COLS})
                  </h4>
                </div>

                <div className="editor-viewport">
                  <img
                    src={activeSnapUrl}
                    alt={`CH ${selectedChannelId}`}
                    className="editor-bg-img"
                  />
                  <svg
                    viewBox={`0 0 ${MOTION_COLS * 10} ${MOTION_ROWS * 10}`}
                    preserveAspectRatio="none"
                    className="editor-svg"
                    data-testid="mobile-motion-svg"
                  >
                    {motionCfg.grid.map((row, rIdx) =>
                      row.map((active, cIdx) => (
                        <rect
                          key={`${rIdx}-${cIdx}`}
                          data-testid={`mobile-motion-cell-${rIdx}-${cIdx}`}
                          x={cIdx * 10}
                          y={rIdx * 10}
                          width={10}
                          height={10}
                          fill={
                            active
                              ? 'rgba(239, 68, 68, 0.38)'
                              : 'rgba(15, 23, 42, 0.15)'
                          }
                          stroke="rgba(255, 255, 255, 0.25)"
                          strokeWidth={0.4}
                          onClick={() => toggleMotionCellAt(rIdx, cIdx)}
                        />
                      ))
                    )}
                  </svg>
                </div>

                <div className="form-row" style={{ marginTop: 12 }}>
                  <label>
                    <input
                      type="checkbox"
                      checked={motionCfg.enabled}
                      onChange={(e) =>
                        setMotionCfg({ ...motionCfg, enabled: e.target.checked })
                      }
                    />{' '}
                    Enable Motion Detection (Sensitivity: {motionCfg.sensitivityLevel})
                  </label>
                  <input
                    type="range"
                    min={0}
                    max={5}
                    value={motionCfg.sensitivityLevel}
                    onChange={(e) =>
                      setMotionCfg({
                        ...motionCfg,
                        sensitivityLevel: Number(e.target.value),
                      })
                    }
                  />
                </div>

                <div className="btn-row">
                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => {
                      const g = createMotionGrid(true);
                      setMotionCfg({ ...motionCfg, grid: g, gridMapHex: motionGridToHex(g) });
                    }}
                  >
                    Select All
                  </button>
                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => {
                      const g = createMotionGrid(false);
                      setMotionCfg({ ...motionCfg, grid: g, gridMapHex: motionGridToHex(g) });
                    }}
                  >
                    Clear All
                  </button>
                </div>

                <button
                  type="button"
                  className="btn-primary full-width"
                  onClick={handleSaveMotion}
                >
                  Save Motion Detection
                </button>
              </div>
            )}

            {configSubTab === 'osd' && overlayCfg && (
              <div className="mobile-card">
                <div className="mobile-card-header">
                  <h4>
                    <Sliders size={15} /> CH {selectedChannelId} OSD Overlays
                  </h4>
                </div>

                <div className="form-row">
                  <label>
                    <input
                      type="checkbox"
                      checked={overlayCfg.dateTime.enabled}
                      onChange={(e) =>
                        setOverlayCfg({
                          ...overlayCfg,
                          dateTime: { ...overlayCfg.dateTime, enabled: e.target.checked },
                        })
                      }
                    />{' '}
                    Show Date & Time Stamp
                  </label>
                </div>

                <div className="form-row">
                  <label>
                    <input
                      type="checkbox"
                      checked={overlayCfg.channelName.enabled}
                      onChange={(e) =>
                        setOverlayCfg({
                          ...overlayCfg,
                          channelName: {
                            ...overlayCfg.channelName,
                            enabled: e.target.checked,
                          },
                        })
                      }
                    />{' '}
                    Show Camera Name Overlay
                  </label>
                </div>

                <button
                  type="button"
                  className="btn-primary full-width"
                  onClick={handleSaveOverlays}
                >
                  Save OSD Overlays
                </button>
              </div>
            )}
          </div>
        )}
      </main>

      {/* Fixed Bottom Mobile Tab Navigation Bar */}
      <nav className="mobile-bottom-nav" aria-label="Mobile Navigation">
        <button
          type="button"
          className={`mobile-nav-item ${activeTab === 'live' ? 'active' : ''}`}
          onClick={() => setActiveTab('live')}
        >
          <Radio size={18} />
          <span>Live</span>
        </button>
        <button
          type="button"
          className={`mobile-nav-item ${activeTab === 'ptz' ? 'active' : ''}`}
          onClick={() => setActiveTab('ptz')}
        >
          <Move size={18} />
          <span>PTZ</span>
        </button>
        <button
          type="button"
          className={`mobile-nav-item ${activeTab === 'archive' ? 'active' : ''}`}
          onClick={() => setActiveTab('archive')}
        >
          <Film size={18} />
          <span>Archive</span>
        </button>
        <button
          type="button"
          className={`mobile-nav-item ${activeTab === 'config' ? 'active' : ''}`}
          onClick={() => setActiveTab('config')}
        >
          <Settings size={18} />
          <span>System</span>
        </button>
      </nav>
    </div>
  );
};

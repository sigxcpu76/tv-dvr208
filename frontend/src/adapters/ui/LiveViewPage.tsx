import React, { useState } from 'react';
import {
  Camera,
  Download,
  Grid,
  Maximize2,
  Move,
  RefreshCw,
  Square,
  Video,
  ZoomIn,
  ZoomOut,
} from 'lucide-react';
import { useDvrPort } from '../../application/DvrContext';
import { NORMALIZED_HEIGHT, NORMALIZED_WIDTH, VideoChannel } from '../../domain/models';
import { ManagedVideo } from './ManagedVideo';

interface LiveViewPageProps {
  channels: VideoChannel[];
}

export const LiveViewPage: React.FC<LiveViewPageProps> = ({ channels }) => {
  const dvr = useDvrPort();
  const [gridSize, setGridSize] = useState<1 | 4 | 8>(8);
  const [selectedChannelId, setSelectedChannelId] = useState<number>(1);
  const [streamType, setStreamType] = useState<1 | 2>(2);
  const [renderMode, setRenderMode] = useState<'video' | 'snapshot'>('video');
  const [snapshotTick, setSnapshotTick] = useState<number>(0);
  const [ptzSpeed, setPtzSpeed] = useState<number>(50);
  const [ptzStatus, setPtzStatus] = useState<string>('Ready');
  const [zoom3dMode, setZoom3dMode] = useState<boolean>(false);
  const [dragStart, setDragStart] = useState<{ x: number; y: number } | null>(null);
  const [dragCurrent, setDragCurrent] = useState<{ x: number; y: number } | null>(null);

  const visibleChannels =
    gridSize === 1
      ? channels.filter((c) => c.id === selectedChannelId)
      : channels.slice(0, gridSize);

  const handlePtzMove = async (pan: number, tilt: number, zoom: number, label: string) => {
    try {
      setPtzStatus(`${label} (Ch ${selectedChannelId})`);
      await dvr.ptzMove(selectedChannelId, { pan, tilt, zoom });
    } catch (e) {
      setPtzStatus(`PTZ Error: ${(e as Error).message}`);
    }
  };

  const handlePtzStop = async () => {
    try {
      await dvr.ptzStop(selectedChannelId);
      setPtzStatus(`Stopped (Ch ${selectedChannelId})`);
    } catch (e) {
      setPtzStatus(`Stop Error: ${(e as Error).message}`);
    }
  };

  const handlePreset = async (presetId: number, action: 'goto' | 'set') => {
    try {
      await dvr.ptzPreset(selectedChannelId, presetId, action);
      setPtzStatus(`${action === 'goto' ? 'Called' : 'Saved'} Preset ${presetId} (Ch ${selectedChannelId})`);
    } catch (e) {
      setPtzStatus(`Preset Error: ${(e as Error).message}`);
    }
  };

  const handleSvgMouseDown = (e: React.MouseEvent<SVGSVGElement>, chId: number) => {
    if (!zoom3dMode || chId !== selectedChannelId) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const x = Math.round(((e.clientX - rect.left) / rect.width) * NORMALIZED_WIDTH);
    const y = Math.round(((e.clientY - rect.top) / rect.height) * NORMALIZED_HEIGHT);
    setDragStart({ x, y });
    setDragCurrent({ x, y });
  };

  const handleSvgMouseMove = (e: React.MouseEvent<SVGSVGElement>) => {
    if (!dragStart) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const x = Math.round(((e.clientX - rect.left) / rect.width) * NORMALIZED_WIDTH);
    const y = Math.round(((e.clientY - rect.top) / rect.height) * NORMALIZED_HEIGHT);
    setDragCurrent({ x, y });
  };

  const handleSvgMouseUp = async () => {
    if (!dragStart || !dragCurrent) return;
    const pos = {
      startX: dragStart.x,
      startY: dragStart.y,
      endX: dragCurrent.x,
      endY: dragCurrent.y,
    };
    setDragStart(null);
    setDragCurrent(null);
    try {
      await dvr.ptzZoom3D(selectedChannelId, pos);
      setPtzStatus(`3D Zoom applied on Ch ${selectedChannelId}`);
    } catch (e) {
      setPtzStatus(`3D Zoom error: ${(e as Error).message}`);
    }
  };

  return (
    <div className="live-layout">
      <div className="live-main">
        {/* Top Control Bar */}
        <div className="toolbar">
          <div className="toolbar-group">
            <span className="toolbar-label">Grid Layout:</span>
            <button
              type="button"
              className={`btn-chip ${gridSize === 1 ? 'active' : ''}`}
              onClick={() => setGridSize(1)}
              aria-label="1x1 View"
            >
              <Square size={14} /> 1×1
            </button>
            <button
              type="button"
              className={`btn-chip ${gridSize === 4 ? 'active' : ''}`}
              onClick={() => setGridSize(4)}
              aria-label="2x2 View"
            >
              <Grid size={14} /> 2×2
            </button>
            <button
              type="button"
              className={`btn-chip ${gridSize === 8 ? 'active' : ''}`}
              onClick={() => setGridSize(8)}
              aria-label="8 Camera View"
            >
              <Grid size={14} /> 8-Ch
            </button>
          </div>

          <div className="toolbar-group">
            <span className="toolbar-label">Stream:</span>
            <button
              type="button"
              className={`btn-chip ${streamType === 2 ? 'active' : ''}`}
              onClick={() => setStreamType(2)}
            >
              Sub (960×480)
            </button>
            <button
              type="button"
              className={`btn-chip ${streamType === 1 ? 'active' : ''}`}
              onClick={() => setStreamType(1)}
            >
              Main (1080p HD)
            </button>
          </div>

          <div className="toolbar-group">
            <span className="toolbar-label">Mode:</span>
            <button
              type="button"
              className={`btn-chip ${renderMode === 'video' ? 'active' : ''}`}
              onClick={() => setRenderMode('video')}
            >
              <Video size={14} /> Live fMP4
            </button>
            <button
              type="button"
              className={`btn-chip ${renderMode === 'snapshot' ? 'active' : ''}`}
              onClick={() => setRenderMode('snapshot')}
            >
              <Camera size={14} /> Snapshot
            </button>
            {renderMode === 'snapshot' && (
              <button
                type="button"
                className="btn-chip"
                onClick={() => setSnapshotTick((t) => t + 1)}
                title="Refresh Snapshots"
              >
                <RefreshCw size={14} /> Refresh
              </button>
            )}
          </div>
        </div>

        {/* Multi-Camera Video Grid */}
        <div className={`camera-grid grid-${gridSize}`}>
          {visibleChannels.map((ch) => {
            const isSelected = ch.id === selectedChannelId;
            const activeStream = gridSize === 1 ? 1 : streamType;
            const streamInfo = activeStream === 1 ? ch.mainStream : ch.subStream;
            const liveUrl = dvr.getLiveStreamUrl(ch.id, activeStream);
            const snapUrl = `${dvr.getSnapshotUrl(ch.id, 1, false)}&tick=${snapshotTick}`;
            const downloadSnapUrl = dvr.getSnapshotUrl(ch.id, 1, true);

            return (
              <div
                key={ch.id}
                data-testid={`camera-tile-${ch.id}`}
                className={`camera-tile ${isSelected ? 'selected' : ''}`}
                onClick={() => setSelectedChannelId(ch.id)}
                onDoubleClick={() => setGridSize((prev) => (prev === 1 ? 8 : 1))}
              >
                <div className="camera-tile-header">
                  <div className="camera-title">
                    <span className="ch-badge">CH {ch.id}</span>
                    <strong>{ch.name}</strong>
                    <span className="codec-tag">
                      {streamInfo
                        ? `${streamInfo.videoCodecType} ${streamInfo.resolutionWidth}×${streamInfo.resolutionHeight}`
                        : ch.resDesc}
                    </span>
                  </div>
                  <div className="camera-actions">
                    <a
                      href={downloadSnapUrl}
                      className="icon-btn"
                      title="Download 1080p JPEG Snapshot"
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Download size={14} />
                    </a>
                    <button
                      type="button"
                      className="icon-btn"
                      title="Toggle Single/Multi View"
                      onClick={(e) => {
                        e.stopPropagation();
                        setSelectedChannelId(ch.id);
                        setGridSize((prev) => (prev === 1 ? 8 : 1));
                      }}
                    >
                      <Maximize2 size={14} />
                    </button>
                  </div>
                </div>

                <div className="camera-viewport">
                  {renderMode === 'video' && (gridSize <= 4 || isSelected) ? (
                    <ManagedVideo
                      key={`${ch.id}-${activeStream}`}
                      src={liveUrl}
                      poster={snapUrl}
                      autoPlay
                      muted
                      playsInline
                      controls={gridSize === 1}
                      className="camera-video"
                    />
                  ) : (
                    <img src={snapUrl} alt={`Camera ${ch.name}`} className="camera-video" />
                  )}

                  {zoom3dMode && isSelected && (
                    <svg
                      viewBox={`0 0 ${NORMALIZED_WIDTH} ${NORMALIZED_HEIGHT}`}
                      preserveAspectRatio="none"
                      className="video-svg-overlay"
                      onMouseDown={(e) => handleSvgMouseDown(e, ch.id)}
                      onMouseMove={handleSvgMouseMove}
                      onMouseUp={handleSvgMouseUp}
                    >
                      {dragStart && dragCurrent && (
                        <rect
                          x={Math.min(dragStart.x, dragCurrent.x)}
                          y={Math.min(dragStart.y, dragCurrent.y)}
                          width={Math.abs(dragCurrent.x - dragStart.x)}
                          height={Math.abs(dragCurrent.y - dragStart.y)}
                          fill="rgba(56, 189, 248, 0.25)"
                          stroke="#38bdf8"
                          strokeWidth={2}
                        />
                      )}
                    </svg>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* PTZ & Channel Inspector Sidebar */}
      <aside className="ptz-sidebar">
        <div className="panel-card">
          <h3>
            <Move size={16} /> PTZ Controller — CH {selectedChannelId}
          </h3>
          <p className="muted-text">
            Active Camera:{' '}
            <strong>
              {channels.find((c) => c.id === selectedChannelId)?.name ?? `Channel ${selectedChannelId}`}
            </strong>
          </p>

          <div className="ptz-pad">
            <button
              type="button"
              className="ptz-btn"
              onMouseDown={() => handlePtzMove(-ptzSpeed, ptzSpeed, 0, 'Up-Left')}
              onMouseUp={handlePtzStop}
            >
              ↖
            </button>
            <button
              type="button"
              className="ptz-btn"
              aria-label="PTZ Up"
              onMouseDown={() => handlePtzMove(0, ptzSpeed, 0, 'Tilt Up')}
              onMouseUp={handlePtzStop}
            >
              ▲
            </button>
            <button
              type="button"
              className="ptz-btn"
              onMouseDown={() => handlePtzMove(ptzSpeed, ptzSpeed, 0, 'Up-Right')}
              onMouseUp={handlePtzStop}
            >
              ↗
            </button>

            <button
              type="button"
              className="ptz-btn"
              aria-label="PTZ Left"
              onMouseDown={() => handlePtzMove(-ptzSpeed, 0, 0, 'Pan Left')}
              onMouseUp={handlePtzStop}
            >
              ◀
            </button>
            <button
              type="button"
              className="ptz-btn stop"
              aria-label="PTZ Stop"
              onClick={handlePtzStop}
            >
              ■
            </button>
            <button
              type="button"
              className="ptz-btn"
              aria-label="PTZ Right"
              onMouseDown={() => handlePtzMove(ptzSpeed, 0, 0, 'Pan Right')}
              onMouseUp={handlePtzStop}
            >
              ▶
            </button>

            <button
              type="button"
              className="ptz-btn"
              onMouseDown={() => handlePtzMove(-ptzSpeed, -ptzSpeed, 0, 'Down-Left')}
              onMouseUp={handlePtzStop}
            >
              ↙
            </button>
            <button
              type="button"
              className="ptz-btn"
              aria-label="PTZ Down"
              onMouseDown={() => handlePtzMove(0, -ptzSpeed, 0, 'Tilt Down')}
              onMouseUp={handlePtzStop}
            >
              ▼
            </button>
            <button
              type="button"
              className="ptz-btn"
              onMouseDown={() => handlePtzMove(ptzSpeed, -ptzSpeed, 0, 'Down-Right')}
              onMouseUp={handlePtzStop}
            >
              ↘
            </button>
          </div>

          <div className="ptz-zoom-row">
            <button
              type="button"
              className="btn-secondary"
              onMouseDown={() => handlePtzMove(0, 0, ptzSpeed, 'Zoom In')}
              onMouseUp={handlePtzStop}
            >
              <ZoomIn size={14} /> Zoom +
            </button>
            <button
              type="button"
              className="btn-secondary"
              onMouseDown={() => handlePtzMove(0, 0, -ptzSpeed, 'Zoom Out')}
              onMouseUp={handlePtzStop}
            >
              <ZoomOut size={14} /> Zoom -
            </button>
          </div>

          <div className="form-row">
            <label htmlFor="ptz-speed">PTZ Speed: {ptzSpeed}%</label>
            <input
              id="ptz-speed"
              type="range"
              min={10}
              max={100}
              step={10}
              value={ptzSpeed}
              onChange={(e) => setPtzSpeed(Number(e.target.value))}
            />
          </div>

          <div className="form-row">
            <button
              type="button"
              className={`btn-chip full-width ${zoom3dMode ? 'active' : ''}`}
              onClick={() => setZoom3dMode((v) => !v)}
            >
              {zoom3dMode ? '3D Box Zoom: ON (Drag on Video)' : 'Enable 3D Box Zoom'}
            </button>
          </div>

          <div className="presets-section">
            <span className="toolbar-label">Presets:</span>
            {[1, 2, 3, 4].map((p) => (
              <div key={p} className="preset-row">
                <span>Preset #{p}</span>
                <div>
                  <button
                    type="button"
                    className="btn-xs"
                    onClick={() => handlePreset(p, 'goto')}
                  >
                    Call
                  </button>
                  <button
                    type="button"
                    className="btn-xs"
                    onClick={() => handlePreset(p, 'set')}
                  >
                    Set
                  </button>
                </div>
              </div>
            ))}
          </div>

          <div className="status-pill" data-testid="ptz-status">
            {ptzStatus}
          </div>
        </div>
      </aside>
    </div>
  );
};

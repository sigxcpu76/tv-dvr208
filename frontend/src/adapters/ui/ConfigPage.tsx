import React, { useEffect, useState } from 'react';
import {
  Activity,
  CheckCircle2,
  Cpu,
  EyeOff,
  HardDrive,
  Network,
  Power,
  Save,
  Shield,
  Type,
} from 'lucide-react';
import { useDvrPort } from '../../application/DvrContext';
import {
  MOTION_COLS,
  MOTION_ROWS,
  MotionDetectionConfig,
  NORMALIZED_HEIGHT,
  NORMALIZED_WIDTH,
  PrivacyMaskConfig,
  SystemOverview,
  VideoChannel,
  VideoOverlayConfig,
} from '../../domain/models';
import {
  countActiveCells,
  createMotionGrid,
  motionGridToHex,
  setMotionCell,
} from '../../domain/motionGrid';

interface ConfigPageProps {
  channels: VideoChannel[];
  overview: SystemOverview | null;
}

type EditorTab = 'motion' | 'osd' | 'privacy' | 'system';

export const ConfigPage: React.FC<ConfigPageProps> = ({ channels, overview }) => {
  const dvr = useDvrPort();

  const [activeTab, setActiveTab] = useState<EditorTab>('motion');
  const [channelId, setChannelId] = useState<number>(1);
  const [statusMsg, setStatusMsg] = useState<string>('');
  const [isPainting, setIsPainting] = useState<boolean>(false);
  const [paintValue, setPaintValue] = useState<boolean>(true);

  const [motionCfg, setMotionCfg] = useState<MotionDetectionConfig | null>(null);
  const [overlayCfg, setOverlayCfg] = useState<VideoOverlayConfig | null>(null);
  const [privacyCfg, setPrivacyCfg] = useState<PrivacyMaskConfig | null>(null);

  useEffect(() => {
    let mounted = true;
    setStatusMsg('Loading channel configuration...');
    Promise.all([
      dvr.getChannelMotion(channelId),
      dvr.getChannelOverlays(channelId),
      dvr.getChannelPrivacyMask(channelId),
    ])
      .then(([m, o, p]) => {
        if (!mounted) return;
        setMotionCfg(m);
        setOverlayCfg(o);
        setPrivacyCfg(p);
        setStatusMsg('');
      })
      .catch((err) => {
        if (!mounted) return;
        setStatusMsg(`Failed to load config: ${(err as Error).message}`);
      });
    return () => {
      mounted = false;
    };
  }, [dvr, channelId]);

  const handleSaveMotion = async () => {
    if (!motionCfg) return;
    try {
      const updated: MotionDetectionConfig = {
        ...motionCfg,
        gridMapHex: motionGridToHex(motionCfg.grid),
      };
      await dvr.updateChannelMotion(channelId, updated);
      setMotionCfg(updated);
      setStatusMsg(`Saved Motion Detection for Channel ${channelId}`);
    } catch (e) {
      setStatusMsg(`Save failed: ${(e as Error).message}`);
    }
  };

  const handleSaveOverlays = async () => {
    if (!overlayCfg) return;
    try {
      await dvr.updateChannelOverlays(channelId, overlayCfg);
      setStatusMsg(`Saved OSD Overlays for Channel ${channelId}`);
    } catch (e) {
      setStatusMsg(`Save failed: ${(e as Error).message}`);
    }
  };

  const handleSavePrivacy = async () => {
    if (!privacyCfg) return;
    try {
      await dvr.updateChannelPrivacyMask(channelId, privacyCfg);
      setStatusMsg(`Saved Privacy Masks for Channel ${channelId}`);
    } catch (e) {
      setStatusMsg(`Save failed: ${(e as Error).message}`);
    }
  };

  const toggleMotionCellAt = (r: number, c: number, val: boolean) => {
    if (!motionCfg) return;
    const nextGrid = setMotionCell(motionCfg.grid, r, c, val);
    setMotionCfg({
      ...motionCfg,
      grid: nextGrid,
      gridMapHex: motionGridToHex(nextGrid),
    });
  };

  const activeChannelName =
    channels.find((c) => c.id === channelId)?.name ?? `Channel ${channelId}`;
  const snapshotBgUrl = dvr.getSnapshotUrl(channelId, 1, false);

  return (
    <div className="config-layout">
      {/* Sub-navigation bar */}
      <div className="toolbar">
        <div className="toolbar-group">
          <button
            type="button"
            className={`btn-chip ${activeTab === 'motion' ? 'active' : ''}`}
            onClick={() => setActiveTab('motion')}
          >
            <Activity size={14} /> Motion Grid (22×18)
          </button>
          <button
            type="button"
            className={`btn-chip ${activeTab === 'osd' ? 'active' : ''}`}
            onClick={() => setActiveTab('osd')}
          >
            <Type size={14} /> OSD Overlays
          </button>
          <button
            type="button"
            className={`btn-chip ${activeTab === 'privacy' ? 'active' : ''}`}
            onClick={() => setActiveTab('privacy')}
          >
            <EyeOff size={14} /> Privacy Masks
          </button>
          <button
            type="button"
            className={`btn-chip ${activeTab === 'system' ? 'active' : ''}`}
            onClick={() => setActiveTab('system')}
          >
            <Cpu size={14} /> System, Storage & Network
          </button>
        </div>

        {activeTab !== 'system' && (
          <div className="toolbar-group">
            <label htmlFor="cfg-channel-select" className="toolbar-label">
              Camera:
            </label>
            <select
              id="cfg-channel-select"
              value={channelId}
              onChange={(e) => setChannelId(Number(e.target.value))}
            >
              {channels.map((ch) => (
                <option key={ch.id} value={ch.id}>
                  CH {ch.id} — {ch.name}
                </option>
              ))}
            </select>
          </div>
        )}

        {statusMsg && (
          <div className="status-pill" data-testid="config-status">
            <CheckCircle2 size={14} /> {statusMsg}
          </div>
        )}
      </div>

      {activeTab !== 'system' ? (
        <div className="visual-editor-grid">
          {/* Interactive SVG over Camera Preview Canvas */}
          <div className="panel-card editor-canvas-card">
            <h3>
              HTML5 SVG Visual Region Editor — CH {channelId} ({activeChannelName})
            </h3>
            <div
              className="editor-viewport"
              onMouseUp={() => setIsPainting(false)}
              onMouseLeave={() => setIsPainting(false)}
            >
              <img
                src={snapshotBgUrl}
                alt={`Channel ${channelId} Reference`}
                className="editor-bg-img"
              />

              {/* Mode 1: 22x18 Motion Detection Grid SVG */}
              {activeTab === 'motion' && motionCfg && (
                <svg
                  viewBox={`0 0 ${MOTION_COLS * 10} ${MOTION_ROWS * 10}`}
                  preserveAspectRatio="none"
                  className="editor-svg"
                  data-testid="motion-svg-grid"
                >
                  {motionCfg.grid.map((row, rIdx) =>
                    row.map((active, cIdx) => (
                      <rect
                        key={`${rIdx}-${cIdx}`}
                        data-testid={`motion-cell-${rIdx}-${cIdx}`}
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
                        onMouseDown={() => {
                          const nextVal = !active;
                          setIsPainting(true);
                          setPaintValue(nextVal);
                          toggleMotionCellAt(rIdx, cIdx, nextVal);
                        }}
                        onMouseEnter={() => {
                          if (isPainting) {
                            toggleMotionCellAt(rIdx, cIdx, paintValue);
                          }
                        }}
                      />
                    ))
                  )}
                </svg>
              )}

              {/* Mode 2: 704x576 OSD Overlays SVG */}
              {activeTab === 'osd' && overlayCfg && (
                <svg
                  viewBox={`0 0 ${NORMALIZED_WIDTH} ${NORMALIZED_HEIGHT}`}
                  preserveAspectRatio="none"
                  className="editor-svg"
                  data-testid="osd-svg-canvas"
                >
                  {overlayCfg.dateTime.enabled && (
                    <g
                      transform={`translate(${overlayCfg.dateTime.positionX}, ${overlayCfg.dateTime.positionY})`}
                    >
                      <rect
                        x={0}
                        y={0}
                        width={260}
                        height={34}
                        rx={4}
                        fill="rgba(15, 23, 42, 0.75)"
                        stroke="#38bdf8"
                        strokeWidth={2}
                      />
                      <text x={10} y={22} fill="#f8fafc" fontSize={18} fontFamily="monospace">
                        2026-11-07 16:00:00
                      </text>
                    </g>
                  )}

                  {overlayCfg.channelName.enabled && (
                    <g
                      transform={`translate(${overlayCfg.channelName.positionX}, ${overlayCfg.channelName.positionY})`}
                    >
                      <rect
                        x={0}
                        y={0}
                        width={175}
                        height={34}
                        rx={4}
                        fill="rgba(15, 23, 42, 0.75)"
                        stroke="#22c55e"
                        strokeWidth={2}
                      />
                      <text x={10} y={22} fill="#f8fafc" fontSize={18} fontWeight="bold">
                        {activeChannelName}
                      </text>
                    </g>
                  )}
                </svg>
              )}

              {/* Mode 3: 704x576 Privacy Masks SVG */}
              {activeTab === 'privacy' && privacyCfg && (
                <svg
                  viewBox={`0 0 ${NORMALIZED_WIDTH} ${NORMALIZED_HEIGHT}`}
                  preserveAspectRatio="none"
                  className="editor-svg"
                >
                  {privacyCfg.regions
                    .filter((r) => r.enabled)
                    .map((r) => (
                      <g key={r.id}>
                        <rect
                          x={r.x}
                          y={r.y}
                          width={r.width}
                          height={r.height}
                          fill="rgba(15, 23, 42, 0.88)"
                          stroke="#f59e0b"
                          strokeWidth={2}
                        />
                        <text
                          x={r.x + 8}
                          y={r.y + 22}
                          fill="#f59e0b"
                          fontSize={16}
                          fontWeight="bold"
                        >
                          Mask #{r.id}
                        </text>
                      </g>
                    ))}
                </svg>
              )}
            </div>
          </div>

          {/* Right Inspector Form for Selected Visual Editor */}
          <div className="panel-card">
            {activeTab === 'motion' && motionCfg && (
              <div>
                <h3>Motion Detection Parameters</h3>
                <p className="muted-text">
                  Click or drag across the 22×18 grid over the camera view to arm or disarm zones.
                </p>

                <div className="form-row checkbox-row">
                  <label>
                    <input
                      type="checkbox"
                      checked={motionCfg.enabled}
                      onChange={(e) =>
                        setMotionCfg({ ...motionCfg, enabled: e.target.checked })
                      }
                    />
                    Enable Motion Detection
                  </label>
                </div>

                <div className="form-row checkbox-row">
                  <label>
                    <input
                      type="checkbox"
                      checked={motionCfg.enableHighlight}
                      onChange={(e) =>
                        setMotionCfg({
                          ...motionCfg,
                          enableHighlight: e.target.checked,
                        })
                      }
                    />
                    Highlight Dynamic Motion in Live View
                  </label>
                </div>

                <div className="form-row">
                  <label htmlFor="motion-sensitivity">
                    Sensitivity Level: {motionCfg.sensitivityLevel}
                  </label>
                  <input
                    id="motion-sensitivity"
                    type="range"
                    min={0}
                    max={5}
                    step={1}
                    value={motionCfg.sensitivityLevel}
                    onChange={(e) =>
                      setMotionCfg({
                        ...motionCfg,
                        sensitivityLevel: Number(e.target.value),
                      })
                    }
                  />
                </div>

                <div className="form-row">
                  <span>
                    Active Grid Cells:{' '}
                    <strong>
                      {countActiveCells(motionCfg.grid)} / {MOTION_ROWS * MOTION_COLS}
                    </strong>
                  </span>
                  <div className="btn-row">
                    <button
                      type="button"
                      className="btn-secondary"
                      onClick={() => {
                        const full = createMotionGrid(true);
                        setMotionCfg({
                          ...motionCfg,
                          grid: full,
                          gridMapHex: motionGridToHex(full),
                        });
                      }}
                    >
                      Select All
                    </button>
                    <button
                      type="button"
                      className="btn-secondary"
                      onClick={() => {
                        const empty = createMotionGrid(false);
                        setMotionCfg({
                          ...motionCfg,
                          grid: empty,
                          gridMapHex: motionGridToHex(empty),
                        });
                      }}
                    >
                      Clear All
                    </button>
                  </div>
                </div>

                <button
                  type="button"
                  className="btn-primary full-width"
                  onClick={handleSaveMotion}
                >
                  <Save size={15} /> Save Motion Detection
                </button>
              </div>
            )}

            {activeTab === 'osd' && overlayCfg && (
              <div>
                <h3>OSD Overlay Coordinates (704×576)</h3>

                <div className="form-row checkbox-row">
                  <label>
                    <input
                      type="checkbox"
                      checked={overlayCfg.dateTime.enabled}
                      onChange={(e) =>
                        setOverlayCfg({
                          ...overlayCfg,
                          dateTime: {
                            ...overlayCfg.dateTime,
                            enabled: e.target.checked,
                          },
                        })
                      }
                    />
                    Show Date & Time Overlay
                  </label>
                </div>

                <div className="coord-grid">
                  <div>
                    <label htmlFor="dt-x">Timestamp X</label>
                    <input
                      id="dt-x"
                      type="number"
                      min={0}
                      max={NORMALIZED_WIDTH}
                      value={overlayCfg.dateTime.positionX}
                      onChange={(e) =>
                        setOverlayCfg({
                          ...overlayCfg,
                          dateTime: {
                            ...overlayCfg.dateTime,
                            positionX: Number(e.target.value),
                          },
                        })
                      }
                    />
                  </div>
                  <div>
                    <label htmlFor="dt-y">Timestamp Y</label>
                    <input
                      id="dt-y"
                      type="number"
                      min={0}
                      max={NORMALIZED_HEIGHT}
                      value={overlayCfg.dateTime.positionY}
                      onChange={(e) =>
                        setOverlayCfg({
                          ...overlayCfg,
                          dateTime: {
                            ...overlayCfg.dateTime,
                            positionY: Number(e.target.value),
                          },
                        })
                      }
                    />
                  </div>
                </div>

                <div className="form-row checkbox-row">
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
                    />
                    Show Camera Name Overlay
                  </label>
                </div>

                <div className="coord-grid">
                  <div>
                    <label htmlFor="ch-x">Camera Name X</label>
                    <input
                      id="ch-x"
                      type="number"
                      min={0}
                      max={NORMALIZED_WIDTH}
                      value={overlayCfg.channelName.positionX}
                      onChange={(e) =>
                        setOverlayCfg({
                          ...overlayCfg,
                          channelName: {
                            ...overlayCfg.channelName,
                            positionX: Number(e.target.value),
                          },
                        })
                      }
                    />
                  </div>
                  <div>
                    <label htmlFor="ch-y">Camera Name Y</label>
                    <input
                      id="ch-y"
                      type="number"
                      min={0}
                      max={NORMALIZED_HEIGHT}
                      value={overlayCfg.channelName.positionY}
                      onChange={(e) =>
                        setOverlayCfg({
                          ...overlayCfg,
                          channelName: {
                            ...overlayCfg.channelName,
                            positionY: Number(e.target.value),
                          },
                        })
                      }
                    />
                  </div>
                </div>

                <button
                  type="button"
                  className="btn-primary full-width"
                  onClick={handleSaveOverlays}
                >
                  <Save size={15} /> Save OSD Overlays
                </button>
              </div>
            )}

            {activeTab === 'privacy' && privacyCfg && (
              <div>
                <h3>Privacy Mask Regions</h3>
                <div className="form-row checkbox-row">
                  <label>
                    <input
                      type="checkbox"
                      checked={privacyCfg.enabled}
                      onChange={(e) =>
                        setPrivacyCfg({ ...privacyCfg, enabled: e.target.checked })
                      }
                    />
                    Enable Privacy Masking
                  </label>
                </div>

                <div className="btn-row">
                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => {
                      if (privacyCfg.regions.length >= 4) return;
                      const nextId = privacyCfg.regions.length + 1;
                      setPrivacyCfg({
                        ...privacyCfg,
                        enabled: true,
                        regions: [
                          ...privacyCfg.regions,
                          {
                            id: nextId,
                            enabled: true,
                            x: 80 + nextId * 40,
                            y: 80 + nextId * 30,
                            width: 160,
                            height: 120,
                          },
                        ],
                      });
                    }}
                  >
                    + Add Mask Box
                  </button>
                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => setPrivacyCfg({ ...privacyCfg, regions: [] })}
                  >
                    Clear All
                  </button>
                </div>

                <button
                  type="button"
                  className="btn-primary full-width"
                  onClick={handleSavePrivacy}
                >
                  <Save size={15} /> Save Privacy Masks
                </button>
              </div>
            )}
          </div>
        </div>
      ) : (
        /* System, Storage, Network & Users Cards */
        <div className="system-cards-grid">
          <div className="panel-card">
            <h3>
              <Cpu size={16} /> Hardware & Firmware Info
            </h3>
            {overview?.deviceInfo ? (
              <dl className="kv-list">
                <div>
                  <dt>Device Name</dt>
                  <dd>{overview.deviceInfo.deviceName}</dd>
                </div>
                <div>
                  <dt>Model</dt>
                  <dd>{overview.deviceInfo.model}</dd>
                </div>
                <div>
                  <dt>Serial Number</dt>
                  <dd className="mono">{overview.deviceInfo.serialNumber}</dd>
                </div>
                <div>
                  <dt>Firmware</dt>
                  <dd>
                    {overview.deviceInfo.firmwareVersion} (
                    {overview.deviceInfo.firmwareReleasedDate})
                  </dd>
                </div>
                <div>
                  <dt>Encoder Version</dt>
                  <dd>
                    {overview.deviceInfo.encoderVersion} (
                    {overview.deviceInfo.encoderReleasedDate})
                  </dd>
                </div>
                <div>
                  <dt>Clock / Mode</dt>
                  <dd>
                    {overview.systemTime?.localTime} ({overview.systemTime?.timeMode})
                  </dd>
                </div>
              </dl>
            ) : (
              <p className="muted-text">Loading device information...</p>
            )}
          </div>

          <div className="panel-card">
            <h3>
              <HardDrive size={16} /> Storage Volumes (SATA HDD)
            </h3>
            {overview?.storage?.map((hdd) => (
              <div key={hdd.id} className="hdd-card">
                <div className="hdd-header">
                  <strong>
                    Disk #{hdd.id} ({hdd.name}) — {hdd.type}
                  </strong>
                  <span className="status-badge ok">{hdd.status.toUpperCase()}</span>
                </div>
                <dl className="kv-list">
                  <div>
                    <dt>Capacity</dt>
                    <dd>{(hdd.capacityMb / 1024).toFixed(1)} GB</dd>
                  </div>
                  <div>
                    <dt>Mode / Property</dt>
                    <dd>{hdd.property} (Cyclic Overwrite)</dd>
                  </div>
                </dl>
              </div>
            ))}
          </div>

          <div className="panel-card">
            <h3>
              <Network size={16} /> Network Configuration
            </h3>
            {overview?.network && (
              <dl className="kv-list">
                <div>
                  <dt>IPv4 Address</dt>
                  <dd className="mono">{overview.network.ipAddress}</dd>
                </div>
                <div>
                  <dt>Subnet Mask</dt>
                  <dd className="mono">{overview.network.subnetMask}</dd>
                </div>
                <div>
                  <dt>Default Gateway</dt>
                  <dd className="mono">{overview.network.defaultGateway}</dd>
                </div>
                <div>
                  <dt>Primary DNS</dt>
                  <dd className="mono">{overview.network.primaryDns}</dd>
                </div>
                <div>
                  <dt>MAC Address</dt>
                  <dd className="mono">{overview.network.macAddress}</dd>
                </div>
                <div>
                  <dt>MTU</dt>
                  <dd>{overview.network.mtu}</dd>
                </div>
              </dl>
            )}
          </div>

          <div className="panel-card">
            <h3>
              <Shield size={16} /> Users & Maintenance
            </h3>
            <ul className="user-list">
              {overview?.users?.map((u) => (
                <li key={u.id}>
                  <strong>{u.userName}</strong>
                  <span className="ch-badge">{u.userLevel}</span>
                </li>
              ))}
            </ul>
            <button
              type="button"
              className="btn-danger full-width"
              onClick={async () => {
                await dvr.rebootDevice();
                setStatusMsg('Reboot command sent to DVR');
              }}
            >
              <Power size={15} /> Reboot TV-DVR208
            </button>
          </div>
        </div>
      )}
    </div>
  );
};

import React, { useEffect, useState } from 'react';
import { Calendar, Clock, Download, Film, Play, Search } from 'lucide-react';
import { useDvrPort } from '../../application/DvrContext';
import { RecordingDaySpan, RecordingSegment, VideoChannel } from '../../domain/models';
import {
  computeTimelineBars,
  formatBytes,
  formatDuration,
  formatTimeUTC,
} from '../../domain/timeline';
import { ManagedVideo } from './ManagedVideo';

interface ArchivePageProps {
  channels: VideoChannel[];
  defaultDateIso?: string;
}

export const ArchivePage: React.FC<ArchivePageProps> = ({
  channels,
  defaultDateIso = '2026-11-07',
}) => {
  const dvr = useDvrPort();

  const [channelId, setChannelId] = useState<number>(1);
  const [selectedDate, setSelectedDate] = useState<string>(defaultDateIso);
  const [calendarDays, setCalendarDays] = useState<RecordingDaySpan[]>([]);
  const [segments, setSegments] = useState<RecordingSegment[]>([]);
  const [activeSegment, setActiveSegment] = useState<RecordingSegment | null>(null);
  const [playbackRate, setPlaybackRate] = useState<number>(1.0);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const [yearStr, monthStr] = selectedDate.split('-');
  const year = Number(yearStr) || 2026;
  const month = Number(monthStr) || 11;

  useEffect(() => {
    let mounted = true;
    dvr
      .getMonthlyCalendar(channelId, year, month)
      .then((days) => {
        if (mounted) setCalendarDays(days);
      })
      .catch(() => {
        if (mounted) setCalendarDays([]);
      });
    return () => {
      mounted = false;
    };
  }, [dvr, channelId, year, month]);

  const loadDaySegments = async (chId: number, dateIso: string) => {
    setLoading(true);
    setError(null);
    try {
      const startIso = `${dateIso}T00:00:00Z`;
      const endIso = `${dateIso}T23:59:59Z`;
      const segs = await dvr.searchRecordings(chId, startIso, endIso);
      setSegments(segs);
      if (segs.length > 0) {
        setActiveSegment(segs[0]);
      } else {
        setActiveSegment(null);
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadDaySegments(channelId, selectedDate);
  }, [channelId, selectedDate]);

  const timelineBars = computeTimelineBars(segments, selectedDate);
  const activeChannelName =
    channels.find((c) => c.id === channelId)?.name ?? `Channel ${channelId}`;

  const playUrl = activeSegment
    ? dvr.getArchivePlayUrl(channelId, activeSegment.playbackUri)
    : '';

  return (
    <div className="archive-layout">
      {/* Left Sidebar: Channel & Monthly Calendar */}
      <aside className="archive-sidebar">
        <div className="panel-card">
          <h3>
            <Calendar size={16} /> Recording Archive
          </h3>

          <div className="form-row">
            <label htmlFor="archive-channel-select">Camera Channel</label>
            <select
              id="archive-channel-select"
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

          <div className="form-row">
            <label htmlFor="archive-date-input">Date (UTC)</label>
            <div className="input-with-btn">
              <input
                id="archive-date-input"
                type="date"
                value={selectedDate}
                onChange={(e) => setSelectedDate(e.target.value)}
              />
              <button
                type="button"
                className="btn-primary"
                onClick={() => loadDaySegments(channelId, selectedDate)}
              >
                <Search size={14} />
              </button>
            </div>
          </div>

          <div className="calendar-grid-wrapper">
            <span className="toolbar-label">
              Recorded Days in {year}-{String(month).padStart(2, '0')}:
            </span>
            <div className="calendar-days-grid">
              {Array.from({ length: 31 }, (_, i) => i + 1).map((dayNum) => {
                const span = calendarDays.find((d) => d.dayNumber === dayNum);
                const hasRec = Boolean(span?.hasRecords);
                const dayIso = `${year}-${String(month).padStart(2, '0')}-${String(dayNum).padStart(2, '0')}`;
                const isSelected = selectedDate === dayIso;

                return (
                  <button
                    key={dayNum}
                    type="button"
                    className={`cal-day-btn ${hasRec ? 'has-record' : ''} ${isSelected ? 'selected' : ''}`}
                    onClick={() => setSelectedDate(dayIso)}
                    title={hasRec ? `Recordings available (${span?.recordType ?? 'timing'})` : 'No recordings'}
                  >
                    {dayNum}
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      </aside>

      {/* Main Content: Player, 24-Hour Timeline Scrubber & Segment Table */}
      <div className="archive-main">
        {/* Video Player Card */}
        <div className="panel-card archive-player-card">
          <div className="player-header">
            <div>
              <h3>
                <Film size={16} /> Playback — CH {channelId} ({activeChannelName})
              </h3>
              {activeSegment && (
                <span className="muted-text">
                  Playing: {formatTimeUTC(activeSegment.startTime)} to{' '}
                  {formatTimeUTC(activeSegment.endTime)} ({formatBytes(activeSegment.sizeBytes)})
                </span>
              )}
            </div>
            <div className="toolbar-group">
              <span className="toolbar-label">Speed:</span>
              {[0.5, 1.0, 2.0, 4.0].map((rate) => (
                <button
                  key={rate}
                  type="button"
                  className={`btn-chip ${playbackRate === rate ? 'active' : ''}`}
                  onClick={() => setPlaybackRate(rate)}
                >
                  {rate}×
                </button>
              ))}
              {activeSegment && (
                <a
                  href={dvr.getArchiveDownloadUrl(
                    activeSegment.playbackUri,
                    `CH${channelId}_${activeChannelName}_${selectedDate}_${formatTimeUTC(activeSegment.startTime).replace(/:/g, '')}.mp4`
                  )}
                  className="btn-primary"
                  data-testid="active-download-btn"
                >
                  <Download size={14} /> Download MP4
                </a>
              )}
            </div>
          </div>

          <div className="archive-viewport">
            {activeSegment ? (
              <ManagedVideo
                key={activeSegment.playbackUri}
                src={playUrl}
                playbackRate={playbackRate}
                controls
                autoPlay
                playsInline
                className="archive-video"
              />
            ) : (
              <div className="empty-player">
                {loading
                  ? 'Searching DVR recordings...'
                  : 'Select a recorded segment on the 24h timeline or table below to play'}
              </div>
            )}
          </div>

          {/* 24-Hour Interactive Timeline Scrubber */}
          <div className="timeline-container">
            <div className="timeline-header">
              <span>
                <Clock size={13} /> 24-Hour Recording Timeline ({selectedDate})
              </span>
              <span>{segments.length} segments found</span>
            </div>
            <div className="timeline-track" data-testid="timeline-track">
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
                    title={`${bar.label} (${formatBytes(bar.segment.sizeBytes)})`}
                    onClick={() => setActiveSegment(bar.segment)}
                  />
                );
              })}
            </div>
            <div className="timeline-ruler">
              {[0, 3, 6, 9, 12, 15, 18, 21, 24].map((h) => (
                <span key={h}>{String(h).padStart(2, '0')}:00</span>
              ))}
            </div>
          </div>
        </div>

        {/* Segments List & Lossless MP4 Download Table */}
        <div className="panel-card">
          <h3>Archived Segments ({segments.length})</h3>
          {error && <div className="error-banner">{error}</div>}
          <div className="table-scroll">
            <table className="data-table">
              <thead>
                <tr>
                  <th>#</th>
                  <th>Start (UTC)</th>
                  <th>End (UTC)</th>
                  <th>Duration</th>
                  <th>Size</th>
                  <th>File ID</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {segments.map((seg, i) => {
                  const isCurrent = activeSegment?.playbackUri === seg.playbackUri;
                  const dlUrl = dvr.getArchiveDownloadUrl(
                    seg.playbackUri,
                    `CH${channelId}_${selectedDate}_${formatTimeUTC(seg.startTime).replace(/:/g, '')}.mp4`
                  );
                  return (
                    <tr key={seg.playbackUri || i} className={isCurrent ? 'active-row' : ''}>
                      <td>{i + 1}</td>
                      <td>{formatTimeUTC(seg.startTime)}</td>
                      <td>{formatTimeUTC(seg.endTime)}</td>
                      <td>{formatDuration(seg.startTime, seg.endTime)}</td>
                      <td>{formatBytes(seg.sizeBytes)}</td>
                      <td className="mono">{seg.fileName || 'stream'}</td>
                      <td className="actions-cell">
                        <button
                          type="button"
                          className="btn-xs"
                          onClick={() => setActiveSegment(seg)}
                        >
                          <Play size={12} /> Play
                        </button>
                        <a href={dlUrl} className="btn-xs accent">
                          <Download size={12} /> .MP4
                        </a>
                      </td>
                    </tr>
                  );
                })}
                {segments.length === 0 && !loading && (
                  <tr>
                    <td colSpan={7} className="empty-cell">
                      No recordings found for this date.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
};

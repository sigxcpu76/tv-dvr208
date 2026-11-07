import { RecordingSegment } from './models';

export interface TimelineBar {
  segment: RecordingSegment;
  leftPercent: number;
  widthPercent: number;
  label: string;
}

const SECONDS_PER_DAY = 86400;

export function computeTimelineBars(
  segments: RecordingSegment[],
  dayIsoDate: string
): TimelineBar[] {
  const dayStartMs = new Date(`${dayIsoDate}T00:00:00Z`).getTime();
  const dayEndMs = dayStartMs + SECONDS_PER_DAY * 1000;

  return segments
    .map((seg) => {
      const st = Math.max(new Date(seg.startTime).getTime(), dayStartMs);
      const et = Math.min(new Date(seg.endTime).getTime(), dayEndMs);
      if (et <= st) return null;

      const leftSec = (st - dayStartMs) / 1000;
      const durSec = (et - st) / 1000;
      const leftPercent = Math.max(0, Math.min(100, (leftSec / SECONDS_PER_DAY) * 100));
      const widthPercent = Math.max(0.25, Math.min(100 - leftPercent, (durSec / SECONDS_PER_DAY) * 100));

      return {
        segment: seg,
        leftPercent,
        widthPercent,
        label: `${formatTimeUTC(seg.startTime)} – ${formatTimeUTC(seg.endTime)}`,
      };
    })
    .filter((b): b is TimelineBar => b !== null);
}

export function formatTimeUTC(isoStr: string): string {
  const d = new Date(isoStr);
  if (Number.isNaN(d.getTime())) return isoStr;
  return d.toISOString().slice(11, 19);
}

export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let val = bytes;
  let idx = 0;
  while (val >= 1024 && idx < units.length - 1) {
    val /= 1024;
    idx++;
  }
  return `${val.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
}

export function formatDuration(startIso: string, endIso: string): string {
  const diffSec = Math.max(
    0,
    Math.round((new Date(endIso).getTime() - new Date(startIso).getTime()) / 1000)
  );
  const mins = Math.floor(diffSec / 60);
  const secs = diffSec % 60;
  if (mins >= 60) {
    const hrs = Math.floor(mins / 60);
    const remMins = mins % 60;
    return `${hrs}h ${remMins}m ${secs}s`;
  }
  return `${mins}m ${secs}s`;
}

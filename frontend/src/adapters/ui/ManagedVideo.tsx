import React, { useEffect, useRef } from 'react';

export interface ManagedVideoProps extends React.VideoHTMLAttributes<HTMLVideoElement> {
  src: string;
  playbackRate?: number;
}

/**
 * Converts a gateway progressive fMP4 URL to its Apple HLS (.m3u8) equivalent:
 * - /api/channels/{id}/live.mp4?stream={s} -> /api/channels/{id}/hls/{s}/index.m3u8
 * - /api/archive/channels/{id}/play.mp4?uri=... -> /api/archive/channels/{id}/hls/index.m3u8?uri=...
 */
export function toHlsUrl(mp4Url: string): string | null {
  if (!mp4Url) return null;
  const liveMatch = mp4Url.match(/^(.*\/api\/channels\/(\d+))\/live\.mp4(?:\?(.*))?$/);
  if (liveMatch) {
    const prefix = liveMatch[1];
    const query = liveMatch[3] || '';
    const params = new URLSearchParams(query);
    const stream = params.get('stream') || '2';
    params.delete('stream');
    const qs = params.toString();
    return `${prefix}/hls/${stream}/index.m3u8${qs ? `?${qs}` : ''}`;
  }

  const archiveMatch = mp4Url.match(/^(.*\/api\/archive\/channels\/(\d+))\/play\.mp4(?:\?(.*))?$/);
  if (archiveMatch) {
    const prefix = archiveMatch[1];
    const query = archiveMatch[3] || '';
    return `${prefix}/hls/index.m3u8${query ? `?${query}` : ''}`;
  }

  return null;
}

/**
 * Detects iOS / iPadOS / Safari environments where chunked progressive fMP4
 * without HTTP Range / Content-Length is rejected by AVPlayer, and native HLS (.m3u8) is required.
 */
export function prefersNativeHls(el?: HTMLVideoElement | null): boolean {
  if (typeof navigator === 'undefined') return false;
  const ua = navigator.userAgent || '';
  const isIOS =
    /iPad|iPhone|iPod/i.test(ua) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
  const isSafari = /^((?!chrome|android|crios|fxios).)*safari/i.test(ua);
  if (isIOS || isSafari) {
    return true;
  }
  if (el && typeof el.canPlayType === 'function') {
    const canHls = el.canPlayType('application/vnd.apple.mpegurl');
    if (canHls === 'probably' || canHls === 'maybe') {
      return true;
    }
  }
  return false;
}

function releaseMediaElement(el: HTMLVideoElement | null): void {
  if (!el) return;
  try {
    el.pause();
    el.removeAttribute('src');
    el.load();
  } catch {
    // Ignore in test/jsdom environments where HTMLMediaElement.load may not be implemented
  }
}

/**
 * ManagedVideo wraps <video> to:
 * 1. Automatically serve Apple HLS (.m3u8) on iPhone / iPad / Safari where chunked fMP4 is unsupported,
 *    with automatic fallback to HLS if progressive fMP4 errors on any mobile browser.
 * 2. Guarantee that active HTTP streams are immediately aborted when the component unmounts,
 *    when `src` changes, or when the page unloads/refreshes.
 */
export const ManagedVideo: React.FC<ManagedVideoProps> = ({
  src,
  playbackRate = 1.0,
  autoPlay,
  ...rest
}) => {
  const videoRef = useRef<HTMLVideoElement | null>(null);

  useEffect(() => {
    const el = videoRef.current;
    if (!el || !src) return;

    const hlsUrl = toHlsUrl(src);
    const useHlsInitially = Boolean(hlsUrl && prefersNativeHls(el));
    let triedHlsFallback = useHlsInitially;

    const startPlayback = (targetUrl: string) => {
      el.src = targetUrl;
      try {
        el.playbackRate = playbackRate;
        if (autoPlay) {
          const playPromise = el.play();
          if (playPromise && typeof playPromise.catch === 'function') {
            playPromise.catch(() => {});
          }
        }
      } catch {
        // Ignore in non-media test environments
      }
    };

    startPlayback(useHlsInitially && hlsUrl ? hlsUrl : src);

    const handleError = () => {
      if (!triedHlsFallback && hlsUrl) {
        triedHlsFallback = true;
        startPlayback(hlsUrl);
      }
    };

    const handleUnload = () => releaseMediaElement(el);
    el.addEventListener('error', handleError);
    window.addEventListener('beforeunload', handleUnload);
    window.addEventListener('pagehide', handleUnload);

    return () => {
      el.removeEventListener('error', handleError);
      window.removeEventListener('beforeunload', handleUnload);
      window.removeEventListener('pagehide', handleUnload);
      releaseMediaElement(el);
    };
  }, [src, autoPlay]);

  useEffect(() => {
    if (videoRef.current) {
      try {
        videoRef.current.playbackRate = playbackRate;
      } catch {
        // Ignore
      }
    }
  }, [playbackRate, src]);

  return <video ref={videoRef} autoPlay={autoPlay} {...rest} />;
};

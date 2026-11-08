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
 * Converts a gateway live fMP4 URL (/api/channels/{id}/live.mp4?stream={s}) to its binary WebSocket URL
 * (ws[s]://host/api/channels/{id}/live.ws?stream={s}) for MSE streaming without consuming HTTP/1.1 sockets.
 */
export function toLiveWsUrl(mp4Url: string): string | null {
  if (!mp4Url) return null;
  const liveMatch = mp4Url.match(/^(.*\/api\/channels\/(\d+))\/live\.mp4(?:\?(.*))?$/);
  if (!liveMatch) return null;

  const prefix = liveMatch[1];
  const query = liveMatch[3] ? `?${liveMatch[3]}` : '';
  const wsPath = `${prefix}/live.ws${query}`;

  if (wsPath.startsWith('http://')) {
    return `ws://${wsPath.slice('http://'.length)}`;
  }
  if (wsPath.startsWith('https://')) {
    return `wss://${wsPath.slice('https://'.length)}`;
  }
  if (typeof window !== 'undefined' && window.location?.host) {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const normalized = wsPath.startsWith('/') ? wsPath : `/${wsPath}`;
    return `${proto}//${window.location.host}${normalized}`;
  }
  return null;
}

/**
 * Detects iOS / iPadOS / Safari environments where chunked progressive fMP4
 * without HTTP Range / Content-Length is rejected by AVPlayer, and native HLS (.m3u8) is required.
 * Note: Desktop Chrome/Chromium must NOT use native HLS URLs because `canPlayType('application/vnd.apple.mpegurl')`
 * returns `'maybe'` on Chromium builds whose FFmpegDemuxer cannot actually open `.m3u8` playlists.
 */
export function prefersNativeHls(_el?: HTMLVideoElement | null): boolean {
  if (typeof navigator === 'undefined') return false;
  const ua = navigator.userAgent || '';
  const isIOS =
    /iPad|iPhone|iPod/i.test(ua) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
  const isSafari = /^((?!chrome|android|crios|fxios).)*safari/i.test(ua);
  return isIOS || isSafari;
}

function supportsMse(): boolean {
  if (typeof window === 'undefined') return false;
  const ms = window.MediaSource;
  if (!ms || typeof ms.isTypeSupported !== 'function' || typeof window.WebSocket === 'undefined') {
    return false;
  }
  try {
    return ms.isTypeSupported('video/mp4; codecs="avc1.64001f"');
  } catch {
    return false;
  }
}

/**
 * Scans an fMP4 initialization segment (ftyp + moov) for the ISO BMFF `avcC` box and extracts
 * `AVCProfileIndication`, `profile_compatibility`, and `AVCLevelIndication` to build the exact
 * RFC 6381 codec string (e.g. `video/mp4; codecs="avc1.64001f"` or `avc1.64002a`).
 */
export function detectAvcMime(buf: Uint8Array): string | null {
  for (let i = 0; i + 7 < buf.length; i += 1) {
    if (
      buf[i] === 0x61 && // 'a'
      buf[i + 1] === 0x76 && // 'v'
      buf[i + 2] === 0x63 && // 'c'
      buf[i + 3] === 0x43 // 'C'
    ) {
      const profile = buf[i + 5].toString(16).padStart(2, '0');
      const compat = buf[i + 6].toString(16).padStart(2, '0');
      const level = buf[i + 7].toString(16).padStart(2, '0');
      return `video/mp4; codecs="avc1.${profile}${compat}${level}"`;
    }
  }
  return null;
}

function concatBytes(a: Uint8Array, b: Uint8Array): Uint8Array {
  if (a.length === 0) return b;
  if (b.length === 0) return a;
  const out = new Uint8Array(a.length + b.length);
  out.set(a, 0);
  out.set(b, a.length);
  return out;
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
 * 1. Automatically serve Apple HLS (.m3u8) on iPhone / iPad / Safari where chunked fMP4 is unsupported.
 * 2. Stream low-latency fMP4 over WebSocket + MediaSource (MSE) for live channels on Desktop Chrome /
 *    Edge / Firefox / Android so all 8 camera channels stream simultaneously without occupying the
 *    browser's 6-connection HTTP/1.1 pool, with automatic fallback to direct progressive fMP4.
 * 3. Guarantee that active WebSocket / HTTP streams are immediately aborted when the component unmounts,
 *    when `src` changes, or when the page unloads/refreshes.
 */
export const ManagedVideo: React.FC<ManagedVideoProps> = ({
  src,
  playbackRate = 1.0,
  autoPlay,
  muted,
  ...rest
}) => {
  const videoRef = useRef<HTMLVideoElement | null>(null);

  useEffect(() => {
    const el = videoRef.current;
    if (!el || !src) return;

    const hlsUrl = toHlsUrl(src);
    const useHls = Boolean(hlsUrl && prefersNativeHls(el));
    const wsUrl = !useHls ? toLiveWsUrl(src) : null;
    const useMse = Boolean(wsUrl && supportsMse());

    let retryCount = 0;
    let retryTimer: ReturnType<typeof setTimeout> | null = null;
    let activeWs: WebSocket | null = null;
    let activeObjectUrl: string | null = null;
    let unmounted = false;

    const cleanupMse = () => {
      if (activeWs) {
        activeWs.onopen = null;
        activeWs.onmessage = null;
        activeWs.onerror = null;
        activeWs.onclose = null;
        try {
          activeWs.close();
        } catch {
          // Ignore
        }
        activeWs = null;
      }
      if (activeObjectUrl) {
        try {
          URL.revokeObjectURL(activeObjectUrl);
        } catch {
          // Ignore
        }
        activeObjectUrl = null;
      }
    };

    const applyMuteAndRate = () => {
      try {
        if (muted !== undefined) {
          el.defaultMuted = Boolean(muted);
          el.muted = Boolean(muted);
        }
        el.playbackRate = playbackRate;
      } catch {
        // Ignore in non-media test environments
      }
    };

    const tryAutoPlay = () => {
      if (unmounted || !autoPlay) return;
      try {
        const playPromise = el.play();
        if (playPromise && typeof playPromise.catch === 'function') {
          playPromise.catch(() => {});
        }
      } catch {
        // Ignore in non-media test environments
      }
    };

    const startDirectPlayback = (targetUrl: string) => {
      if (unmounted) return;
      cleanupMse();
      applyMuteAndRate();
      el.src = targetUrl;
      tryAutoPlay();
    };

    const primaryUrl = useHls && hlsUrl ? hlsUrl : src;

    const scheduleRetry = (fallbackDirect = false) => {
      if (unmounted || retryCount >= 3) return;
      retryCount += 1;
      if (retryTimer) clearTimeout(retryTimer);
      retryTimer = setTimeout(() => {
        if (unmounted) return;
        if (useMse && wsUrl && !fallbackDirect) {
          startMsePlayback(wsUrl);
        } else {
          const sep = primaryUrl.includes('?') ? '&' : '?';
          startDirectPlayback(`${primaryUrl}${sep}_retry=${retryCount}_${Date.now()}`);
        }
      }, 1200);
    };

    const startMsePlayback = (targetWsUrl: string) => {
      if (unmounted) return;
      cleanupMse();
      applyMuteAndRate();

      const mediaSource = new MediaSource();
      activeObjectUrl = URL.createObjectURL(mediaSource);
      el.src = activeObjectUrl;

      mediaSource.addEventListener(
        'sourceopen',
        () => {
          if (unmounted || mediaSource.readyState !== 'open') return;

          let sourceBuffer: SourceBuffer | null = null;
          let initBytes: Uint8Array = new Uint8Array(0);
          let pendingBytes: Uint8Array = new Uint8Array(0);
          let receivedMedia = false;

          const flushPending = () => {
            if (
              unmounted ||
              !sourceBuffer ||
              sourceBuffer.updating ||
              pendingBytes.length === 0 ||
              mediaSource.readyState !== 'open'
            ) {
              return;
            }
            const chunkToAppend = pendingBytes;
            pendingBytes = new Uint8Array(0);
            try {
              sourceBuffer.appendBuffer(chunkToAppend as unknown as BufferSource);
            } catch {
              // If SourceBuffer quota or decode fails, reconnect cleanly
              scheduleRetry(true);
            }
          };

          const ws = new WebSocket(targetWsUrl);
          ws.binaryType = 'arraybuffer';
          activeWs = ws;

          ws.onmessage = (ev: MessageEvent) => {
            if (unmounted || !(ev.data instanceof ArrayBuffer)) return;
            const chunk = new Uint8Array(ev.data);
            if (chunk.length === 0) return;

            if (!sourceBuffer) {
              initBytes = concatBytes(initBytes, chunk);
              const detectedMime =
                detectAvcMime(initBytes) ||
                (initBytes.length >= 4096 ? 'video/mp4; codecs="avc1.64002a"' : null);
              if (!detectedMime) return;

              const finalMime = MediaSource.isTypeSupported(detectedMime)
                ? detectedMime
                : 'video/mp4; codecs="avc1.64001f"';

              try {
                sourceBuffer = mediaSource.addSourceBuffer(finalMime);
                sourceBuffer.mode = 'segments';
              } catch {
                scheduleRetry(true);
                return;
              }

              sourceBuffer.addEventListener('updateend', () => {
                if (unmounted || !sourceBuffer) return;
                if (!receivedMedia && el.readyState >= 2) {
                  receivedMedia = true;
                  retryCount = 0;
                  tryAutoPlay();
                }
                try {
                  if (!sourceBuffer.updating && el.buffered.length > 0) {
                    const start = el.buffered.start(0);
                    const end = el.buffered.end(el.buffered.length - 1);
                    if (autoPlay && el.paused && el.readyState >= 2) {
                      tryAutoPlay();
                    }
                    // Keep live latency tight (< 3s behind live edge)
                    if (end - el.currentTime > 3.0) {
                      el.currentTime = Math.max(start, end - 0.25);
                    }
                    // Evict older buffered segments so memory stays bounded
                    if (end - start > 20.0 && !sourceBuffer.updating) {
                      sourceBuffer.remove(start, end - 10.0);
                      return;
                    }
                  }
                } catch {
                  // Ignore buffered range errors during transitions
                }
                flushPending();
              });

              pendingBytes = concatBytes(pendingBytes, initBytes);
              initBytes = new Uint8Array(0);
              flushPending();
              return;
            }

            pendingBytes = concatBytes(pendingBytes, chunk);
            flushPending();
          };

          ws.onerror = () => {
            if (!unmounted) scheduleRetry(!receivedMedia);
          };

          ws.onclose = () => {
            if (!unmounted) scheduleRetry(!receivedMedia);
          };
        },
        { once: true }
      );
    };

    if (useMse && wsUrl) {
      startMsePlayback(wsUrl);
    } else {
      startDirectPlayback(primaryUrl);
    }

    const handleLoadedData = () => {
      retryCount = 0;
      tryAutoPlay();
    };

    const handleError = () => {
      scheduleRetry(false);
    };

    const handleUnload = () => {
      cleanupMse();
      releaseMediaElement(el);
    };

    el.addEventListener('loadeddata', handleLoadedData);
    el.addEventListener('error', handleError);
    window.addEventListener('beforeunload', handleUnload);
    window.addEventListener('pagehide', handleUnload);

    return () => {
      unmounted = true;
      if (retryTimer) clearTimeout(retryTimer);
      el.removeEventListener('loadeddata', handleLoadedData);
      el.removeEventListener('error', handleError);
      window.removeEventListener('beforeunload', handleUnload);
      window.removeEventListener('pagehide', handleUnload);
      cleanupMse();
      releaseMediaElement(el);
    };
  }, [src, autoPlay, muted]);

  useEffect(() => {
    if (videoRef.current) {
      try {
        videoRef.current.playbackRate = playbackRate;
      } catch {
        // Ignore
      }
    }
  }, [playbackRate, src]);

  return <video ref={videoRef} autoPlay={autoPlay} muted={muted} {...rest} />;
};

import React, { useEffect, useRef } from 'react';

export interface ManagedVideoProps extends React.VideoHTMLAttributes<HTMLVideoElement> {
  src: string;
  playbackRate?: number;
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
 * ManagedVideo wraps <video> to guarantee that active HTTP fMP4 streams are immediately
 * aborted when the component unmounts, when `src` changes, or when the page unloads/refreshes.
 * Without explicit `pause() + removeAttribute('src') + load()`, browsers keep infinite
 * progressive MP4 HTTP/1.1 connections open in the background and exhaust the 6-socket per-origin pool.
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

    el.src = src;
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

    const handleUnload = () => releaseMediaElement(el);
    window.addEventListener('beforeunload', handleUnload);
    window.addEventListener('pagehide', handleUnload);

    return () => {
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

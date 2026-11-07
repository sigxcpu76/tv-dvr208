import React, { useEffect, useState } from 'react';
import { Film, Radio, Settings, ShieldCheck } from 'lucide-react';
import { useDvrPort } from './application/DvrContext';
import { SystemOverview, VideoChannel } from './domain/models';
import { ArchivePage } from './adapters/ui/ArchivePage';
import { ConfigPage } from './adapters/ui/ConfigPage';
import { LiveViewPage } from './adapters/ui/LiveViewPage';

type MainSection = 'live' | 'archive' | 'config';

export const App: React.FC = () => {
  const dvr = useDvrPort();

  const [section, setSection] = useState<MainSection>('live');
  const [channels, setChannels] = useState<VideoChannel[]>([]);
  const [overview, setOverview] = useState<SystemOverview | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    Promise.all([dvr.listChannels(), dvr.getSystemOverview()])
      .then(([chs, ov]) => {
        if (!mounted) return;
        setChannels(chs);
        setOverview(ov);
      })
      .catch((err) => {
        if (!mounted) return;
        setError((err as Error).message);
      });
    return () => {
      mounted = false;
    };
  }, [dvr]);

  return (
    <div className="app-shell">
      <header className="top-navbar">
        <div className="brand">
          <ShieldCheck size={22} className="brand-icon" />
          <div>
            <h1>{overview?.deviceInfo?.deviceName ?? 'TV-DVR208'}</h1>
            <span className="subtitle">
              Modern HTML5 Console • {overview?.network?.ipAddress ?? '192.168.1.64'} • FW{' '}
              {overview?.deviceInfo?.firmwareVersion ?? 'V3.4.7'}
            </span>
          </div>
        </div>

        <nav className="main-nav" aria-label="Main Navigation">
          <button
            type="button"
            className={`nav-tab ${section === 'live' ? 'active' : ''}`}
            onClick={() => setSection('live')}
          >
            <Radio size={16} /> Live View ({channels.length || 8} Ch)
          </button>
          <button
            type="button"
            className={`nav-tab ${section === 'archive' ? 'active' : ''}`}
            onClick={() => setSection('archive')}
          >
            <Film size={16} /> Archive & Playback
          </button>
          <button
            type="button"
            className={`nav-tab ${section === 'config' ? 'active' : ''}`}
            onClick={() => setSection('config')}
          >
            <Settings size={16} /> DVR Configuration
          </button>
        </nav>

        <div className="navbar-status">
          <span className="online-dot" />
          <span>{overview?.systemTime?.localTime?.slice(0, 19).replace('T', ' ') ?? 'Online'}</span>
        </div>
      </header>

      {error && (
        <div className="error-banner" role="alert">
          Gateway connection error: {error}
        </div>
      )}

      <main className="main-workspace">
        {section === 'live' && <LiveViewPage channels={channels} />}
        {section === 'archive' && <ArchivePage channels={channels} />}
        {section === 'config' && <ConfigPage channels={channels} overview={overview} />}
      </main>
    </div>
  );
};

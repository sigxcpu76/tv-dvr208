import React, { createContext, useContext } from 'react';
import { DvrApiPort } from '../ports/DvrApiPort';

const DvrContext = createContext<DvrApiPort | null>(null);

export const DvrProvider: React.FC<{
  adapter: DvrApiPort;
  children: React.ReactNode;
}> = ({ adapter, children }) => {
  return <DvrContext.Provider value={adapter}>{children}</DvrContext.Provider>;
};

export function useDvrPort(): DvrApiPort {
  const ctx = useContext(DvrContext);
  if (!ctx) {
    throw new Error('useDvrPort must be used within a DvrProvider');
  }
  return ctx;
}

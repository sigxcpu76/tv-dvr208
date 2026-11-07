import '@testing-library/jest-dom/vitest';

// Stub HTMLMediaElement play/pause/load in jsdom
Object.defineProperty(HTMLMediaElement.prototype, 'play', {
  configurable: true,
  value: () => Promise.resolve(),
});
Object.defineProperty(HTMLMediaElement.prototype, 'pause', {
  configurable: true,
  value: () => undefined,
});
Object.defineProperty(HTMLMediaElement.prototype, 'load', {
  configurable: true,
  value: () => undefined,
});

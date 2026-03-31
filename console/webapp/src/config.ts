const apiPort = import.meta.env.VITE_CONSOLE_API_PORT?.trim() || "8090";

export const consoleApiUrl = import.meta.env.VITE_CONSOLE_API_URL?.trim()
  || `${window.location.protocol}//${window.location.hostname}:${apiPort}/console/api/v1`;

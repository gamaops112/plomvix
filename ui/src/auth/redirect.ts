export function safeNextPath(raw: string | null): string {
  if (!raw) return '/app/explore';

  try {
    const decoded = decodeURIComponent(raw);
    if (decoded.startsWith('/app/') || decoded === '/app' || decoded === '/dev/design') {
      return decoded;
    }
    return '/app/explore';
  } catch {
    return '/app/explore';
  }
}

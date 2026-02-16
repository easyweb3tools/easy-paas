export function isSafePathSegment(v: string): boolean {
  return /^[a-zA-Z0-9_-]+$/.test(v.trim());
}

export function toSafePathSegment(v: string): string {
  const trimmed = v.trim();
  if (!isSafePathSegment(trimmed)) {
    throw new Error("Invalid path parameter");
  }
  return encodeURIComponent(trimmed);
}


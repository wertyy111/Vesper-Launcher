import { useEffect, useState } from 'react';

export function AvatarImage({
  url,
  placeholder,
  alt,
  className,
  style,
}: {
  url?: unknown;
  placeholder?: unknown;
  alt: string;
  className?: string;
  style?: React.CSSProperties;
}) {
  const cleanAlt = typeof alt === 'string' ? alt.trim() : '';

  const isImageStr = (val: unknown): val is string => {
    if (typeof val !== 'string') return false;
    const s = val.trim();
    if (!s) return false;
    return (
      s.startsWith('http://') ||
      s.startsWith('https://') ||
      s.startsWith('data:image/') ||
      s.startsWith('/assets/') ||
      s.startsWith('/launcher-file/') ||
      /\.(png|jpg|jpeg|webp|gif|svg)$/i.test(s)
    );
  };

  const rawUrl = isImageStr(url) ? (url as string).trim() : null;
  const placeholderIsImage = isImageStr(placeholder);
  const placeholderImg = placeholderIsImage ? (placeholder as string).trim() : null;

  // Fallback to player skin head for valid Minecraft usernames (3-16 alphanumeric or underscore)
  const isMcUsername =
    /^[A-Za-z0-9_]{3,16}$/.test(cleanAlt) &&
    !['profile', 'avatar', 'unknown', 'none'].includes(cleanAlt.toLowerCase());
  const headFallbackUrl = isMcUsername
    ? `https://minotar.net/avatar/${encodeURIComponent(cleanAlt)}/64.png`
    : null;

  const [currentSrc, setCurrentSrc] = useState<string | null>(null);
  const [triedHead, setTriedHead] = useState(false);
  const [triedPlaceholder, setTriedPlaceholder] = useState(false);
  const [hasError, setHasError] = useState(false);

  useEffect(() => {
    setTriedHead(false);
    setTriedPlaceholder(false);
    setHasError(false);

    if (rawUrl) {
      setCurrentSrc(rawUrl);
    } else if (headFallbackUrl) {
      setCurrentSrc(headFallbackUrl);
      setTriedHead(true);
    } else if (placeholderImg) {
      setCurrentSrc(placeholderImg);
      setTriedPlaceholder(true);
    } else {
      setCurrentSrc(null);
    }
  }, [rawUrl, headFallbackUrl, placeholderImg]);

  const handleError = () => {
    if (!triedHead && headFallbackUrl && currentSrc !== headFallbackUrl) {
      setTriedHead(true);
      setCurrentSrc(headFallbackUrl);
    } else if (!triedPlaceholder && placeholderImg && currentSrc !== placeholderImg) {
      setTriedPlaceholder(true);
      setCurrentSrc(placeholderImg);
    } else {
      setHasError(true);
      setCurrentSrc(null);
    }
  };

  // Safe 2-letter uppercase initials text
  const initials = (!placeholderIsImage && typeof placeholder === 'string' && placeholder.trim().length > 0)
    ? placeholder.trim().slice(0, 2).toUpperCase()
    : (cleanAlt ? cleanAlt.slice(0, 2).toUpperCase() : 'AV');

  if (hasError || !currentSrc) {
    return (
      <span className={className} style={style}>
        {initials}
      </span>
    );
  }

  return (
    <img
      className={className}
      src={currentSrc}
      alt={cleanAlt}
      loading="lazy"
      decoding="async"
      style={style}
      onError={handleError}
    />
  );
}


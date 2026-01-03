import { useState, useEffect } from 'react';

interface UseLiveDurationOptions {
  /** ISO date string for when the entity started */
  startedAt?: string | null;
  /** ISO date string for when the entity ended (null/undefined if still running) */
  endedAt?: string | null;
  /** Whether the entity is in a live state (RUNNING/PENDING) */
  isLive: boolean;
  /** Pre-computed duration in ms (used when available and not live) */
  durationMs?: number | null;
  /** Update interval in ms (default: 500ms) */
  interval?: number;
}

/**
 * Hook that returns a live-updating duration for in-progress runs/tests.
 *
 * - If endedAt exists or durationMs is provided and not live: returns static duration
 * - If isLive and startedAt exists: returns live-updating duration (recalculated every interval)
 * - If no startedAt: returns undefined
 */
export function useLiveDurationMs({
  startedAt,
  endedAt,
  isLive,
  durationMs,
  interval = 500,
}: UseLiveDurationOptions): number | undefined {
  const [liveDuration, setLiveDuration] = useState<number | undefined>(() => {
    // Initial calculation
    if (durationMs != null && !isLive) {
      return durationMs;
    }
    if (endedAt && startedAt) {
      return new Date(endedAt).getTime() - new Date(startedAt).getTime();
    }
    if (isLive && startedAt) {
      return Date.now() - new Date(startedAt).getTime();
    }
    return undefined;
  });

  useEffect(() => {
    // If not live, use static duration
    if (!isLive) {
      if (durationMs != null) {
        setLiveDuration(durationMs);
      } else if (endedAt && startedAt) {
        setLiveDuration(new Date(endedAt).getTime() - new Date(startedAt).getTime());
      }
      return;
    }

    // If live but no startedAt, can't calculate
    if (!startedAt) {
      setLiveDuration(undefined);
      return;
    }

    // Live mode: update duration at regular intervals
    const startTime = new Date(startedAt).getTime();

    const updateDuration = () => {
      setLiveDuration(Date.now() - startTime);
    };

    // Immediate update
    updateDuration();

    // Set up interval
    const intervalId = setInterval(updateDuration, interval);

    return () => {
      clearInterval(intervalId);
    };
  }, [startedAt, endedAt, isLive, durationMs, interval]);

  return liveDuration;
}

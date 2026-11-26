import amberCover from './assets/covers/amber.jpg'
import coding2Cover from './assets/covers/coding2.jpg'
import emeraldCover from './assets/covers/emerald.jpg'
import oceanCover from './assets/covers/ocean.jpg'
import podcastCover from './assets/covers/podcast.jpg'
import roseCover from './assets/covers/rose.jpg'
import violetCover from './assets/covers/violet.jpg'
import zincCover from './assets/covers/zinc.jpg'

const themeCovers: Record<string, string> = {
  ocean: oceanCover,
  violet: violetCover,
  amber: amberCover,
  emerald: emeraldCover,
  rose: roseCover,
  zinc: zincCover,
}

const categoryCovers: Record<string, string> = {
  coding: coding2Cover,
  gaming: violetCover,
  music: amberCover,
  outdoor: emeraldCover,
  chat: roseCover,
}

const fallbackPool = [coding2Cover, podcastCover, amberCover, emeraldCover, roseCover, violetCover, zincCover, oceanCover]

function hashSlug(slug: string) {
  let hash = 0
  for (let i = 0; i < slug.length; i++) hash = (hash * 31 + slug.charCodeAt(i)) >>> 0
  return hash
}

/**
 * Selects a cohesive 16:9 editorial cover image for a room while preserving
 * explicit non-default theme choices and avoiding repetitive thumbnails when
 * multiple rooms share the default theme.
 */
export function roomCover(theme: string, category: string, slug: string): string {
  if (theme && theme !== 'zinc' && themeCovers[theme]) return themeCovers[theme]
  if (category && category !== 'chat' && categoryCovers[category]) return categoryCovers[category]
  return fallbackPool[hashSlug(slug || 'maku') % fallbackPool.length]
}

export function roomInitial(title: string, slug: string): string {
  const source = (title || slug || 'M').trim()
  return Array.from(source)[0]?.toUpperCase() || 'M'
}

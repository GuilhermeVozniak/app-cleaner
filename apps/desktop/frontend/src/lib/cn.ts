import { clsx, type ClassValue } from 'clsx'
import { extendTailwindMerge } from 'tailwind-merge'

// tailwind-merge only knows Tailwind's stock scale, so it would read our
// `text-body` / `text-card` type-scale utilities as text *colours* and drop
// whichever of `text-ink` / `text-body` comes first. Teach it the scale.
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      'font-size': [
        { text: ['display', 'headline', 'stat', 'title', 'card', 'body', 'caption', 'micro'] },
      ],
    },
  },
})

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

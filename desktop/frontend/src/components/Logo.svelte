<script lang="ts">
  // Pare's mark: a "P" on a 5×7 dot matrix, unlit dots faintly visible, and a red full
  // stop in the corner. The same grid is used for the app icon and the README banner.
  let { size = 22 }: { size?: number } = $props()

  const glyph = ['XXXX.', 'X...X', 'X...X', 'XXXX.', 'X....', 'X....', 'X...R']
  const dots = glyph.flatMap((row, r) => [...row].map((c, col) => ({ x: 7 + col * 4.5, y: 2.5 + r * 4.5, c })))
</script>

<svg viewBox="0 0 32 32" width={size} height={size} aria-hidden="true">
  {#each dots as d}
    <circle cx={d.x} cy={d.y} r="1.75" fill={d.c === 'R' ? 'var(--signal)' : 'currentColor'} opacity={d.c === '.' ? 0.16 : 1} />
  {/each}
</svg>

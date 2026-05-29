import { onMount } from 'solid-js'

export default function DotBackground() {
  onMount(() => {
    import('./AWavesElement')
  })

  return (
    <>
      <style>{`
        a-waves {
          --x: -0.5rem;
          --y: 50%;
          position: absolute;
          width: 100%;
          height: 100%;
          overflow: hidden;
          margin: 0;
          padding: 0;
        }
        a-waves::before {
          position: absolute;
          width: 0.5rem;
          height: 0.5rem;
          background: #ffffff;
          border-radius: 50%;
          transform: translate3d(calc(var(--x) - 50%), calc(var(--y) - 50%), 0);
          will-change: transform;
          content: "";
        }
        a-waves svg {
          display: block;
          width: 100%;
          height: 100%;
        }
        a-waves svg .a__dot {
          fill: #160000;
        }
      `}</style>
      <a-waves>
        <svg class="js-svg" />
      </a-waves>
    </>
  )
}

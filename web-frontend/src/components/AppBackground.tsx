// Immersive "cosmic dark terminal" backdrop: three blurred colour glows, a
// noise overlay, and drifting particles. All motion is CSS-driven and disabled
// under prefers-reduced-motion (see index.css).

const PARTICLES = Array.from({ length: 20 }, (_, i) => i)

export function AppBackground() {
  return (
    <div className="pointer-events-none fixed inset-0 -z-10 overflow-hidden bg-[#020617]">
      {/* Coral glow — top left */}
      <div
        className="qs-glow-a absolute -left-32 -top-32 h-[40rem] w-[40rem] rounded-full opacity-40"
        style={{ background: '#ff8c6b', filter: 'blur(120px)', mixBlendMode: 'screen' }}
      />
      {/* Sky-blue glow — bottom right */}
      <div
        className="qs-glow-b absolute -bottom-40 -right-32 h-[44rem] w-[44rem] rounded-full opacity-40"
        style={{ background: '#0ea5e9', filter: 'blur(140px)', mixBlendMode: 'screen' }}
      />
      {/* Teal glow — centre */}
      <div
        className="qs-glow-c absolute left-1/2 top-1/2 h-[36rem] w-[36rem] rounded-full opacity-30"
        style={{ background: '#2dd4bf', filter: 'blur(100px)', mixBlendMode: 'screen' }}
      />

      {/* Rising particles */}
      {PARTICLES.map((i) => (
        <span
          key={i}
          className="qs-particle absolute bottom-0 h-1 w-1 rounded-full bg-white/60"
          style={{
            left: `${(i * 53) % 100}%`,
            animation: `qs-rise ${12 + (i % 7) * 2}s linear ${(i % 5) * 1.5}s infinite`,
          }}
        />
      ))}

      {/* Fractal-noise grain overlay */}
      <svg className="absolute inset-0 h-full w-full opacity-[0.15]" style={{ mixBlendMode: 'color-dodge' }}>
        <filter id="qs-noise">
          <feTurbulence type="fractalNoise" baseFrequency="0.8" numOctaves="2" stitchTiles="stitch" />
        </filter>
        <rect width="100%" height="100%" filter="url(#qs-noise)" />
      </svg>
    </div>
  )
}

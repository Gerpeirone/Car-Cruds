import { useId } from "react";

/** Ilustración neutra propia: no representa una fotografía ni un modelo del catálogo. */
export default function CarArt({
  tone = "silver",
  hero = false,
}: {
  tone?: string;
  hero?: boolean;
}) {
  const id = useId().replace(/:/g, "");
  const colors: Record<string, string[]> = {
    silver: ["#f6f8f6", "#b7c2bc", "#708279"],
    green: ["#b4c7aa", "#69826b", "#344e41"],
    sand: ["#ebe4d5", "#b0a28d", "#756d60"],
  };
  const c = colors[tone] || colors.silver;
  return (
    <svg
      viewBox="0 0 650 330"
      className={`car-art ${hero ? "car-art-hero" : ""}`}
      aria-hidden="true"
    >
      <defs>
        <linearGradient id={`${id}-body`} x1="0" y1="0" x2=".2" y2="1">
          <stop stopColor={c[0]} />
          <stop offset=".58" stopColor={c[1]} />
          <stop offset="1" stopColor={c[2]} />
        </linearGradient>
        <linearGradient id={`${id}-glass`} x1="0" y1="0" x2="1" y2="1">
          <stop stopColor="#8daba7" />
          <stop offset="1" stopColor="#172f2c" />
        </linearGradient>
        <radialGradient id={`${id}-wheel`}>
          <stop stopColor="#b9c1bd" />
          <stop offset=".65" stopColor="#7e8b85" />
          <stop offset=".68" stopColor="#242e2a" />
          <stop offset="1" stopColor="#151c19" />
        </radialGradient>
        <filter id={`${id}-shadow`}>
          <feGaussianBlur stdDeviation="7" />
        </filter>
      </defs>
      <ellipse
        cx="330"
        cy="270"
        rx="253"
        ry="17"
        fill="#173b32"
        opacity=".18"
        filter={`url(#${id}-shadow)`}
      />
      <path
        d="M73 209 Q82 188 135 176 L201 119 Q216 107 256 105 L372 106 Q399 106 433 133 L490 174 L565 188 Q587 193 590 215 L588 246 Q570 260 530 259 L116 260 Q79 257 68 241Z"
        fill={`url(#${id}-body)`}
        stroke={c[2]}
        strokeWidth="2"
      />
      <path
        d="M172 175 L218 128 Q226 120 258 120 L368 121 Q389 121 415 141 L459 175Z"
        fill={`url(#${id}-glass)`}
        stroke={c[0]}
        strokeWidth="3"
      />
      <path
        d="M293 121L288 175M376 125L397 175"
        stroke={c[1]}
        strokeWidth="9"
      />
      <path d="M215 129L196 174L251 174L269 120" fill="#c9dad2" opacity=".2" />
      <path
        d="M297 184L295 241M405 183L421 240M155 190Q334 180 497 191"
        fill="none"
        stroke={c[2]}
        strokeWidth="1.6"
        opacity=".6"
      />
      <path
        d="M79 216L120 216L137 196L94 202M529 202L575 211L578 224L539 218"
        fill="#e8f4d7"
        stroke={c[2]}
        strokeWidth="2"
      />
      <path
        d="M80 239L122 239M528 238L580 240"
        stroke="#263c32"
        strokeWidth="7"
        strokeLinecap="round"
      />
      <path
        d="M218 190h22M333 191h21"
        stroke={c[0]}
        strokeWidth="4"
        strokeLinecap="round"
      />
      <path
        d="M128 252a47 47 0 0 1 94 0M437 251a47 47 0 0 1 94 0"
        fill="#24332b"
      />
      {[175, 484].map((x) => (
        <g key={x}>
          <circle cx={x} cy="252" r="37" fill={`url(#${id}-wheel)`} />
          <circle cx={x} cy="252" r="21" fill="#78877f" />
          {[0, 60, 120].map((a) => (
            <path
              key={a}
              d={`M${x - 18} 252h36`}
              transform={`rotate(${a} ${x} 252)`}
              stroke="#d4dbd5"
              strokeWidth="5"
            />
          ))}
          <circle cx={x} cy="252" r="6" fill="#54675c" />
        </g>
      ))}
      <path
        d="M102 225Q294 216 565 230"
        stroke={c[0]}
        opacity=".3"
        fill="none"
        strokeWidth="3"
      />
      <path
        d="M431 167l22-4 13 9-1 9h-30z"
        fill={c[1]}
        stroke={c[2]}
        strokeWidth="2"
      />
    </svg>
  );
}

package shop

import (
	"fmt"
	"html"
	"strings"
)

type palette struct{ from, to, accent string }

var categoryPalettes = map[string]palette{
	"laptops":     {"#6366f1", "#312e81", "#c7d2fe"},
	"phones":      {"#06b6d4", "#155e75", "#a5f3fc"},
	"audio":       {"#f43f5e", "#881337", "#fecdd3"},
	"wearables":   {"#10b981", "#064e3b", "#a7f3d0"},
	"cameras":     {"#f59e0b", "#78350f", "#fde68a"},
	"accessories": {"#8b5cf6", "#4c1d95", "#ddd6fe"},
}

// Simple line icons, drawn in a 120x120 box centred at (300,230).
var categoryIcons = map[string]string{
	"laptops":     `<rect x="-50" y="-34" width="100" height="64" rx="6"/><path d="M-66 38h132l-8 12h-116z"/>`,
	"phones":      `<rect x="-30" y="-56" width="60" height="112" rx="10"/><path d="M-8 44h16"/>`,
	"audio":       `<path d="M-46 20v-14a46 46 0 0 1 92 0v14"/><rect x="-54" y="10" width="20" height="40" rx="6"/><rect x="34" y="10" width="20" height="40" rx="6"/>`,
	"wearables":   `<rect x="-30" y="-30" width="60" height="60" rx="14"/><path d="M-18 -30l6 -28h24l6 28M-18 30l6 28h24l6 -28"/><path d="M0 -12v12l10 8"/>`,
	"cameras":     `<rect x="-56" y="-30" width="112" height="72" rx="10"/><path d="M-22 -30l8 -16h28l8 16"/><circle cx="0" cy="6" r="22"/>`,
	"accessories": `<path d="M-30 -50h60v100h-60z" transform="rotate(-20)"/><circle cx="0" cy="-20" r="6"/><path d="M0 -6v28"/>`,
}

func initials(name string) string {
	var b strings.Builder
	for _, w := range strings.Fields(name) {
		r := []rune(w)[0]
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
		if b.Len() >= 3 {
			break
		}
	}
	return b.String()
}

func productSVG(p *Product) []byte {
	pal, ok := categoryPalettes[p.CategorySlug]
	if !ok {
		pal = palette{"#64748b", "#1e293b", "#e2e8f0"}
	}
	icon := categoryIcons[p.CategorySlug]
	return []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 450" role="img" aria-label="%[1]s">
<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="%[2]s"/><stop offset="1" stop-color="%[3]s"/></linearGradient>
<radialGradient id="h" cx="0.3" cy="0.2" r="0.8"><stop offset="0" stop-color="#fff" stop-opacity="0.35"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient></defs>
<rect width="600" height="450" fill="url(#g)"/><rect width="600" height="450" fill="url(#h)"/>
<circle cx="520" cy="70" r="120" fill="#fff" fill-opacity="0.06"/><circle cx="60" cy="420" r="160" fill="#fff" fill-opacity="0.05"/>
<g transform="translate(300 200)" fill="none" stroke="%[4]s" stroke-width="7" stroke-linecap="round" stroke-linejoin="round">%[5]s</g>
<text x="300" y="350" text-anchor="middle" font-family="Inter,Helvetica,Arial,sans-serif" font-size="44" font-weight="700" fill="#fff" letter-spacing="4">%[6]s</text>
<text x="300" y="392" text-anchor="middle" font-family="Inter,Helvetica,Arial,sans-serif" font-size="22" fill="%[4]s">%[7]s</text>
</svg>`, html.EscapeString(p.Name), pal.from, pal.to, pal.accent, icon, html.EscapeString(initials(p.Name)), html.EscapeString(p.Brand)))
}

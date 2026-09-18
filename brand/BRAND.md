# Facet Brand

## Identity

Facet's approved identity is **Precision Dovetail Assembly (Puzzle B)**. The
mark combines three equal rounded tiles with a four-piece foreground assembly.
The open play aperture in the foreground reveals the green tile below it.

Use **Facet** as the formal product name in prose, headings, metadata, and
accessible names. The visual wordmark is lowercase **facet.**, with the
terminal period in Facet Red.

## Signature Colors

| Token | Value | Role |
| --- | --- | --- |
| Begonia | `#F36C7A` | Back tile |
| Green | `#10B981` | Middle tile and play-aperture reveal |
| Red | `#D92D48` | Foreground pieces and wordmark period |

Use the signature colors at these exact values. Neutrals may support text,
backgrounds, and monochrome applications. Do not introduce additional
signature colors, rainbow treatments, gradient fills, or glow effects into the
identity.

## Mark Construction

The canonical mark uses `viewBox="0 0 120 120"`.

- Back tile: `x="20" y="16" width="58" height="58" rx="12"`, Begonia.
- Middle tile: `x="31" y="27" width="58" height="58" rx="12"`, Green.
- Foreground nominal bounds: `x="42" y="38" width="58" height="58"`, Red.
- The foreground consists of four separate pieces. Keep their paths and
  translations unchanged.

```svg
<path fill-rule="evenodd" d="M54 38 H89 V48 L94 52 L89 56 V68 H83 V83 H72 L68 87 L64 83 H42 V50 C42 43.37 47.37 38 54 38 Z M61 54 L83 67 L61 80 Z"/>
<path d="M89 38 C95.08 38 100 42.92 100 49 V68 H89 V56 L94 52 L89 48 Z" transform="translate(2 -1.5)"/>
<path d="M83 68 H100 V84 C100 90.63 94.63 96 88 96 H83 Z" transform="translate(2 2)"/>
<path d="M42 83 H64 L68 87 L72 83 H83 V96 H54 C47.37 96 42 90.63 42 83 Z" transform="translate(-1 2)"/>
```

The first path uses the even-odd fill rule. Its triangular subpath is an
aperture, not a white triangle or an additional shape.

## Logo Set

- `logos/mark.svg`: primary full-color mark for light neutral surfaces.
- `logos/mark-inverse.svg`: full-color mark prepared for dark neutral surfaces.
- `logos/lockup.svg`: primary mark and dark visual wordmark.
- `logos/lockup-inverse.svg`: primary mark and white visual wordmark.
- `logos/wordmark.svg`: dark visual wordmark only.
- `logos/wordmark-inverse.svg`: white visual wordmark only.
- `logos/mono-black.svg`: single-color mark using black opacity to retain the
  tile construction.
- `logos/mono-white.svg`: single-color mark using white opacity to retain the
  tile construction.

Use the inverse variants on dark neutral surfaces. Do not recolor the
full-color mark, close the play aperture, merge the foreground pieces, change
the offsets, or place effects behind the logo.

## Typography

The visual wordmark uses the existing Facet display stack: Space Grotesk,
Inter, then a neutral sans-serif fallback. The SVG wordmarks keep text live so
they do not bundle or embed a font. For deterministic production artwork,
outline the wordmark using a properly licensed copy of the selected typeface
without changing its spelling, case, or red terminal period.

## Accessibility

- When the mark sits beside the visual wordmark, use an empty image alternative
  and give the enclosing link or landmark the accessible name `Facet`.
- When the mark appears alone and conveys identity, use `Facet` as its text
  alternative.
- Keep the lowercase visual wordmark out of the accessibility tree when the
  enclosing element already has a formal accessible name.
- Do not use the red period alone as the accessible product name.

## Web Projections

The root `brand/` directory is the canonical kit. Files in `docs/` are explicit
GitHub Pages projections:

| Brand asset | Site projection |
| --- | --- |
| `icons/favicon.svg` | `docs/favicon.svg` |
| `logos/mark.svg` | `docs/logo-mark.svg` |
| `og/og-default.svg` | `docs/og-default.svg` |
| `og/og-default.png` | `docs/og-default.png` |

When a canonical asset changes through an approved brand update, refresh its
projection in the same change and inspect both documentation pages locally.
The SVG is the source for the 1200x630 PNG derivative; regenerate and verify it
from the repository root with:

```text
node scripts/generate-brand-og.mjs
node scripts/generate-brand-og.mjs --check
node --test scripts/brand-assets.test.mjs
```

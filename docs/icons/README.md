# Mini-Orca — MO Orbit v2 logo assets

Prepared from the user-provided logo reference image. The transparent SVG symbol is a faithful **vector trace of the reference screenshot** (not an original editable design document); the original visible M/O geometry and orca eye are preserved. The original screenshot's antialiasing and gradients were simplified to solid theme colors for reliable UI rendering.

The [desktop web client](../../desktop/webapp/README.md) uses this directory as
its branding source. Its build copies the theme marks and favicon into the
embedded frontend, and Wails creates the native macOS icon from the Graphite
1024-pixel app-icon PNG. The remaining exports are available for other consumers.

## What to use

- Default dark UI header: `graphite/ui/mo-mark.svg` (transparent), 24–32px tall. Display `Mini-Orca` as your own UI text, or use `graphite/ui/titlebar-lockup.svg` for the complete logo + text.
- Default desktop application icon: `graphite/app-icon/app-icon.svg`, or PNG sizes. This includes the dark rounded-square background and gold border.
- macOS packaged app icon: `mini-orca.icns` (Graphite).
- Windows packaged icon: `mini-orca.ico` (Graphite).
- Browser tab: `favicon.png` (Graphite), and other sizes in `graphite/app-icon`.
- Theme-aware UI: use `porcelain/` for light background and `midnight/` for blue background.
- Signal UI header: `signal/ui/mo-mark.svg` preserves the mark geometry with red and yellow. This theme supplies a header SVG only; the packaged app icon remains Graphite.
- Crimson Violet UI header: `crimson-violet/ui/mo-mark.svg` uses lilac and red, also as a header SVG only.
- `reference-extraction/mo-mark-source-colors.png` is the screenshot-based extraction to inspect source appearance; **prefer the SVG** for actual UI usage.
- `*-24h.png` names mean the transparent mark PNG is **24 pixels high**; aspect ratio is preserved.

## Colors

| Theme | Background | Primary | Accent |
|---|---|---|---|
| Graphite | `#1C1F1B` | `#F5F1E8` | `#F8C274` |
| Porcelain | `#F8F7F2` | `#164B3A` | `#346448` |
| Midnight | `#0E243A` | `#EAF4FF` | `#78CDCF` |
| Signal | `#151215` | `#FF5A61` | `#FFDC45` |
| Crimson Violet | `#2B1645` | `#CCA6FF` | `#FF5A70` |

## Importing into web/HTMX UI (Go)

Copy the `graphite`/`porcelain`/`midnight` folders into the frontend's **actual static asset directory** and ensure the server serves them under `/assets/branding`. Example:

```html
<div class="app-brand">
  <img src="/assets/branding/graphite/ui/mo-mark.svg"
       width="41" height="28" alt="Mini-Orca logo" />
  <span>mini-orca<span class="brand-dot">.</span></span>
</div>
```

```css
.app-brand { display:flex; align-items:center; gap:9px; font-weight:750; color:#F5F1E8; }
.app-brand img { width:auto; height:28px; flex-shrink:0; }
.brand-dot { color:#F8C274; }
```

Alternatively use `<img src="/assets/branding/graphite/ui/titlebar-lockup.svg" height="32" alt="Mini-Orca">` instead of separate text. Swap `graphite` for `porcelain` or `midnight` when changing theme.

## Importing into Kotlin Compose Desktop (if used)

Copy e.g. `graphite/ui/mo-mark-32h.png` to `src/main/resources/branding/mo-mark.png`, then:

```kotlin
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.height
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp

Image(
    painter = painterResource("branding/mo-mark.png"),
    contentDescription = "Mini-Orca",
    modifier = Modifier.height(28.dp)
)
```

Resource paths depend on the actual module layout; use the module that renders the title bar.

## Packaging

- **Do not** use the transparent mark PNG as the app icon — use the square `app-icon` artwork.
- **Do not** apply CSS `border-radius` to the transparent mark, only to the square app icon if desired.
- For high-DPI desktop UI prefer an SVG or a 2x/3x PNG for the rendered size.
- All SVGs are self-contained and do not require any installed font; titlebar text is outlined.

## Source note

The reference is a composite mockup, not a native vector master. This vectorization was reconstructed from its large raster logo. For a pixel-perfect designer-grade master beyond the screenshot resolution, request the original standalone SVG from the artist/tool that made the mockup.

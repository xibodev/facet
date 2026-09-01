# Third-Party Notices

Video Kit is derived from [OpenMontage](https://github.com/calesthio/OpenMontage)
at pinned commit `cd9f3c1f03368be87b140af494914b8ee4e3c7a4`, licensed
under the GNU Affero General Public License, version 3. See `LICENSE` and
`DONORS.md`. Unit-level direct-copy, adapted-copy, behavioral-port, and original
classifications are recorded in `PROVENANCE.md`.

No composer, implementation, package, font, media, motion-capture, or other
third-party asset is claimed as imported by this notice. When a component is
imported, its applicable license, attribution, and source record must be added
here and to `PROVENANCE.md`.

Video Kit invokes user-installed FFmpeg and ffprobe executables at runtime. It
does not copy, link, vendor, or distribute FFmpeg binaries or source. The terms
applicable to the user's FFmpeg build remain independent; see
<https://ffmpeg.org/legal.html>.

## Independent Components

- Remotion and npm packages retain their separate licenses. Importing the
  OpenMontage composer does not relicense those packages under the GNU AGPL.
- HyperFrames provenance is
  [heygen-com/hyperframes](https://github.com/heygen-com/hyperframes), commit
  `3351fb1a`, tag `v0.7.17`. Its applicable terms and notices must be verified
  and preserved if HyperFrames material is imported.
- If Ink Theater assets are imported, preserve the Patrick Hand font's SIL Open
  Font License 1.1 notice and the applicable CMU Graphics Lab Motion Capture
  Database provenance and notices for imported motion-capture data.
- If the upstream Pixabay sound effects are imported, preserve their individual
  credits and applicable Pixabay Content License notices.
- Provider SDKs and media, font, and other asset files retain their own terms,
  licenses, and attribution requirements.

## Phase 2 Update Gate

Before Phase 2 imports or adapts any composer, implementation, dependency,
fixture, font, media, motion-capture data, or other asset, update this file with
the component's verified license and required notices and add its unit-level
classification to `PROVENANCE.md`.

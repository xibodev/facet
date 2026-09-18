# Third-Party Notices

Facet is licensed under the GNU Affero General Public License, version 3 or
later. See `LICENSE`.

Facet invokes user-installed FFmpeg and ffprobe executables at runtime; it does
not vendor or distribute those binaries. The terms for the user's FFmpeg build
remain independent. See <https://ffmpeg.org/legal.html>.

The Remotion composer and its npm dependencies retain their respective
licenses. Their package identities and versions are recorded in
`remotion-composer/package-lock.json`.

Optional tools such as HyperFrames, Piper, and gflow are installed or supplied
separately and retain their own licenses and notices. Facet's installer
manifest identifies these optional components but does not relicense them.

Media, fonts, provider SDKs, and other assets supplied by a user or fetched from
a provider retain their original rights and attribution requirements. Do not
ship an imported asset without preserving its applicable notice.

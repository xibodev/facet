# Third-Party Notices

Facet is licensed under the GNU Affero General Public License, version 3 or
later. See `LICENSE`.

## Retained behavioral Go ports

Facet retains behavioral Go ports derived from
[OpenMontage](https://github.com/calesthio/OpenMontage) at commit
`cd9f3c1f03368be87b140af494914b8ee4e3c7a4`, licensed under AGPL-3.0, in only
these implementation categories: toolbox contract/registry/process execution;
media probe/frame sampling/scene detection; supplied-footage source editing;
audio mixing/normalization; and technical output review.

The retained derivative implementations remain subject to the AGPL-3.0
requirements, including preservation of applicable copyright and license
notices, provision of Corresponding Source when conveying covered works, and
the source-code offer for users interacting with modified covered software over
a network.

After this cleanup, no donor skills, pipelines, schemas, fixtures, composer
source, or product guidance are shipped from that donor.

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

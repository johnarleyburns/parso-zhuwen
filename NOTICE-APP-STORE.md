# Additional Permission under GNU GPL version 3, section 7 — App Store Distribution

Copyright (c) 2026 John Arley Burns (Parso). All files in this repository are
licensed under the GNU General Public License, version 3 (see `LICENSE`), with
the following additional permission granted by the copyright holder under
GPLv3 section 7:

> **App Store distribution permission.** Notwithstanding any provision of the
> GNU GPLv3, you are permitted to convey covered works, and compiled/object
> forms of covered works, through application distribution platforms operated
> by Apple Inc. (including the Apple App Store and TestFlight), and end users
> are permitted to receive and use such copies under the usage rules of those
> platforms, even where those platforms' terms of service impose restrictions
> that would otherwise be incompatible with the GPLv3. This permission does not
> waive any other requirement of the GPLv3 (including source availability under
> sections 4–6) for any party conveying the work, and it may be removed from
> modified versions per the terms of section 7.

## Why this exists

Apple's platform terms impose usage rules that historically conflict with
GPL distribution terms (the VLC precedent). This section 7 grant resolves the
conflict explicitly, so the project can be both honestly copyleft and shipped
on the App Store.

## Contributor requirement (keeps the grant valid)

The additional permission above is only cleanly grantable while the copyright
in the covered work is held by the project owner or contributors who agree to
it. Therefore all contributions to this repository require:

1. **Developer Certificate of Origin (DCO):** every commit must be signed off
   (`git commit -s`), asserting the contributor's right to submit the work; and
2. **License-with-exception agreement:** by submitting a contribution, the
   contributor licenses it under GPLv3 *including the section 7 additional
   permission above*.

State this in `CONTRIBUTING.md` and enforce sign-off in CI.

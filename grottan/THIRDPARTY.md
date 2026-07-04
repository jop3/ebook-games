# Third-party content & attribution

## Colossal Cave Adventure — via Open Adventure

Grottan's world data (rooms, objects, vocabulary, travel table, and message
text) is generated from **Open Adventure**'s `adventure.yaml` — Eric S.
Raymond's modernization of Will Crowther & Don Woods' 1976 *Colossal Cave
Adventure*, released with the original authors' approval under the BSD-2-Clause
license.

- Upstream: <https://gitlab.com/esr/open-adventure>
- Data source: `adventure.yaml` (ingested at build time by `scratchpad/advgen/`
  into `grottan/story/storydata_gen.go`; only the surface + first-cave Phase-1
  subset is included — see `SPEC_TEXT_ADVENTURE.md` §2).
- Upstream commit ingested: `993291a`.

**Credit line shown on the in-app rules screen:**

> Baserat på Colossal Cave Adventure av Will Crowther & Don Woods, via Open
> Adventure (Eric S. Raymond), BSD-2-Clause.

Copyright holders:

- `SPDX-FileCopyrightText: (C) 1977, 2005 Will Crowther and Don Woods`
- `SPDX-FileCopyrightText: (C) Eric S. Raymond <esr@thyrsus.com>`
- `SPDX-License-Identifier: BSD-2-Clause`

### License (Open Adventure `COPYING`, BSD 2-Clause)

```
		BSD 2-Clause LICENSE

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

1. Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright
notice, this list of conditions and the following disclaimer in the
documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

## SDK — dennwc/inkview

The vendored `third_party/inkview/` is a copy of
[dennwc/inkview](https://github.com/dennwc/inkview) (a Go wrapper over
PocketBook's libinkview), under its own MIT license (`third_party/inkview/LICENSE`).

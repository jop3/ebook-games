# I rad

En spelmotor för "X i rad"-spel till **PocketBook Verse Pro (PB634)**, byggd på
[dennwc/inkview](https://github.com/dennwc/inkview).

Klassikerna är konfigurationer, inte separata implementationer: Tre i rad, Tre i
kvarn, Fyra i rad, Fem i rad/Gomoku, vertikala varianter och hindervarianter.

## Spelare: 1–4

I startmenyn finns en **Läge**-knapp som växlar mellan:

| Läge | Spelare |
|---|---|
| Mot AI | 1 människa + AI (heuristisk) |
| 2 spelare | 2 människor, hot-seat |
| 3 spelare | 3 människor, hot-seat |
| 4 spelare | 4 människor, hot-seat |

Markörer på den gråskaliga e-ink-skärmen: **X** (spelare 1), **O** (spelare 2),
**△** (spelare 3), **□** (spelare 4). Vinst = `Vinstlängd` i rad för valfri
spelare. AI används bara i tvåspelarläget; med 3–4 spelare är alla mänskliga.

## Projektstruktur

```
irad/
  main.go              ink.App: meny, spel-loop, AI-turer, e-ink-uppdatering
  game/                ren regelmotor (ingen SDK/UI-koppling, enhetstestad)
    board.go           Board, ValidMoves, Apply, CheckWin
    state.go           GameState, turordning (2–4 spelare), tillståndsmaskin
    ai.go              hot-baserad heuristik (endast 2-spelarläge)
    presets.go         presettabell + hindermönster
    *_test.go          14 enhetstester
  ui/
    layout.go          skärm- ⇄ cellkoordinater
    render.go          ritrutiner (rutnät, X/O/△/□, status, knappar)
    input.go           touch → Move (§8)
    menu.go            startmeny: variant, läge, Anpassad-steppers
  third_party/inkview/ vendrad SDK (via replace i go.mod)
  .github/workflows/   moln-bygge av irad.app
```

## Bygga `irad.app`

Binären är en cgo-ARM-binär som länkar mot `libinkview`. Det kräver
PocketBook Go SDK:t, som distribueras som en Docker-image.

### Alternativ A — i molnet (inget lokalt installerat)

1. Pusha repot till GitHub.
2. Gå till **Actions** → kör workflowen **Build irad.app** (startar även
   automatiskt vid push).
3. Ladda ner artefakten **irad-app**, packa upp → `irad.app`.

### Alternativ B — lokalt med Docker

```bash
docker run --rm -v "$PWD:/app" -w /app \
  sunsung/pocketbook-go-sdk:latest build -o irad.app .
```

(Alternativa images: `dennwc/pocketbook-go-sdk`,
`5keeve/pocketbook-go-sdk:6.3.0-b288-v1`.)

## Installera på enheten

1. Anslut PocketBooken via USB.
2. Kopiera `irad.app` till mappen **`applications/`** på enhetens rot
   (på den här datorn: `D:\applications\irad.app`).
3. Mata ut säkert. På läsaren: **Appar → User Applications → irad**.

## Utveckling / test

Regelmotorn (`game/`) är ren Go och testas utan SDK:

```bash
go test ./game/
```

UI- och huvudpaketen kan type-checkas mot en pure-Go-stub av inkview när ingen
C-toolchain finns (se utvecklingsanteckningar). Själva `.app`-länkningen sker
bara i SDK-imagen.

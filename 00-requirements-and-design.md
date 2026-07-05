# Zhuwen — Requirements & Design Document (v3 — FINAL for handoff)

**Name:** Zhuwen (朱文) — see §1.1 for the decision record
**Product:** Provably-comprehensible Mandarin reading & listening app
**Platform:** Native iOS (SwiftUI), iOS 17 floor, iPhone-first
**Status:** Pre-handoff design. Agent-ready checkpoints in §14.
**Companion doc:** `mockups.html` — 15 screen mockups referenced throughout as [M1]–[M15].

**v3 changelog:**
- New product invariant **I6**: every story — every tier, every band, including the
  ex-novo quota and Foundations micro-stories — ships with a human-made image from
  Wikimedia Commons. No AI imagery, no imageless stories. Pack build fails otherwise.
- FR-12.4 upgraded from "cover art" to the I6 mandate; §8A extended with per-story
  inventory math, reuse rules, and curation budget; NFR-4 updated for image weight.
- Mockups updated: story surfaces (M4, M11) now show Commons covers.
- Document is now FINAL for handoff; see companion `01-agentic-handoff.md`.

**v2 changelog (supersedes v1; nothing removed, only expanded):**
- Name decided: **Zhuwen** (§1.1), replacing placeholder "Cinnabar" throughout.
- Factory TTS vendor decided: **CosyVoice 3.0**, alternate **Qwen3-TTS** (§7.1).
- New §5A: **Foundations** — the zero-to-story visual bootstrap program (picture-word
  cards from Wikimedia Commons photographs), with new FR-11 series and mockup [M14].
- New §6A: **Source Canon** — stories are adapted from the human corpus (Chinese
  mythology, legends, chengyu tales, history anecdotes, Aesop, public-domain folk and
  children's tales via Wikipedia/Wikisource), not invented ex novo; new FR-12 series,
  factory changes in §8, mockup [M15].
- New §8A: **Commons image pipeline** (licensing, attribution, curation gates).
- Risk table and checkpoints extended accordingly.

---

## 1.1 Name decision record

**Chosen: Zhuwen (朱文).** In seal carving, 朱文 ("vermilion characters") is the
relief style in which the *characters themselves* print in cinnabar-red seal paste —
red words on white paper. It names exactly what the app's signature element already
is (the earned 读完为证 seal), it contains 文 (writing, language, text), it is short,
ownable, and pronounceable ("joo-wen"), and it fits the Parso convention of invented/
opaque marks (Cladiron, Lorewave, Acalum) carried by a descriptive App Store subtitle:
**"Zhuwen — Chinese by Reading."**

Availability check (July 2026): App Store search for "Zhuwen" returns no app of that
name and nothing in language learning; no similar-space website found. The prior
placeholder "Cinnabar" was rejected because the bare name is already taken on the
App Store (a loyalty-club app named exactly "Cinnabar", plus Cinnabar Hills Golf) and
cinnabar.app is registered-and-parked. "SealScript" was also considered and rejected —
an existing App Store app of that exact name teaches Chinese seal script.
**Action items at submission time:** re-verify App Store name uniqueness in App Store
Connect, run a USPTO/EUIPO knockout search for "Zhuwen" in class 9/41, and register
zhuwen.app + zhuwen.parso.guru. Naming is cheap to change until CP-08; the mark is
referenced from a single constant in code and one variable in the factory config.

---

## 1. Vision & thesis

Zhuwen teaches Mandarin (mainland, simplified) through pregenerated stories that are
**mathematically guaranteed to be comprehensible**: every story surfaced to the learner has
≥98% of its running tokens inside that learner's known-word set, and the ≤2% unknown words
are deliberately chosen frontier words the engine wants to teach next (Krashen i+1,
operationalized via Hu & Nation's 98% lexical-coverage threshold).

The differentiator is not "AI stories" (commodity by 2026) but the **enforced invariant**:
no competitor maintains a per-user known-word model and provably selects content against it.
Everyone else buckets by level. Zhuwen selects by *your exact lexicon*.

**Positioning line:** "Every story is 98% words you already know. The other 2% is how you grow."

### Non-goals (v1)
- No speaking practice, no pronunciation scoring, no writing/handwriting.
  V1 trains **reading + listening** and estimates CEFR R/L only. (Production skills = v2+.)
- No runtime LLM anywhere. No chat. No on-demand generation.
- No traditional characters, no Taiwan/HK variants.
- No Android, no iPad-optimized layout (runs scaled), no watch app.
- No social features, leaderboards, or streak-guilt mechanics.

---

## 2. Product invariants (locked; enforce in code and CI)

| # | Invariant | Enforcement |
|---|-----------|-------------|
| I1 | **Coverage gate:** a story may only be *recommended* to a learner if token coverage against their known set ≥ 98%, and all uncovered word types are members of the learner's frontier queue or proper nouns with gloss support. | `StoryCandidate` has a private init; only `CoverageGate.evaluate()` can construct one. Unit-tested with fixture lexicons. |
| I2 | **No accounts, no server state.** All learner data lives on device. Network is limited to: CDN pack downloads (anonymous GET), StoreKit 2. Optional iCloud sync uses the user's private CloudKit database only. | ATS pinned to pack CDN domain; CI script greps for URLSession usage outside `PackClient`. Privacy manifest declares zero tracking domains. |
| I3 | **Pregenerated content only.** Every story, sentence, gloss, audio file, and comprehension question ships from the factory, quality-gated before release. The app contains no generation code. | Content packs are signed; app verifies signature before install. |
| I4 | **Evidence-gated pedagogy claims.** Any claim shown in UI ("98% coverage enables unassisted comprehension") carries a citation from the registry (Cladiron pattern: private-init `Citation` type, CI validates DOIs). | `PedagogyClaim` type mirrors CadenceCore citation registry. |
| I5 | **Every tap teaches the model.** Dictionary lookups, comprehension answers, review grades, and "mark as known" all feed the known-word model. No modal quizzes required to keep the model current. | Event log table; model update is a pure function of events (replayable). |
| I6 | **Every story has a human-made Commons image.** Every story in every pack — all canon tiers, the ex-novo quota, and Foundations micro-stories — carries a cover image sourced from Wikimedia Commons through the §8A pipeline, with license + attribution shipped. AI-generated imagery is banned product-wide. | Factory pack builder hard-fails on any `story.cover_image_id = NULL` or on any image whose provenance record is missing/AI-categorized; CI golden test includes an imageless story fixture that must be rejected. |

---

## 3. Users & core loop

**Persona A — the deliberate beginner (primary).** Adult self-studier, 0–18 months in,
knows Duolingo isn't working, has heard of comprehensible input via YouTube/Reddit.
Wants a system, not a game. Pays for tools (owns Pleco add-ons).

**Persona B — the plateau intermediate.** HSK3–4 (old scale), can read graded content but
native content is a wall. Wants volume of input at exactly the right difficulty.

**Core loop (daily, 10–25 min):**
1. Open app → today's story, pre-selected by the engine ([M4]).
2. Read with tap-to-gloss; unknown frontier words appear 3+ times each ([M5], [M6]).
3. Optional listening pass with word-synced highlight ([M7]).
4. Three comprehension questions; passing stamps the story with a cinnabar seal ([M8]).
5. Short review session (FSRS) of due words, always in sentence context ([M9]).
6. Progress dashboard shows lexicon growth and CEFR/HSK-3.0 band estimate ([M10]).

---

## 4. Functional requirements

### FR-1 Placement (first run) — [M2], [M3]
- FR-1.1 Adaptive yes/no vocabulary checklist (VST-style): learner marks words known/unknown;
  pseudoword foils (plausible but nonexistent 2-char compounds) calibrate overconfidence.
  Items sampled by HSK-3.0 level strata × corpus frequency. 60–120 items, ~4 minutes.
- FR-1.2 Output: a logistic knowledge curve over frequency rank → probabilistic seed of the
  known-word model (each lexicon word gets P(known)); plus CEFR band estimate (A0–B2) and
  HSK-3.0 level estimate (1–6).
- FR-1.3 Two short reading passages at the estimated band with comprehension checks refine
  the estimate (catch character-recognition gaps in learners with strong spoken vocab).
- FR-1.4 Absolute-beginner path skips the test: model seeds empty; the learner enters
  the **Foundations** visual bootstrap program (full spec §5A, FR-11 series), which
  carries them from zero to the ~300-word threshold where the story lattice takes over.
- FR-1.5 Placement is repeatable from Settings; re-placement merges (never destroys) state.

### FR-2 Known-word model
- FR-2.1 Per lexicon word: state ∈ {unseen, introduced, learning, known, mastered},
  P(known) ∈ [0,1], FSRS memory params, exposure count, lookup count, last-seen timestamp.
- FR-2.2 State transitions driven by events: story exposure (word seen, not tapped → evidence
  of knowing), lookup (evidence of not knowing), review grade, explicit "mark known/unknown",
  comprehension-question outcomes on sentences containing the word.
- FR-2.3 "Effective known set" for the coverage gate = words with P(known) ≥ 0.8, plus
  the frontier words currently in "learning".
- FR-2.4 Frontier queue: next candidate words ordered by (HSK-3.0 level, corpus frequency,
  character-component familiarity bonus — words whose characters the learner already knows
  rank earlier).

### FR-3 Story selection & the lattice — [M4], [M11]
- FR-3.1 The pack library ("lattice") indexes every story by its exact word-type set
  (bitmap over the ~11k-word HSK-3.0 lexicon), band, length, topics, grammar patterns,
  and audio availability.
- FR-3.2 Daily selection scores gated candidates by: frontier-word payload (does it introduce
  words due for introduction? do introduced words recur?), SRS payload (does it re-expose
  words due for review? — *reading as spaced repetition*), topic interest (onboarding chips +
  reading history), novelty (never repeat unless requested), and length fit.
- FR-3.3 Library view lets the learner browse everything, but each story shows its personal
  **fit badge**: coverage % against *their* lexicon and count of new words. Stories below 98%
  are visible but marked "above your level — N unknown words" (honesty, not lockout).
- FR-3.4 Courses: multi-chapter arcs whose chapters step the lexicon forward deliberately
  (chapter N+1 assumes chapter N's frontier words are now "learning").

### FR-4 Reader — [M5], [M6]
- FR-4.1 Text rendered word-segmented (segmentation shipped in pack data; the app never
  segments). Tap any word → gloss sheet: pinyin (tone-colored), definitions, HSK level,
  character breakdown, example from *this* story, audio button, Add-to-review.
- FR-4.2 Frontier words carry a subtle first-encounter underline (cinnabar dotted) on first
  occurrence only; toggleable.
- FR-4.3 Pinyin display modes: off / frontier-only / all (default: frontier-only ≤A2, off above).
- FR-4.4 Sentence-level translation on demand (long-press sentence), factory-pregenerated.
- FR-4.5 Font: Songti (serif) for story text, adjustable size; dark & light themes.
- FR-4.6 Every lookup logs an event (I5). Reading position persists.

### FR-5 Listening — [M7]
- FR-5.1 Karaoke mode: pregenerated neural audio with word-level timestamps; current word
  highlights; tap a word to seek. Speeds 0.6×–1.2× (time-stretch, pitch-preserved).
- FR-5.2 Blind-listening mode: audio first with text hidden, reveal after; counts toward
  listening-skill estimate separately from reading.
- FR-5.3 Background audio + lock-screen controls (AVAudioSession playback category,
  MPNowPlayingInfoCenter). Reuse patterns from Lorewave's player.
- FR-5.4 Fallback: stories without pack audio use AVSpeechSynthesizer zh-CN on device
  (see §7 decision); word highlight via `willSpeakRangeOfSpeechString` delegate.

### FR-6 Comprehension & assessment — [M8], [M10]
- FR-6.1 Each story ships 3 factory-generated comprehension questions (multiple choice,
  in Chinese at or below story band; distractors quality-gated).
- FR-6.2 Passing (≥2/3) stamps the story (seal motif) and boosts P(known) for its words.
- FR-6.3 CEFR dashboard: estimated reading band and listening band with confidence interval,
  mapped can-do statements, plus HSK-3.0 level estimate and "words to next level".
  All estimates labeled as estimates; no certification claims (I4 citation on methodology).
- FR-6.4 Optional monthly "checkpoint": a 10-minute mini-assessment (fresh passages held out
  from the recommendation pool) that re-anchors the band estimate.

### FR-7 Review (SRS) — [M9]
- FR-7.1 FSRS scheduler (open algorithm, on-device). Cards are **sentence-context** cards:
  the word appears inside a sentence drawn from stories the learner has read.
- FR-7.2 Review is capped by default (20 cards/day) and framed as optional maintenance;
  the primary SRS mechanism is re-encounter through story selection (FR-3.2).
- FR-7.3 Grades feed the known-word model (FR-2.2).

### FR-8 Packs & offline — [M13]
- FR-8.1 App ships with the Foundations program (incl. its ~220 curated images) +
  A1 starter pack embedded (~55 MB in v2, was ~25 MB pre-images).
- FR-8.2 Additional packs (by band: A1, A2, B1…; audio included) download from CDN,
  signed manifest verified on device. Fully usable offline thereafter.
- FR-8.3 Pack manager UI: size, delete, re-download; no identifiers sent (anonymous GET).

### FR-9 Monetization (no ads) — [M12]
- FR-9.1 Free tier: placement, Foundations course, one engine-selected story/day,
  dictionary, review (capped), progress basics.
- FR-9.2 Pro: unlimited stories, full lattice browsing, listening packs & blind mode,
  full dashboard, monthly checkpoints, all future packs.
- FR-9.3 Pricing: $7.99/mo, $59.99/yr, $149.99 lifetime; 30-day free trial on annual
  (Cladiron playbook). StoreKit 2 only; no receipt server.
- FR-9.4 Paywall copy is factual, single screen, dismissible; never interrupts an
  in-progress story.

### FR-10 Settings & data — [M13]
- FR-10.1 Toggles: pinyin mode, frontier underline, theme, font size, audio voice
  (pack voice vs system TTS), daily review cap.
- FR-10.2 Optional iCloud sync (private CloudKit DB) of learner state; off by default.
- FR-10.3 Export everything (JSON) and erase everything. Privacy page states the
  network-surface guarantee (I2) in plain language.

---

## 5. Non-functional requirements

- NFR-1 Cold launch < 600 ms to Today screen; story open < 150 ms (all local).
- NFR-2 Selector scores 5,000-story lattice in < 50 ms on A15 (bitmap AND + popcount).
- NFR-3 App binary + embedded starter content ≤ 90 MB download size (v2: raised from
  60 MB to carry Foundations imagery; HEIC keeps it honest).
- NFR-4 Full A1+A2 packs with audio ≤ 250 MB on disk (Opus 24 kbps mono ≈ 180 KB/min;
  v3: includes ~35 KB HEIC cover per story under the I6 mandate, §8A.1).
- NFR-5 Zero third-party analytics/crash SDKs. os_log + MetricKit only.
- NFR-6 Accessibility: Dynamic Type through XXL in reader, VoiceOver labels on all
  controls, reduced-motion honored (seal-stamp animation becomes a fade).
- NFR-7 All UI copy localizable; source language English (UI), content language zh-Hans.

---

## 5A. Foundations — the zero-to-story visual bootstrap (new in v2)

The coverage gate has a cold-start problem: with zero known words there is no story
that can be 98% comprehensible. Foundations solves it by building the first ~300 words
through **picture-word comprehensible input** — real photographs, real audio, direct
meaning-to-form binding, minimal English — then handing off to the lattice. This is the
app's Rosetta-style layer, but with honest limits (§5A.4) and no AI imagery anywhere.

### 5A.1 Method & stages

- **F0 (words 1–60): picture-word cards.** [M14] Each card binds photograph + audio +
  hanzi + pinyin for one concrete, highly imageable word: 水 water, 狗 dog, 猫 cat,
  宝宝 baby, 人 person, 山 mountain, 火 fire, 鱼 fish, 米饭 rice, 茶 tea, 车 car…
  Interaction cycle per word: (a) *introduce* — photo shown, audio plays, character
  and tone-colored pinyin beneath; (b) *recognize* — hear the word, pick the matching
  photo among 4; (c) *read* — see the character alone, pick the photo; (d) *bind* —
  see the photo, pick the character among 4. Audio plays on every interaction
  (system TTS layer 2 for taps; factory audio for card intros).
- **F1 (words ~40+): picture sentences.** Function words that cannot be photographed
  (是, 的, 很, 不, 这, 那, 吗) are never taught as isolated cards; they are acquired
  inside minimal picture-anchored patterns where the *content* words carry the meaning:
  photo of a dog + 这是狗 → photo of a cat + 这是猫 → photo of a dog + 这不是猫.
  The pattern's meaning is inferable from the picture contrast (TPRS-style), keeping
  the direct-comprehension contract without English crutches.
- **F2 (words ~120+): picture micro-stories.** 3–6 page picture-book stories, one
  Commons photograph per page, 1–2 short sentences per page, built entirely from
  taught words + the current frontier (the coverage gate applies from here on, same
  I1 machinery, smaller lexicon). First seals are earned here.
- **F3 (words ~250–300): handoff.** The regular Today/lattice loop activates when the
  effective known set can gate at least 20 distinct A1 stories at ≥98%. Foundations
  remains browsable; its words feed the same KnownWordModel from day one — there is
  one model, not a separate flashcard silo.

### 5A.2 Word order & semantic sets

Sessions teach 6–8 words from one semantic set, then immediately recombine them in F1
patterns: animals, food & drink, family, numbers (photographed as countable objects,
e.g. 三个苹果), body, colors (color-field photos), home objects, places, weather,
actions (photographed mid-action: 跑, 吃, 喝, 睡觉). Selection order = HSK-3.0 level-1
membership × corpus frequency × **imageability rating** (concreteness norms; words
below the imageability floor are deferred to F1 patterns or to gloss-supported story
introduction later). Target inventory: ~220 photographed words + ~80 pattern-acquired
function/abstract words = 300 at handoff.

### 5A.3 New functional requirements

- FR-11.1 Every Foundations photograph is a real photograph or public-domain artwork
  sourced from Wikimedia Commons via the §8A pipeline. **No AI-generated imagery,
  anywhere in the product, ever** (this is a stated product value, like no-ads).
- FR-11.2 Each image ships with license + attribution metadata; long-press any image
  → attribution sheet (author, license, source link); a full Credits screen lists all
  images (CC-BY/CC-BY-SA legal requirement, and the right thing regardless).
- FR-11.3 Distractor photos in recognition steps are drawn from *already-taught* words
  only (never untaught confusables), and never from the same minimal-pair set twice
  in a row (狗/猫 contrast is deliberate; 四/十 audio contrast is scheduled, not random).
- FR-11.4 Foundations sessions are 5–8 minutes; a session ends with an F1/F2
  recombination pass, never with isolated cards (words must be met in syntax the same
  day they are introduced).
- FR-11.5 Foundations progress renders on the same Progress screen (words known
  counter counts from word #1); the CEFR card reads "Pre-A1 · Foundations" until handoff.
- FR-11.6 Placement can land a partial beginner *inside* Foundations (e.g., knows 120
  words) — the program starts at the first unmastered set rather than from zero.

### 5A.4 Honest limits (stated in-app on the methodology page)

Picture-based direct binding works for concrete vocabulary and simple copular/
demonstrative patterns; it degrades for abstraction. Zhuwen therefore allows a one-line
English gloss as a *fallback reveal* (behind a tap, never shown by default) rather than
pretending 也 or 就 can be photographed. This is a deliberate deviation from
Rosetta-style purism, documented with its rationale in the methodology page (I4).

## 6. Pedagogy spec (the numbers, with citations to registry)

- **Coverage threshold:** ≥ 98% running-token coverage for unassisted comprehension
  [Hu & Nation 2000; Nation 2006 — registry entries PED-001, PED-002]. The gate uses
  token coverage, not type coverage (a rare word repeated is one *type* but many tokens;
  tokens is what the reading experience feels like).
- **New-word budget:** ≤ 2% of tokens AND ≤ 8 new types per story; every new type occurs
  ≥ 3 times within the story (factory constraint) [incidental-acquisition frequency
  research — PED-003].
- **Re-encounter schedule:** frontier words scheduled for re-exposure in subsequent
  stories at expanding intervals (1d, 3d, 7d targets) via selector weighting; FSRS handles
  explicit review [PED-004].
- **Proper nouns:** excluded from coverage denominator when marked in pack data and
  glossed on first occurrence (standard extensive-reading practice).
- **i+1 definition (operational):** i = effective known set (FR-2.3); +1 = frontier words
  at the head of the queue (FR-2.4). Krashen's hypothesis is the framing; the operational
  claims we make in UI cite the coverage/frequency literature, not Krashen alone
  (the hypothesis itself is contested on production skills — we don't claim speaking gains).

---

## 6A. Source Canon — adapt the human corpus, don't invent ex novo (new in v2)

**Principle:** the LLM's job in the factory is *retelling under lexical constraint*,
not authorship. Every narrative story derives from a registered source in the human
corpus — myth, legend, fable, folk tale, or historical anecdote — with provenance
recorded. This raises the narrative floor (time-tested plots survive constrained
retelling far better than LLM inventions), delivers real cultural payload (learners
acquire China's actual story canon, not synthetic pablum), and adds a second
comprehensibility lever: **known-plot scaffolding** — a Western learner reading
《龟兔赛跑》(The Tortoise and the Hare) at A1 gets top-down support from already
knowing the story, on top of the 98% lexical guarantee.

### 6A.1 Canon tiers (initial registry, ~200 entries at launch)

| Tier | Source class | Examples | Primary refs |
|------|--------------|----------|--------------|
| C1 | Chengyu origin tales 成语故事 | 守株待兔, 拔苗助长, 画蛇添足, 塞翁失马, 井底之蛙, 刻舟求剑, 自相矛盾, 亡羊补牢, 对牛弹琴, 愚公移山 | zh.wikipedia + Wikisource classical originals |
| C2 | Chinese mythology & legends | 十二生肖 zodiac race, 年兽 Nian, 嫦娥奔月, 后羿射日, 夸父追日, 女娲补天, 牛郎织女, 白蛇传, 木兰辞 | Wikipedia (en+zh), 维基文库 |
| C3 | Classical novel episodes (PD) | 西游记 Monkey King arcs (三打白骨精, 大闹天宫), 三国演义 anecdotes (草船借箭, 空城计) | Wikisource originals + Wikipedia plot summaries |
| C4 | Historical anecdotes | 司马光砸缸, 曹冲称象, 孔融让梨, 卧薪尝胆 | zh.wikipedia |
| C5 | Aesop's fables | Tortoise & Hare, Boy Who Cried Wolf, North Wind & Sun, Fox & Grapes | Wikipedia + PD translations (Perry Index) |
| C6 | World folk & fairy tales (PD originals) | Grimm, Andersen, Arabian Nights, Panchatantra | Wikisource/Wikipedia |
| C7 | Nonfiction from the encyclopedia | Chinese festivals (春节, 中秋节), cities, food (饺子, 火锅), inventions, geography | Wikipedia articles as factual ground |

Chengyu tales (C1) are pedagogically privileged: each story *is* the etymology of an
idiom the learner will meet forever after; the story's seal face can carry the chengyu
itself. C7 covers the mundane-vocabulary gap (daily-life register) that myth cannot:
a survival-vocabulary story about buying vegetables is grounded in Wikipedia's factual
articles about Chinese markets/food rather than invented from nothing. **Residual
ex-novo quota:** ≤20% of the library, restricted to daily-life dialogic scenarios where
no canonical source fits, flagged `origin: original` in metadata.

### 6A.2 New functional requirements

- FR-12.1 Every story record carries provenance: canon ID, source URLs (Wikipedia/
  Wikisource revision permalinks), tier, and adaptation notes. The reader's story-info
  sheet shows "Adapted from: …" with a link-out; nonfiction (C7) shows its factual basis.
- FR-12.2 Legal rule enforced in the registry: adapt only public-domain source *texts*
  and traditional plots; never adapt an in-copyright retelling, translation, or film
  version (e.g., the Mulan *ballad* is PD; Disney's Mulan is radioactive). Registry
  entries require a `pd_rationale` field before the factory will accept them.
- FR-12.3 One canon entry may yield multiple retellings at different bands (龟兔赛跑
  at A1 in 200 words; at B1 in 600 with the fox subplot) — same canon ID, distinct
  story IDs; the selector never recommends two bands of the same canon entry within
  90 days.
- FR-12.4 **(upgraded to invariant I6 in v3)** Every story MUST carry a cover image
  from the human corpus via Wikimedia Commons: PD illustrations (e.g., Milo Winter's
  1919 Aesop plates, classical 西游记 woodcuts, PD nianhua 年画 prints), museum
  open-access scans, or Commons photographs — through the §8A pipeline with
  attribution (FR-11.2 applies). This is unconditional: a story without a compliant
  image cannot be packed, shipped, or displayed. Covers render on the Today card,
  library rows, story info sheet, and reader header. [M4][M11][M15]
- FR-12.5 Courses (FR-3.4) map onto the canon: "Twelve Animals" (zodiac, 12 chapters),
  "Journey West, Slowly" (Monkey King arc), "Thirty Chengyu" — each chapter steps the
  lexicon per the existing course mechanics.

### 6A.3 Factory changes (amends §8 pipeline)

Story briefs are no longer free prompts; the brief generator reads a canon entry and
emits: plot beats (from the source summary), named characters with fixed Chinese names,
cultural notes the retelling must preserve, the target lexicon slice, length band, and
the tier-appropriate register. The LLM prompt is "retell these beats within this
vocabulary," a materially easier constrained-generation task than "invent a story" —
expected to cut repair-loop iterations substantially. QA gains a **fidelity check**:
a second model pass verifies the retelling preserves the registered beats (no
hallucinated endings to 塞翁失马). All other gates (coverage, recurrence, grammar
whitelist) unchanged.

## 7. On-device TTS: findings & decision

**Findings (verified July 2026):**
- `AVSpeechSynthesizer` supports zh-CN on device (voice "Tingting" compact preinstalled).
  Enhanced/premium neural-ish voices exist since iOS 16 but **must be manually downloaded
  by the user in Settings → Accessibility (100+ MB each); apps cannot trigger or bundle
  that download.** Compact Tingting is intelligible but robotic; unacceptable as the
  primary listening experience for a paid product.
- The synthesizer's delegate reports word ranges as it speaks
  (`willSpeakRangeOfSpeechString`), giving free highlight sync for fallback mode.
- Pronunciation override via attributed-string IPA is undocumented/fragile for Mandarin;
  heteronym control (e.g., 得 de/dé/děi, 行 xíng/háng) is unreliable — another reason
  not to lean on system TTS for story audio.

**Decision (three-layer audio strategy):**
1. **Primary — factory audio.** Neural TTS rendered in the content factory (CosyVoice 2
  or Qwen-TTS class, mainland-standard 普通话, one female + one male voice), loudness-
  normalized, Opus 24 kbps mono, with **word-level timestamps from forced alignment**
  shipped in the pack manifest. Human-adjacent quality, correct heteronyms (factory
  pipeline verifies pinyin against segmentation), deterministic, offline after download.
2. **Tap-to-hear words/sentences — system TTS.** `AVSpeechSynthesizer` zh-CN, instant,
  no assets. Good enough for isolated word audio.
3. **Fallback story narration — system TTS** with delegate-driven highlight, used only
  when a story's audio pack isn't downloaded; UI labels it "System voice" and offers
  the pack download.

If the user has manually installed an enhanced zh-CN voice, the app detects and prefers
it for layers 2–3 (`AVSpeechSynthesisVoice.speechVoices()` quality filter).

### 7.1 Factory voice vendor decision (new in v2)

Requirement: free, license-clean for commercially distributed baked audio,
Mandarin-first quality, runnable locally in batch. Survey of the mid-2026 open field:

| Model | License | Mandarin fit | Verdict |
|-------|---------|-------------|---------|
| **CosyVoice 3.0** (Alibaba) | Apache 2.0 | Mandarin is its home language; 0.5B; best-in-class cross-lingual cloning; lineage reports 30–50% fewer pronunciation errors vs CosyVoice 1 | **PRIMARY** |
| **Qwen3-TTS** (Alibaba) | Apache 2.0 | 600M; Chinese among 10 languages; emotion control; early-2026 release | **ALTERNATE / A-B** |
| Chatterbox-Turbo (Resemble) | MIT | Chinese among 9 base languages; beat ElevenLabs 65.3/24.5 in a vendor-run blind test, but English-centric | bench only |
| Kokoro-82M | Apache 2.0 | efficiency champion; Mandarin not its strength; fixed voicepacks | no |
| MeloTTS | MIT | zh-en mixing, CPU-fast, older quality tier | no |
| Fish Speech / IndexTTS2 | complex/restricted | strong audio, documented licensing pitfalls for commercial use | **excluded on license** |
| F5-TTS | weights CC-BY-NC | — | excluded |
| XTTS-v2 | CPML non-commercial | — | excluded |

**Decision:** CosyVoice 3.0 as the factory render stage (Apple-silicon MPS locally or
rented GPU hours; latency irrelevant offline; cost ≈ electricity — meets the "free"
constraint). Two fixed house voices — "Xiaoyu" (female) and "Laohu" (male) — defined by
self-recorded or PD/consented reference prompts; never cloned from a real person without
consent. Qwen3-TTS wired as a config-switchable alternate for per-pack A/B listening
before ship. Every rendered utterance passes the existing checks: input pinyin (with
3-3 and 不/一 tone-sandhi applied) cross-verified against the lexicon, forced-alignment
anomaly detection, human audit sampling. Re-verify both model cards' license terms at
CP-09 — open-weight terms occasionally shift between releases.

---

## 8. Architecture

```
┌─────────────────────────── OFFLINE (factory, Go) ───────────────────────────┐
│ lexicon ingest (HSK-3.0 lists + freq)                                        │
│ canon registry (§6A: Wikipedia/Wikisource provenance) → beat-sheet briefs    │
│ Commons image pipeline (§8A: query → license filter → curate → attribute)    │
│  → LLM retelling (constrained by brief + lexicon slice)                      │
│  → segmentation (jieba/pkuseg + lexicon-aligned custom dict)                │
│  → COVERAGE GATE (token coverage vs target lexicon slice, new-type budget,  │
│     recurrence check, grammar-pattern whitelist per band)                   │
│  → repair loop (constrained rewrite prompts) → QA sampling (human audit %)  │
│  → question gen + gate → translation gen → TTS render → forced alignment    │
│  → pack build (SQLite + Opus + JSON manifest) → sign → CDN                  │
└──────────────────────────────────────────────────────────────────────────────┘
                                     │ anonymous HTTPS GET
┌──────────────────────────── ON DEVICE (Swift) ──────────────────────────────┐
│ PackClient → PackStore (verified, installed packs)                          │
│ LexiconStore (read-only, from packs)                                        │
│ KnownWordModel  ←── EventLog (append-only; lookups, grades, exposures)      │
│ FrontierQueue                                                               │
│ Selector: gate(bitmap AND) → score → Today feed / Library fit badges        │
│ Reader / Listener (AVAudioSession) / Review (FSRS) / Placement / Progress   │
│ SwiftData (learner state) + SQLite read-only attach (pack content)          │
│ Optional CloudKit private sync (learner state only, never content)          │
└──────────────────────────────────────────────────────────────────────────────┘
```

Factory details, gate algorithms, and pack format live in
`01-content-factory-spec.md` (next doc in this series); the app can be built against
fixture packs immediately.

---

## 8A. Commons image pipeline (new in v2)

Serves Foundations cards (FR-11) and story cover art (FR-12.4). Runs in the factory
(Go), with a human-curation TUI stage (Bubble Tea, parso-pdaudio pattern).

**Stages:**
1. **Query.** For each target word/canon entry, query the Wikimedia Commons API
   (`action=query&generator=search` + Wikidata P18 "image" claims for the concept's
   item — P18 images are usually the best single depiction of a concept). Pull top-N
   candidates with `imageinfo` including `extmetadata` (license, author, attribution).
2. **License filter (hard gate).** Accept only: Public Domain / CC0 / CC-BY / CC-BY-SA
   (any version). Reject: NC, ND, GFDL-only, "fair use", missing/ambiguous license
   metadata. Store the full attribution record (author, license name + URL, source
   permalink, retrieval date) in the pack DB — CC-BY/SA attribution is a legal
   obligation and ships in-app per FR-11.2. Note on SA: our crops/resizes are
   derivatives of the *image*, so each processed CC-BY-SA image is itself distributed
   under CC-BY-SA with attribution — acceptable for asset files; it does not infect
   app code. Prefer PD/CC0/CC-BY over SA when candidates tie.
3. **Automated quality gate.** Reject: resolution < 1200px on the short side,
   watermarks/visible text (OCR pass — text in an image leaks or contradicts answers,
   FR-11.3), collages, heavy compression, NSFW/graphic (safety pass), and — enforced
   by provenance, not vibes — anything Commons categorizes as AI-generated
   (`Category:AI-generated images` and descendants are excluded at query time; FR-11.1).
4. **Human curation TUI.** One keystroke per candidate: pick best-of-N per word,
   flag cultural mismatches (包子 must be baozi, not a generic bun; 早饭 should look
   like a Chinese breakfast), request re-query. Throughput target: ~150 words/hour;
   the whole Foundations inventory (~220 photographed words × pick-of-6) is roughly
   a two-evening curation job.
5. **Processing.** Square + 4:3 crops (attention-aware), sRGB, consistent tonal
   normalization, HEIC at two sizes (480px card, 1200px zoom); recognition-step
   distractor sets precomputed to satisfy FR-11.3 constraints.
6. **Manifest.** `image(id, word_id|canon_id, file, w, h, license, license_url,
   author, source_url, retrieved_at, curator_note)` — joined into packs.

### 8A.1 Per-story image mandate (I6): inventory math & reuse rules (v3)

- **Scale.** Launch library ≈ 1,240 stories. Reuse rules keep curation tractable:
  (a) all band-retellings of one canon entry share that entry's artwork (same canon ID
  → same image, FR-12.3); (b) C7/original daily-life stories may draw from a curated
  **topic pool** (~150 photographs keyed by topic: market, kitchen, train station,
  classroom…) with a uniqueness constraint of one use per topic-image per band;
  (c) course chapters may share one cover per course plus per-chapter variants when
  available. Net unique images needed at launch ≈ 200 canon + 150 topic + 220
  Foundations ≈ **570 curated images** — roughly five evenings in the curation TUI at
  the §8A throughput target.
- **Enforcement.** The pack builder joins `story → image` and hard-fails the build on
  any NULL, on any image lacking a full provenance record, or on any image whose
  Commons categorization intersects the AI-generated exclusion set (I6). There is no
  placeholder-art escape hatch; a story blocked on imagery simply doesn't ship.
- **Weight.** 480px HEIC covers average ~35 KB → ~45 MB across the full launch library,
  amortized into level packs (NFR-4 already accommodates this within the 250 MB A1+A2
  budget; recheck at CP-09 pack sizing).

**Story cover art (FR-12.4 / I6)** uses the same stages against curated Commons categories:
PD book illustrations (Milo Winter's 1919 Aesop plates for C5, classical woodcuts for
C3, 年画 New Year prints for C2), museum open-access scans, and photographs for C7
nonfiction. A per-tier style preference keeps shelves visually coherent
(illustration for tales, photography for nonfiction).

## 9. Data model (on-device)

**Pack SQLite (read-only, one file per pack):**
- `story(id, title_zh, title_en, band, hsk3_level, token_count, type_count,
  coverage_bitmap BLOB, new_type_ids JSON, topics JSON, grammar_ids JSON,
  audio_file, alignment JSON, body JSON)` — body is the segmented token stream:
  `[{w: word_id|literal, s: sentence_idx, pn: bool}]`
  plus v2 provenance: `canon_id, tier, source_urls JSON, origin ('canon'|'original'), cover_image_id`
- `question(id, story_id, prompt_zh, options JSON, answer_idx, band)`
- `image(id, word_id|canon_id, file, w, h, license, license_url, author, source_url,
  retrieved_at)` — §8A manifest (v2)
- `foundations_card(word_id, image_id, set_id, stage, distractor_ids JSON)` (v2)
- `sentence_translation(story_id, sentence_idx, en)`
- `lexicon(word_id, simp, pinyin, pinyin_tones JSON, gloss JSON, hsk3_level,
  freq_rank, char_ids JSON)`; `character(char_id, glyph, pinyin, gloss, components)`

**Learner store (SwiftData):**
- `WordState(wordID, state, pKnown, fsrsStability, fsrsDifficulty, due, exposures,
  lookups, firstSeen, lastSeen)`
- `Event(ts, kind, wordID?, storyID?, payload)` — append-only, source of truth;
  WordState is a rebuildable projection.
- `StoryProgress(storyID, position, completedAt?, sealEarned, listenedBlind)`
- `PlacementResult(ts, curveParams, cefrEstimate, hskEstimate, ci)`
- `Prefs`, `PackRecord(packID, version, installedAt, bytes)`

**Coverage math:** known set → 11k-bit bitmap (1.4 KB). Story coverage check =
`popcount(storyTypes & ~known)` against per-type token weights; NFR-2 met trivially.

---

## 10. Screen inventory & navigation

Tab bar: **Today · Library · Review · Progress** (+ Settings via Today toolbar).

| # | Screen | Mockup | Notes |
|---|--------|--------|-------|
| 1 | Welcome / privacy manifesto | M1 | No sign-in exists; states I2 plainly |
| 2 | Placement — word check | M2 | Rapid yes/no cards, progress dots |
| 3 | Placement — result | M3 | Knowledge curve, CEFR/HSK-3.0 estimate |
| 4 | Today | M4 | Daily story card w/ fit stats, frontier preview, calm log |
| 5 | Reader | M5 | Songti text, frontier underlines, toolbar |
| 6 | Word sheet | M6 | Gloss, tone-colored pinyin, char breakdown |
| 7 | Listening | M7 | Karaoke highlight, speed, blind toggle |
| 8 | Comprehension + seal | M8 | 3 questions → cinnabar stamp moment |
| 9 | Review (FSRS) | M9 | Sentence-context card |
| 10 | Progress | M10 | Band estimate, lexicon growth, coverage curve |
| 11 | Library | M11 | Lattice browser w/ personal fit badges |
| 12 | Paywall | M12 | Factual, 3 SKUs, trial |
| 13 | Settings & packs | M13 | Pinyin/audio prefs, pack manager, export/erase |
| 14 | Foundations card (v2) | M14 | Picture-word bootstrap, Commons photo + attribution |
| 15 | Story info / provenance (v2) | M15 | "Adapted from" canon source, PD cover art credits |

Design language: iOS-native (SF Pro UI chrome), story text in Songti SC;
accent = cinnabar red (#C3272B family) used *only* for meaning (frontier words,
seals, progress) — never decorative; signature element = the **seal stamp** (印章)
earned per completed story, which doubles as the app's progress vocabulary.
Liquid-Glass adoption note: same surgical policy as Cladiron — system chrome free,
content surfaces opaque.

---

## 11. Monetization & business (no ads, restated)

- Free/Pro split per FR-9; coach… (n/a here) — **the engine is the upsell**: free users
  get the loop but rationed volume; serious CI learners consume 3–10 stories/day.
- Lifetime is safe because marginal cost ≈ CDN bytes (I3 architecture).
- Optional non-subscription ladder (sovereignty ethos): per-band one-time pack purchases
  ($14.99/band) as an alternative to Pro — decide post-launch based on support burden.
- School/B2B licensing deferred; requires accounts, violates I2 — would be a separate
  SKU/app if ever pursued.

## 12. Success metrics (local-only analytics, user-visible)

Because there's no telemetry (NFR-5), success is measured via App Store metrics,
reviews, and opt-in feedback. In-app, the learner sees their own: days read,
words known, band trajectory. Business gates: 1k downloads/mo organic by M+3 post-launch,
trial→paid ≥ 8%, yr-1 target $2–4k MRR (honest range from prior analysis).

## 13. Risks & mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| LLM story quality at A1 (stilted, unnatural under vocab constraints) | High | Factory repair loop + human audit sample per pack; A1 relies more on scripted Foundations course; use Chinese-strong models (Qwen/DeepSeek) |
| Segmentation disagreements (word boundaries) corrupt coverage math | High | Single canonical segmenter + custom dict aligned to HSK-3.0 list, frozen per pack version; gate operates on factory segmentation only |
| Placement mis-seeds model → gate starves or floods | Med | Conservative seed + fast correction from lookups (I5); "too easy/too hard" feedback control on Today card |
| Factory TTS heteronym/tone errors | Med | Pipeline cross-checks TTS input pinyin vs lexicon; alignment anomaly detection; audit sample |
| HSK-3.0 list revisions post-launch | Med | Lexicon versioned in packs; model keyed by stable word IDs, not levels |
| Discovery/distribution (known structural weakness) | High | HSK-3.0 launch-window content marketing; CI community presence; not solvable in-product |
| Apple system TTS perceived as cheap if user hits fallback | Low | Label clearly, prompt pack download |
| Commons image licensing error (wrong/changed metadata) | Med | Hard license gate on extmetadata + stored permalinks + retrieval snapshots; audit sample per pack (v2) |
| Curated image is culturally wrong / ambiguous referent | Med | Human curation TUI stage with cultural flag; beginner-tester pass on Foundations (v2) |
| Canon retelling drifts from source (hallucinated beats) | Med | Fidelity check pass vs registered beat sheet; provenance shown in-app invites correction (v2) |
| Known-plot scaffolding fails for non-Western users on C5/C6 | Low | Tiers are metadata; selector can weight C1–C4 for any audience (v2) |

## 14. Build phases (agentic checkpoints)

1. **CP-01 Factory walking skeleton (Go):** lexicon ingest → canon registry with 10
   seed entries (§6A) → beat-sheet briefs → retell 20 A2 stories → segment → coverage
   gate → emit fixture pack. *The pipeline is the product.*
2. **CP-02 Pack format + fixture packs** (spec in `01-agentic-handoff.md`, golden
   files incl. an I6-violating fixture that must fail, signature scheme).
3. **CP-03 App skeleton:** tabs, PackStore, Reader against fixture pack (no model yet).
4. **CP-04 KnownWordModel + EventLog + Selector** with unit-tested gate (I1).
5. **CP-05 Placement flow** end-to-end seeding the model.
6. **CP-06 Listening** (factory audio + alignment; AVSpeech fallback).
7. **CP-07 Review (FSRS), Progress dashboard, comprehension/seal.**
8. **CP-08 Packs/CDN, StoreKit, paywall, settings, export/erase.**
8a. **CP-08a Commons image pipeline + curation TUI (§8A); Foundations program
   (F0–F3, FR-11) built against the curated inventory.**
9. **CP-09 Factory scale-up:** canon registry to ~200 entries, A1–B1 packs,
   CosyVoice 3.0 render (+Qwen3-TTS A/B), audit workflow, license re-verification.
10. **CP-10 Polish, accessibility pass, App Store assets, HSK-3.0 launch messaging.**

Open decisions before CP-01 (v2 status): ~~final name~~ → Zhuwen (§1.1);
~~voice vendor~~ → CosyVoice 3.0 / Qwen3-TTS (§7.1);
whether per-band one-time purchases ship at launch (recommend: no, add later).

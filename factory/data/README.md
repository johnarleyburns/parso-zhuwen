# factory/data — external data sources & provenance

This directory is where **operator-supplied, license-cleared** source data lives at build
time. Nothing copyrighted or license-unclear is committed to the repo (handoff §0.7).

## HSK-3.0 vocabulary list (for `zhuwenctl lexicon ingest`)

`zhuwenctl lexicon ingest --src <dir> --out lexicon.sqlite --version <lexicon_version>`
reads HSK-3.0 word-list files from `<dir>` and writes the frozen `lexicon.sqlite`
(stable integer word IDs — "word IDs are forever", §4.1 — plus `hsk3_level` and
`freq_rank` attributes).

### Expected input format
One TSV file (`.tsv`) per HSK level or a single combined file, tab-separated, `#` comments
and blank lines ignored:

```
# id  simp  pinyin  hsk3_level  freq_rank  [char_ids]
1     的    de      1           1
2     我    wǒ      1           2
...
```

`id` and `simp` must be unique and stable across versions. If `id`/`freq_rank` are not
present in the source, assign them deterministically (ingest order = rank) and record the
mapping so future versions keep IDs stable.

### Provenance & licensing — REQUIRED before committing any real list

The official HSK 3.0 standard 《国际中文教育中文水平等级标准》 (Ministry of Education /
CLEC, 2021) is a published government standard **without an explicit open-content licence**.
Its redistribution terms are unclear, so the raw list is **not committed** here — see
`plans/blockers.md` B-1. To add a real list, record in this file, per source file:

- exact source (publisher, title, edition/date, URL or ISBN),
- the licence or written permission under which it is redistributed,
- the retrieval date and a sha256 of the file.

If you cannot state a clear licence, **stop** and file/append to `plans/blockers.md` rather
than committing the file (§0.7).

### Tests use the fixture lexicon, not this data
The pipeline's tests run against the vendored fixture lexicon
(`internal/assets/lexicon.tsv`, `fixture-hsk3.0-v0`), so the factory builds and tests with
no external data present. The real `lexicon_version` ships as a new version once B-1 clears.

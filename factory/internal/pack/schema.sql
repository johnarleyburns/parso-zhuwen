-- content.sqlite schema (handoff §3, 00 §9). FROZEN at CP-02.
-- Changing this file requires bumping pack.SchemaVersion and updating the drift test.
-- cover_image_id is NOT NULL: invariant I6 enforced at the schema level.

CREATE TABLE meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE lexicon (
  word_id    INTEGER PRIMARY KEY,
  simp       TEXT NOT NULL,
  pinyin     TEXT NOT NULL,
  hsk3_level INTEGER NOT NULL,
  freq_rank  INTEGER NOT NULL,
  en         TEXT NOT NULL DEFAULT '',
  char_ids   TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE character (
  char_id    INTEGER PRIMARY KEY,
  glyph      TEXT NOT NULL,
  pinyin     TEXT,
  gloss      TEXT,
  components TEXT
);

CREATE TABLE image (
  id           TEXT PRIMARY KEY,
  word_id      INTEGER,
  canon_id     TEXT,
  file         TEXT NOT NULL,
  w            INTEGER NOT NULL,
  h            INTEGER NOT NULL,
  license      TEXT NOT NULL,
  license_url  TEXT NOT NULL,
  author       TEXT NOT NULL,
  source_url   TEXT NOT NULL,
  retrieved_at TEXT NOT NULL
);

CREATE TABLE story (
  id              TEXT PRIMARY KEY,
  title_zh        TEXT NOT NULL,
  title_en        TEXT,
  band            TEXT NOT NULL,
  hsk3_level      INTEGER NOT NULL,
  token_count     INTEGER NOT NULL,
  type_count      INTEGER NOT NULL,
  coverage_bitmap BLOB NOT NULL,
  new_type_ids    TEXT NOT NULL DEFAULT '[]',
  topics          TEXT NOT NULL DEFAULT '[]',
  grammar_ids     TEXT NOT NULL DEFAULT '[]',
  audio_file      TEXT,
  alignment       TEXT,
  body            TEXT NOT NULL,
  canon_id        TEXT,
  tier            TEXT,
  origin          TEXT NOT NULL,
  source_urls     TEXT NOT NULL DEFAULT '[]',
  pd_rationale    TEXT,
  cover_image_id  TEXT NOT NULL,
  fixture         INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE question (
  id         TEXT PRIMARY KEY,
  story_id   TEXT NOT NULL,
  prompt_zh  TEXT NOT NULL,
  options    TEXT NOT NULL,
  answer_idx INTEGER NOT NULL,
  band       TEXT NOT NULL
);

CREATE TABLE sentence_translation (
  story_id     TEXT NOT NULL,
  sentence_idx INTEGER NOT NULL,
  en           TEXT NOT NULL,
  PRIMARY KEY (story_id, sentence_idx)
);

CREATE TABLE citation (
  id        TEXT PRIMARY KEY,
  claim     TEXT NOT NULL,
  reference TEXT NOT NULL,
  doi       TEXT
);

CREATE TABLE alignment (
  story_id  TEXT NOT NULL,
  token_idx INTEGER NOT NULL,
  t0_ms     INTEGER NOT NULL,
  t1_ms     INTEGER NOT NULL,
  PRIMARY KEY (story_id, token_idx)
);

CREATE TABLE foundations_card (
  word_id        INTEGER PRIMARY KEY,
  image_id       TEXT NOT NULL,
  set_id         TEXT NOT NULL,
  stage          TEXT NOT NULL,
  distractor_ids TEXT NOT NULL DEFAULT '[]'
);

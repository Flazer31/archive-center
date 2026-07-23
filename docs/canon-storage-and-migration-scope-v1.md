# Canon Storage And Migration Scope v1

Status: Archive Center 3.1 storage contract, additive schema, and local installer lifecycle implemented  
Contract ID: `canon-storage-and-migration-scope.v1`  
Depends on: `canon-pack-manifest.v1`, `canon-identity-provenance-dedup.v1`

## 1. Purpose

This document fixes the minimum MariaDB ownership and additive migration scope
required before Archive Center implements local Canon Pack dry-run, install,
remove and rollback.

It protects these existing guarantees:

- current user-provided full text and approved reference data remain intact;
- Local Canon Overlay and pack-owned data can coexist without overwriting each
  other;
- one logical fact can retain evidence from several independent sources;
- pack removal cannot delete user data or another pack's evidence;
- ChromaDB remains a rebuildable index and cannot become installation truth;
- pack failure cannot block `/ready`, main memory save, delete or recall.

This document began as a scope decision. The approved additive schema has now
been implemented and live-gated; the importer and pack lifecycle runtime remain
pending.

## 2. Existing Structures To Reuse

| Existing structure | Reused responsibility | Required protection |
| --- | --- | --- |
| `reference_works` | Local work root and existing session binding target | Do not replace current `work_id` |
| `reference_continuities` | Local continuity and branch family | Preserve current IDs and parent links |
| `reference_documents` | User-local full text and current extraction source | Never turn pack metadata into public raw text |
| `reference_timeline_nodes` | Timeline and event anchors | Keep review and branch behavior |
| `reference_entities` | Local Canon entities | Do not merge by name alone |
| `reference_entity_aliases` | Entity surface aliases | Preserve ambiguous aliases |
| `reference_claims` | Derived assertions and existing recall fields | Preserve current rows and review state |
| `reference_claim_knowers` | Entity-scoped knowledge | Include in logical fact scope |
| `session_reference_bindings` | Explicit chat-to-work binding | Pack removal must not invalidate a bound work |
| `session_reference_runtime` | Session-local reference runtime | Never use as pack installation state |
| reference Chroma collection | Candidate acceleration | Rebuild only after MariaDB commit |

Existing tables remain authoritative for current 3.0 behavior. Version 1 does
not rename them, change their primary keys, remove columns, or reinterpret a
legacy `content_hash` as an exact-byte digest.

## 3. Why Existing Metadata JSON Is Insufficient

`metadata_json` and `provenance_json` remain useful for bounded extraction
details, but these relationships require foreign keys and independently
queryable lifecycle state:

- one portable work and edition mapped to one local work;
- one pack version and installation generation;
- several origins attached to one reference item;
- several source observations and evidence locators attached to one fact;
- several assertions grouped under one verified logical fact;
- a Local Canon Overlay action that survives pack update or removal.

Putting all of those into one JSON field would make removal, rollback,
deduplication and evidence preservation unverifiable.

## 4. Minimum New Storage

Names below are contract names. Exact SQL types and index lengths are fixed in
the implementation migration after disposable-MariaDB validation.

### 4.1 `reference_work_editions`

Maps portable work and edition identity to the existing local work root.

Required fields:

- local `edition_row_id` primary key;
- existing `work_id` foreign key;
- `stable_work_id`, `edition_id` and identity contract version;
- original language, edition language, label and status;
- edition metadata JSON for declared identifiers and scope note;
- revision and timestamps.

Required rules:

- unique `(stable_work_id, edition_id)`;
- `work_id` uses `ON DELETE RESTRICT` while an active pack install exists;
- legacy works may exist with no edition mapping until an explicit migration or
  identity resolution creates one;
- title equality never creates this row automatically.

### 4.2 `reference_work_titles`

Supports title, original title, translated title and alias search without using
the display title as identity.

Required fields:

- title row ID, `work_id`, optional `edition_row_id`;
- title kind, original text, language and script;
- normalization contract, normalized lookup key and its full SHA-256 digest;
- source observation ID when available;
- created timestamp.

Required rules:

- duplicate lookup keys for the same scoped title are suppressed by the full
  digest, not an index-prefix comparison;
- the same normalized key may point to several works or editions and therefore
  remains a search result set, not a unique identity constraint.

### 4.3 `canon_pack_installs`

Owns an immutable manifest generation and its lifecycle independently of the
Archive Center executable.

Required fields:

- `install_id`, pack ID, pack version and install generation;
- manifest contract, manifest SHA-256 and canonical manifest JSON;
- `work_id` and `edition_row_id`;
- pack status, review status and trust status;
- lifecycle status: `staged`, `active`, `inactive`, `failed`, `removed`;
- validation report JSON and coverage report JSON;
- installed, activated, removed and updated timestamps.

Required rules:

- unique `(pack_id, pack_version, install_generation)`;
- manifest JSON contains no private full text or secrets;
- only a fully validated `staged` generation can become `active`;
- at most one active generation of the same pack line and edition;
- install failure records diagnostics but does not change the previous active
  generation;
- removal is a lifecycle state before physical cleanup.

### 4.4 `reference_source_observations`

Preserves each pack, discovery or user-local observation even when exact bytes
are shared.

Required fields:

- observation ID, `work_id`, `edition_row_id` and continuity scope;
- origin kind and optional `install_id`;
- manifest source ID or local source key;
- source type, URI, license, access class and retrieval time;
- hash contract, exact document SHA-256 and optional existing `document_id`;
- provenance JSON and created timestamp.

Required rules:

- `document_id` is nullable because a public pack can carry digest and locator
  metadata without distributing source text;
- equal exact hashes may reuse local bytes but do not collapse observations;
- credentials, cookies, tokens and browser session values are forbidden;
- deleting or removing a pack deletes only observations exclusively owned by
  that install and never deletes a linked user-local `reference_document`.

### 4.5 `reference_item_origins`

Maps existing timeline, entity or claim rows to pack, discovery, legacy-local or
Local Canon Overlay origins.

Required fields:

- origin membership ID;
- `work_id`, `edition_row_id` and item kind;
- nullable `node_id`, `entity_id` and `claim_id` foreign keys with exactly one
  selected by a check constraint;
- origin kind, origin owner ID and optional `install_id`;
- source item ID, review state and timestamps.

Required rules:

- unique origin membership for one owner and source item;
- item kind is limited to production reference item kinds and must match the
  selected foreign-key column;
- pack removal deletes its memberships only;
- a reference item may be physically deleted only when it has no origin
  membership, no overlay dependency and no protected binding/runtime reference;
- legacy rows receive a versioned `legacy_local` origin through an idempotent
  reconciliation job, not a permanent query fallback.

### 4.6 `reference_item_evidence`

Provides the many-to-many evidence union missing from current claim rows.

Required fields:

- evidence edge ID and item kind;
- nullable `node_id`, `entity_id` and `claim_id` foreign keys with exactly one
  selected by a check constraint;
- source observation ID;
- document hash contract and hash;
- canonical locator JSON;
- evidence state and timestamps.

Required rules:

- unique `(selected item FK, observation ID, document hash, locator digest)`;
- evidence removal follows its origin, not a global source-priority winner;
- the existing `reference_claims.document_id` and `evidence_excerpt` remain for
  compatibility during the migration window but are not the future union
  owner;
- pack evidence contains locators, not redistributed full text.

### 4.7 `reference_logical_facts`

Owns the stable retrieval group that can contain exact or verified-equivalent
claim expressions.

Required fields:

- logical fact ID primary key;
- `work_id`, `edition_row_id` and continuity/applicability scope digest;
- logical fact status and revision;
- created and updated timestamps.

Required rules:

- logical facts do not contain source evidence or private full text;
- different edition or applicability scope cannot share a logical fact;
- a logical fact remains while any claim identity, origin or overlay rule
  references it;
- conflict links do not silently replace the logical facts they relate.

### 4.8 `reference_fact_identities`

Maps claim rows to exact fingerprints and verified logical facts.

Required fields:

- `claim_id` primary/foreign key;
- fingerprint contract and exact fingerprint;
- `logical_fact_id` foreign key;
- equivalence status and basis;
- edition, continuity and applicability scope digest;
- revision and timestamps.

Required rules:

- one exact fingerprint identifies one reusable assertion row;
- unique `(fingerprint contract, exact fingerprint)` rejects incompatible
  duplicate assertion rows;
- additional exact origins and evidence join that row instead of overwriting it;
- differently worded assertions may share a logical fact ID only after a
  `verified_fact_equivalent` decision;
- similarity alone cannot set equivalence;
- ID or fingerprint collision with incompatible payload rejects the write.

### 4.9 `reference_overlay_rules`

Preserves user-local choices without mutating installed pack rows.

Required fields:

- overlay rule ID, `work_id`, `edition_row_id`;
- nullable target `logical_fact_id`, `entity_id` or `node_id` foreign keys with
  exactly one target selected while active;
- action: `supplement`, `override`, `suppress_for_retrieval`, `conflict`;
- local replacement item when applicable;
- status, reason, revision and timestamps.

Required rules:

- rules are local user data and never enter a public pack;
- pack update, removal and rollback do not delete them;
- a missing target after pack removal becomes dormant, not deleted;
- target foreign keys use `ON DELETE RESTRICT`; the service marks the rule
  dormant and clears its target before cleanup, and never cascades deletion
  into local rules;
- the check constraint permits zero targets only when status is `dormant`;
- explicit overlay behavior affects retrieval without erasing pack provenance.

### 4.10 Foreign-key deletion policy

| Child relationship | Delete policy | Reason |
| --- | --- | --- |
| edition to existing work | `RESTRICT` | A work cannot disappear while portable identity is installed |
| pack install to work/edition | `RESTRICT` | Deactivate and clean the install explicitly |
| source observation to install | `RESTRICT` | Remove evidence and origin membership in one service transaction first |
| item origin to selected timeline/entity/claim FK | `CASCADE` only when the item itself is explicitly eligible for cleanup | No dangling origin row |
| evidence to selected item and source observation | `RESTRICT` during normal lifecycle | Origin-aware service cleanup proves no surviving evidence is lost |
| fact identity to claim | `CASCADE` after claim cleanup eligibility is proven | Identity cannot outlive its assertion |
| fact identity to logical fact | `RESTRICT` | Logical group remains while assertions exist |
| overlay to work/edition | `RESTRICT` | Pack lifecycle cannot delete local overlay |
| overlay target | `SET NULL` after marking dormant | Preserve local intent for later pack reinstall |

The implementation may use separate tables instead of nullable target columns
only if it preserves the same foreign-key guarantees. An unenforced generic
`item_kind + item_id` string pair is not sufficient.

## 5. Deliberately Deferred Storage

The first installer migration does not add these without a reproduced runtime
requirement:

- Registry cache and publisher-revocation tables;
- granular Source Discovery query, fetch-observation and coverage-delta tables beyond the
  implemented isolated `source_discovery_jobs` JSON ledger;
- structured conflict-review queue and issue-item join tables;
- separate document-blob storage outside existing private documents;
- graph-wide entity rewrite or cross-work entity identity;
- RisuAI Host Context, lorebook or narrative-consistency tables;
- new Chroma collections or vector truth tables;
- generic plugin or executable payload tables.

Manifest conflict, uncertainty and coverage reports remain in the immutable
install snapshot and diagnostics response. They must not be admitted as approved
facts. Source Discovery exceptions and pending candidates currently remain in
the isolated job ledger; structured issue tables are deferred until individual
issues require independent mutation.

## 6. Existing Table Changes

Version 1 prefers new relationship tables over rewriting current tables. The
only existing-column change allowed in the first implementation proposal is an
optional nullable exact-byte digest pair on `reference_documents`:

- `source_hash_contract`;
- `source_bytes_sha256`.

These columns may be omitted from the first installer if public pack sources
use `reference_source_observations` and user-local exact-byte migration is not
yet implemented. Existing `content_hash` must remain unchanged and must be
reported as the legacy trimmed-text contract until a source is re-observed.

No existing value is silently relabeled or backfilled with fabricated exact
bytes.

## 7. Additive Migration Boundary

The migration must be additive:

- create new tables, indexes and nullable columns only;
- do not drop, rename or narrow existing columns or indexes;
- do not change current primary keys or foreign-key delete actions;
- do not rewrite existing user raw text, review state or IDs;
- do not create pack installs from legacy data automatically;
- make every schema statement idempotent for a dry-run and retry;
- stop before mutation if required tables, collation, engine or foreign-key
  prerequisites are incompatible.

Legacy-origin reconciliation is a separate, idempotent data job with preview,
counts and stable failure rows. It runs only after schema success. It must not
change review status or Chroma data. Its compatibility window ends when every
existing reference item has a durable origin membership and the release ledger
records that condition.

## 8. Package Upgrade And Rollback

The current updater v1 rejects migration and `mariadb-schema` changes. Therefore
this schema must not be shipped as an automatic lightweight update.

Until an updater v2 additive-migration and DB rollback contract is implemented
and tested, the release uses a safe full-package upgrade with:

1. schema and data dry-run;
2. user database backup or verified recovery point;
3. additive schema application;
4. schema/version and existing-data verification;
5. backend startup and `/ready` verification;
6. explicit operator-visible failure without deleting existing data.

Application rollback does not drop the new additive tables. An older binary
must ignore them while continuing to read existing 3.0 tables. Pack installation
rollback is separate: the previous active generation remains active until the
new generation commits.

## 9. Installer Transaction Boundary

The local v1 installer follows `preview -> validate -> install`:

### Preview and validate

No DB mutation is allowed while validating:

- manifest and contract version;
- Archive Center/schema compatibility;
- archive allowlist, paths, member types and safety limits;
- file sizes and SHA-256 checksums;
- work, edition, continuity and source identity;
- exact document identity and source-reference integrity;
- conflict, uncertainty and coverage reports;
- proposed row counts and affected existing local data.

### Install

One MariaDB transaction creates the install, work/edition/title mappings, source
observations, approved reference items, item origins, and evidence edges. It
becomes active only after all required rows validate. Failure rolls back the DB
write and leaves the previous generation unchanged.

Logical fact identity, evidence union across exact duplicate facts, Local Overlay
resolution, and Chroma lifecycle indexing remain later runtime work. The local
installer must not claim those behaviors merely because their additive tables
already exist.

Chroma indexing occurs after MariaDB commit. Chroma failure marks reference
indexing degraded and retryable; it does not roll back or delete committed
MariaDB Canon data and does not block main memory.

### Remove

Removal deactivates the install, removes its retrieval eligibility and schedules
origin-aware cleanup. It does not delete:

- user full text;
- Local Canon Overlay or overlay rules;
- evidence from another origin;
- a logical fact still owned by another origin;
- session bindings or main long-term memory.

## 10. LLM And Embedding Boundary

The following operations are deterministic and require no LLM or embedding
configuration:

- JSON Schema and manifest validation;
- compatibility, archive and checksum validation;
- exact-byte document hashing;
- stable identity lookup and exact duplicate detection;
- evidence-edge union for exact duplicates;
- local pack preview, install, deactivate, remove and rollback;
- migration dry-run and existing-data preservation checks.

LLM use is optional or later-stage for:

- extracting structured candidates from user documents;
- Source Discovery query expansion and source-document extraction;
- proposing entity resolution or near-duplicate equivalence;
- interpreting ambiguous coverage and conflict candidates.

An LLM answer alone cannot approve, merge or delete Canon facts. Normal
`evidence_validated_batch` processing must verify source hash, locator, edition,
scope, deduplication and conflict conditions. Ambiguous results remain
exceptions.

Embedding is required only for vector indexing and semantic retrieval. Exact
install and MariaDB browse continue without it. Missing critic or embedding
configuration produces an isolated original-DB warning, not a `/ready` or main
memory failure.

The current RisuAI adapter already sends the existing auxiliary/critic LLM and
embedding settings in `client_meta` for reference extraction and vector work.
Do not add a duplicate original-DB LLM settings surface. Future internet Source
Discovery additionally needs a separate search/fetch provider and allowed-source
contract; an LLM configuration by itself does not provide web access.

## 11. Validation And Test Gates

### Contract and migration tests

- schema dry-run against a clean disposable MariaDB;
- schema dry-run against a copy shaped like an existing 3.0 database;
- repeated migration is idempotent;
- pre/post counts and hashes of existing reference rows are identical;
- invalid prerequisite fails before mutation;
- legacy-origin reconciliation preview and retry are deterministic;
- `.env`, DB files, raw user text and secrets are absent from package assets.

### Installer tests without LLM

- valid neutral manifest preview, install, browse, deactivate and remove;
- invalid schema, checksum, compatibility and archive path rejected before DB
  write;
- same document from two origins preserves two source observations;
- exact fact duplicate creates one fact with multiple evidence edges;
- verified equivalent expressions inject once while retaining assertions;
- scope-distinct and conflicting facts remain separate;
- pack removal preserves user source, overlay and other-origin evidence;
- Chroma unavailable leaves MariaDB install successful and retryable;
- pack failure does not affect `/ready`, main save, delete or recall.

### Live test entry

The user-facing test begins only after local pack preview, install, remove and
browse APIs are implemented and the disposable-MariaDB gates pass. That first
test uses the neutral fictional fixture and requires no LLM credentials. LLM,
search-provider and embedding tests are separate later gates for extraction,
Source Discovery and semantic recall.

## 12. Completion And Next Work

The storage range decision, additive schema, importer, pack lifecycle, local
Registry, Local Overlay resolver and bounded Source Discovery runtime are
implemented. Full source profiles, mirror/equivalence classification, automatic
batch admission and final RisuAI retrieval/injection E2E remain pending.

The DB-write-free local pack validator remains `POST /canon-packs/preview/v1`.
Approved packs are installed separately through `POST /canon-packs/install/v1`
in MariaDB authority mode.

The additive migration exists as `migrations/002_canon_pack_storage.sql`. It
creates the approved relationship/lifecycle tables plus the isolated
`source_discovery_jobs` ledger. After the live
gate described below, the same ordered statements are registered in
`001_schema.sql` for fresh installs and in the production `mariadb-schema`
compatibility pass for existing databases.

Local tests load the proposal through the production `mariadb-schema` SQL
loader, reject non-`CREATE TABLE IF NOT EXISTS` statements, verify the required
tables/checks/delete policies, and execute the statement list twice with
sqlmock. This proves loader compatibility and the intended retry shape. The
live gate below supplies MariaDB syntax and idempotency evidence.

`.github/workflows/canon-pack-storage-migration.yml` supplies the repeatable
live gate with an empty disposable MariaDB 11.4 service. One gated integration
test creates the existing reference prerequisites, seeds neutral pre-migration
rows, applies `002` twice through `applyStatements`, verifies all ten tables,
checks the pre-existing row fingerprint, and exercises RESTRICT, exactly-one
target, active-overlay target, and one-active-generation constraints. A second
test applies the actual production `001 + compatibility` path to a separate
disposable database, seeds neutral rows, reapplies it, and checks table presence
and row preservation. Both tests refuse to run without an explicit disposable
flag and dedicated test database prefix.

An equivalent local gate ran an official-checksum MariaDB 11.4.12 Windows
portable instance from `%TEMP%`. The first live attempts exposed unsupported
string/nullable generated-column expressions and a conflicting overlay
`SET NULL`/`CHECK` policy. The migration was corrected to use a nullable numeric
active marker, an explicit checked edition scope, target `RESTRICT`, and
target-specific evidence unique keys. The final run applied `002` twice,
preserved the seeded pre-migration row fingerprint, and passed all constraint
checks.

That evidence permits production registration for full 3.1 package installs
and upgrades. It does not permit updater v1 to apply schema changes, and it
does not implement pack install/remove APIs. Remote GitHub Actions has not yet
run and remains the repeatable CI gate.

# Canon Identity, Provenance, And Dedup Contract v1

Status: Archive Center 3.1 exact-fact identity and dedup runtime implemented; mirror/equivalence expansion pending  
Contract ID: `canon-identity-provenance-dedup.v1`  
Depends on: `canon-pack-manifest.v1`

Storage and migration scope: `canon-storage-and-migration-scope.v1`

## 1. Purpose

This contract defines how Archive Center identifies a work and edition, keeps
source provenance, detects duplicate documents and Canon facts, preserves
conflicts, and avoids repeated search injection when data comes from all of
these paths:

- an installed Canon Pack;
- Source Discovery;
- the user's Private Full-Text Source Library;
- the user's Local Canon Overlay.

The product goal is not to replace user-provided material. Archive Center must
combine reusable public data and legally held local data without asking the user
to find or clean every source by hand. Exact duplicates are collapsed for
storage and retrieval, while independent supporting sources remain attached as
separate evidence.

This contract does not define pack installation tables, a database migration,
Registry governance, a crawler, semantic similarity thresholds, or RisuAI
Host Context and lorebook behavior.

## 2. Required User Outcome

The normal flow is:

1. The user searches for a work or adds a local document.
2. Archive Center resolves a work and edition candidate without merging by
   title alone.
3. Installed pack data, discovered sources, and local material enter the same
   provenance and duplicate analysis.
4. Exact duplicate documents and facts do not create repeated Canon injection.
5. Independent evidence is retained even when it supports an existing fact.
6. Edition, continuity, branch, time, knowledge-scope differences and actual
   contradictions are preserved.
7. Only unresolved identity, conflict, uncertainty, or insufficient-source
   exceptions require user attention.

There is no per-item approval requirement for every normal Canon fact.

## 3. Identifier Classes

The following identifiers are different contracts and must never be silently
substituted for one another.

| Identifier | Meaning | Merge authority |
| --- | --- | --- |
| `stable_work_id` | Namespaced work identity from `work.stable_id` | Registry or explicit identity resolution |
| `edition_id` | Namespaced edition identity | Exact identifier or explicit edition resolution |
| `continuity_id` | Namespaced continuity or branch family | Exact identifier or explicit mapping |
| local `work_id` / row ID | MariaDB installation-local identity | Database only |
| pack item ID | Identity within one pack line and generation | That pack manifest only |
| document digest | Identity of exact source bytes | Hash algorithm contract |
| fact fingerprint | Identity of one scoped derived Canon assertion | Backend canonicalizer contract |

The current random MariaDB `work_id` is not a portable work identity. A future
install mapping must associate it with `stable_work_id` and `edition_id`
without rewriting existing user rows.

## 4. Text Lookup Normalization

`canon_text_lookup_key.v1` is used only to find candidates. It is not sufficient
authority to merge works, editions, entities, or facts.

The canonicalizer must:

1. reject invalid Unicode input;
2. normalize to Unicode NFC;
3. apply Unicode default case folding;
4. trim leading and trailing Unicode whitespace;
5. collapse internal Unicode whitespace runs to one ASCII space;
6. preserve punctuation, diacritics, script and word order.

Transliteration, translated-title matching, punctuation removal, fuzzy search
and model similarity may add identity candidates but may not create identity.
The original text, language tag and script remain stored alongside the lookup
key. Normalizer changes require a new version and must not rewrite IDs in place.

## 5. Work, Edition, And Continuity Resolution

### 5.1 Work resolution

An exact `stable_work_id` match identifies the same work. Title, translated
title and alias matches only produce `work_identity_candidate` results.

Two records must not be merged automatically when any of these differ or are
unresolved:

- publisher or external catalog identity;
- work type;
- original language or original title scope;
- adaptation, remake, sequel, compilation, or similarly distinct work scope.

### 5.2 Edition resolution

An edition is scoped by `edition_id`. ISBN, publisher catalog ID, platform
release ID and equivalent declared identifiers are supporting evidence. The
label or publication date alone is not an identity key.

Different translations, revisions, regional releases, adaptations and edited
editions remain distinct unless an explicit mapping says they share an edition
scope. When edition identity is missing, data stays `edition_unresolved` and
cannot silently enter another edition's approved fact set.

### 5.3 Continuity resolution

`continuity_id` and branch scope are part of fact applicability. Similar
continuity labels do not merge continuities. A parent mapping may express a
relationship but does not erase the child scope.

## 6. Source And Document Provenance

### 6.1 Source observation

A source observation records the manifest source ID, source type, URI, license,
access class, retrieval time and retrieved document digest. `source_uri` is a
locator, not document identity. The same URI may change, and different URIs may
serve identical bytes.

URI normalization may be used for refetch candidates: lowercase scheme and
host, remove a default port and URI fragment, and resolve dot segments. Query
parameters and their order are preserved because they can select different
resources. Credentials, cookies and tokens must never enter a normalized URI.

### 6.2 Exact document digest

`source_bytes_sha256.v1` is lowercase SHA-256 over the exact retrieved or
uploaded file bytes before trimming, newline conversion, Unicode normalization,
decoding or extraction.

For text pasted directly through an API with no original file, the source bytes
are the UTF-8 bytes of the received string before content trimming. The source
kind must state that the bytes came from pasted text.

A derived normalized-text digest may be stored as a duplicate candidate signal,
but it never replaces the exact byte digest.

### 6.3 Blob deduplication and provenance union

Equal `source_bytes_sha256.v1` values may share one immutable document blob.
Every retrieval, pack, Registry publisher and user-local origin remains a
separate source observation linked to that blob. Finding an existing blob must
append or reuse the provenance edge; it must not discard the new source.

Evidence identity is the tuple:

```text
(source observation ID, document hash algorithm, document hash, canonical locator)
```

Evidence locators follow `canon-pack-manifest.v1`. Two evidence edges are exact
duplicates only when the full tuple is equal.

Private full text remains local. A public pack may reference its digest and
locator without containing the source body.

## 7. Entity And Alias Identity

Pack-local entity IDs remain stable within their pack line. A future local
resolver maps them to installation-local Canon entity IDs.

An authoritative external entity identifier or an already verified mapping can
establish identity. Otherwise `(entity kind, canon_text_lookup_key.v1(name))`
and alias matches produce only `entity_identity_candidate` results.

The following are not safe automatic merges:

- two characters with the same name;
- a title or role shared by multiple entities;
- the same name in different editions or continuities;
- aliases that point to multiple entities;
- translated names without an identified source entity.

An alias keeps its original text, normalized lookup key, language, source and
target entity. An alias key that points to more than one viable entity is an
`alias_conflict`, not a winner selected by source priority or model confidence.

## 8. Canon Fact Identity

### 8.1 Exact fact fingerprint

`canon_fact_fingerprint.v1` is SHA-256 over canonical JSON containing all of:

- `stable_work_id` and `edition_id`;
- sorted continuity IDs;
- claim type;
- resolved subject and object entity IDs, when present;
- `canon_text_lookup_key.v1` of the bounded derived assertion;
- branch key;
- temporal scope and valid-from/valid-to anchors;
- reveal-from anchor;
- knowledge scope and sorted knower entity IDs.

The canonical JSON uses UTF-8, lexicographically sorted object keys, no
insignificant whitespace and sorted arrays only where the field is defined as a
set. Evidence, source priority, review time and wording display preferences are
excluded so that independent evidence can join the same fact.

If any required scope is unresolved, the item is not eligible for exact
cross-source merge. It remains an unresolved candidate.

### 8.2 Exact duplicates

Facts with the same fingerprint are one logical fact. The system must:

- retain every distinct evidence edge;
- retain origin membership for pack, discovery and local overlay;
- preserve review and trust information per origin;
- avoid creating a second logical fact or a second injection item.

It must not replace public evidence merely because a local source exists, or
replace local evidence when a pack update arrives.

### 8.3 Near duplicates

Semantic similarity, translated paraphrases and model-proposed equivalence can
create a `near_duplicate_candidate` cluster only. They must not automatically
delete, overwrite, approve or merge facts. No fixed similarity threshold is
defined by this contract.

A candidate may become `verified_fact_equivalent` without per-item user review
only when the backend records a versioned equivalence decision showing that the
structured subject, predicate or claim type, value, edition, continuity,
branch, temporal, reveal and knowledge scopes are the same and the evidence
supports that mapping. Similarity or a model answer alone is not an equivalence
basis. The decision keeps every original assertion and evidence edge while
assigning one logical fact ID for retrieval.

Normal evidence-validated equivalents may be handled in a batch. Ambiguous
translation, unresolved entity mapping and differing scope remain exceptions.

### 8.4 Conflict candidates

A versioned structured fact slot may group assertions that address the same
subject/property and scope. Different assertion values within that slot create
a conflict candidate. A model-provided `fact_key` is diagnostic input, not a
trusted identity key until the backend validates or derives it.

Differences in edition, continuity, branch, valid time, reveal time or knowledge
scope are normally `scope_distinct`, not contradictions. True contradictions
remain separate facts linked by an explicit conflict record. No source-priority
list silently deletes a side of a conflict.

## 9. Local Canon Overlay

User-provided documents enter the Private Full-Text Source Library and their
derived approved facts enter Local Canon Overlay. They participate in the same
document and fact fingerprints as pack and discovery data.

An exact local duplicate adds local provenance to the logical fact. A local
correction does not mutate the installed pack record. It creates an explicit
overlay relationship such as supplement, override, suppress-for-retrieval, or
conflict. The exact storage representation is deferred to the next schema-gap
stage.

An explicit user override may control local retrieval, but the underlying pack
fact and evidence remain available for inspection and rollback.

## 10. Search And Injection Deduplication

Search may return several source rows for one logical fact. Before budget and
injection, the backend groups applicable results by the verified logical fact
ID when present, otherwise by the exact fingerprint:

```text
(verified logical fact ID or canon_fact_fingerprint.v1,
 active edition, continuity, branch, time,
 reveal ceiling, knowledge scope)
```

One representative assertion is injected per group, with provenance summarized
from all applicable evidence. This is retrieval grouping, not destructive DB
deduplication.

Conflict and uncertainty groups are not reduced to a hidden winner. Existing
spoiler, branch, time, knower and main-memory priority rules continue to apply.
JavaScript receives the backend result and does not calculate identity or
duplicate policy.

## 11. Stable Outcomes

Backend validation and future APIs must use stable outcome codes rather than a
single score:

| Code | Meaning | Mutation |
| --- | --- | --- |
| `exact_document_duplicate` | Exact source bytes already exist | Reuse blob; preserve new provenance |
| `exact_fact_duplicate` | Same scoped fact already exists | Union evidence/origin; no second fact |
| `work_identity_candidate` | Title or alias suggests a work | No merge |
| `edition_unresolved` | Edition cannot be proven | Isolate candidate |
| `entity_identity_candidate` | Name or alias suggests an entity | No merge |
| `alias_conflict` | Alias resolves to multiple entities | Preserve exception |
| `near_duplicate_candidate` | Semantic equivalence is possible | No automatic merge |
| `verified_fact_equivalent` | Structured value and every applicability scope match with evidence | Assign one logical fact ID; preserve assertions and evidence |
| `scope_distinct` | Similar assertion has a different scope | Preserve both |
| `canon_conflict_preserved` | Same scoped slot has incompatible values | Preserve both and conflict edge |
| `identity_payload_collision` | Same stable ID carries incompatible identity data | Reject mutation |
| `hash_contract_mismatch` | Hash algorithm or byte semantics differ | Do not claim an exact duplicate |

These outcomes do not rank overall pack quality and are not completion scores.

## 12. Current Runtime Mapping And Gaps

### 12.1 Reusable production behavior

- `ReferenceWork`, `ReferenceContinuity`, `ReferenceDocument`, entity, alias,
  claim and timeline records already separate the main reference domains.
- `reference_documents.content_hash` and the unique
  `(work_id, continuity_id, content_hash)` key already prevent some repeated
  local document imports.
- `referenceStableID` provides deterministic local IDs for continuity,
  document, timeline, entity and claim paths.
- `referenceGroundedSourceExcerpt` and candidate metadata already distinguish
  evidence found in the imported source chunk.
- current diagnostics detect exact normalized entity/claim duplicates and
  fact-key conflict candidates.
- current recall already reapplies review, branch, temporal, reveal and knower
  filters after vector retrieval.

### 12.2 Gaps that must not be reported as implemented

- `reference_works.work_id` is a random local UUID and has no stable-work or
  edition mapping.
- uploaded content is currently trimmed before SHA-256, so its hash is not
  `source_bytes_sha256.v1`.
- returning an existing document for the same hash does not preserve a second
  independent source observation.
- current entity IDs derive mainly from lowercased names; homonyms, language
  and alias conflicts do not have a durable resolution contract.
- current claim IDs omit edition, branch, temporal, reveal, knowledge and
  knower scopes. A pending upsert can overwrite document/evidence metadata for
  a colliding ID.
- one claim row points to one `document_id`; independent evidence cannot yet be
  represented as a durable many-to-many evidence union.
- current library ViewModel deduplication selects one duplicate row, preferring a
  `user_` review source, but does not union evidence or origin membership.
- model-provided `fact_key` is stored in metadata and is not a backend-derived,
  versioned identity key.
- pack generation, source observation, fact identity, origin membership and
  Local Canon Overlay relationships now have additive MariaDB storage owners.
  Runtime writes, resolution policy and retrieval suppression behavior remain
  unimplemented.

## 13. Neutral Examples

### 13.1 Exact fact with additional evidence

The fictional pack states that surveyor Mira belongs to Harbor Survey. A user
adds a separate licensed ledger expressing the same membership with different
wording. After the backend verifies the same entities, membership value and all
applicability scopes, one logical fact retains both original assertions and two
evidence edges and is injected once.

### 13.2 Similar text with different scope

One fictional edition states that the North Beacon is open before an event; a
revised edition states it is closed after that event. The text is related, but
edition and temporal scopes differ. The system preserves both as
`scope_distinct`.

### 13.3 Alias ambiguity

Two fictional people are both called “Keeper”. The shared alias creates an
`alias_conflict`; it does not merge the people or select one by popularity.

## 14. Completion And Next Work

The contract and its additive relationship storage are complete. Identity
resolution, provenance union, deduplication and overlay policy are not runtime
complete.

The completed next-stage scope is recorded in
`canon-storage-and-migration-scope-v1.md` for:

- stable work/edition mapping;
- immutable source blobs and multiple source observations;
- fact identity and many-to-many evidence edges;
- pack install generation and origin membership;
- Local Canon Overlay relationships;
- durable conflict, uncertainty and coverage records.

The reviewed additive migration is implemented. No importer API or further
schema expansion is authorized merely by this contract.

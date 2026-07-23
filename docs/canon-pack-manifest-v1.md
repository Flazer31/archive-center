# Canon Pack Manifest v1 Contract

Status: design draft for Archive Center 3.1  
Contract ID: `canon-pack-manifest.v1`  
Schema: `canon-pack-manifest-v1.schema.json`

Related identity and duplicate policy:
`canon-identity-provenance-dedup-v1.md`

Related storage and migration scope:
`canon-storage-and-migration-scope-v1.md`

## Purpose

This contract describes the minimum portable metadata used to identify,
inspect, validate, and locally install a Canon Pack. It follows the product
flow in which a user selects a work and
edition, then Archive Center finds and installs a compatible reviewed pack.

The manifest is provenance, review, compatibility, and package-integrity data.
It is not a container for a copyrighted full-text source library, user chats,
RisuAI Host Context, long-term memory, credentials, or executable behavior.

## Package Layout

The archive root contains exactly one `canon-pack-manifest.json`. Every other
regular file must appear once in `files`, and every `files[].path` must be a
normalized relative path permitted by the schema. The manifest itself is not
listed because a file cannot contain its own stable checksum.

Version 1 permits only these optional ancillary members:

- `notices/LICENSE.txt`
- `notices/NOTICE.txt`
- `signatures/manifest.sig`

All Canon records are serialized in the manifest `content` object. Importers
must reject unlisted files, duplicate paths, absolute paths, backslashes,
parent traversal, symlinks, hard links, device files, executables, and archive
members outside the allowlist. Importers must also apply bounded compressed
size, expanded size, member count, and compression-ratio limits before
extraction. Those operational limits are security limits, not completion
targets or required entity counts.

Each listed file uses SHA-256 over its exact bytes. Validation must reject a
missing file, an extra file, a size mismatch, a checksum mismatch, or a changed
manifest. A signature, when present, signs the canonicalized manifest bytes;
it does not replace checksum validation. Executable signing such as SignPath
is unrelated to Canon Pack publisher trust.

## Identity And Compatibility

- `work.stable_id` identifies the creative work independently of title or
  translation. Its namespace must be stable and documented by the publisher.
- `work.edition.edition_id` and `continuity_ids` identify edition and branch
  scope. Title matching alone must never silently merge editions.
- `pack.id` identifies the publisher's pack line; `pack.version` is SemVer.
- `compatibility.archive_center` states the supported Archive Center interval.
- `compatibility.schema` pins this manifest contract and schema version.

The schema validates syntax, not ownership of a namespace, semantic-version
ordering, publisher identity, signature authenticity, or whether two IDs refer
to the same real-world work. A future validator owns those checks.

## Review And Admission

`production` records how the pack was made. `review` records the pack-level
review procedure and result. Machine-assisted production is allowed, but a
machine recommendation alone cannot set `review.status` to `approved`.
Approved packs require an identified review procedure, review time, and an
admission basis of `human_review`, `trusted_pack_review`, or
`evidence_validated_batch`. Human and trusted-pack review require an identified
reviewer. `evidence_validated_batch` is an automatic admission after source
hash, evidence locator, edition scope, deduplication, and conflict checks pass;
it is not per-item user approval. Items that fail those checks remain explicit
`conflict` or `uncertain` exceptions instead of entering the admitted batch.

Every Canon record carries its own `review_state` and one or more evidence
locators. `conflict` and `uncertain` states are preserved rather than resolved
by a hidden winner. Installation admission policy remains backend-owned and is
not defined as an API by this document.

## Sources And Evidence

A source entry records source type, URI, license, access class, retrieval time,
and document SHA-256. It contains no source body. Evidence locators reference a
declared source and repeat the document hash so that evidence cannot silently
move to a revised document. Locators identify a page, chapter, section,
paragraph, fragment, timestamp, or structured record without embedding an
excerpt.

`document_sha256` uses the exact-byte `source_bytes_sha256.v1` semantics defined
by `canon-identity-provenance-dedup.v1`.

Source URIs are limited to HTTP(S) or URN identifiers and must not contain URI
userinfo, credentials, tokens, cookies, or other secret query parameters.

Derived summaries and claim statements are bounded metadata. They must not be
used to reconstruct or substitute for a copyrighted work. Private full text
stays in the user's local Private Full-Text Source Library.

## Coverage

Coverage is reported per named domain as `covered`, `partial`, `insufficient`,
or `not_assessed`, with missing topics and notes where relevant. Saturation is
reported separately. There is no universal score, fixed record count, or fixed
number of entities, locations, factions, settings, events, relations, or
claims that makes a pack complete.

## Trust States

- `unsigned_local`: locally created or imported without publisher trust.
- `trusted_publisher`: publisher identity and pack signature were validated by
  a separately approved trust policy.
- `invalid_signature`: a claimed signature failed validation and the pack must
  not be admitted.

Registry governance, trust roots, revocation, API implementation, database
migrations, and runtime behavior are outside this bounded data contract.

## Current Preview Validator

Archive Center now exposes the preparatory, DB-write-free endpoint
`POST /canon-packs/preview/v1`. It accepts an `application/zip` body and returns
`canon-pack-preview.v1` with validation profile
`canon-pack-manifest.v1-install-preview-subset`.

The validator reads the archive in memory without extraction. It enforces the
root manifest, bounded compressed and expanded sizes, member-count and
compression-ratio limits, normalized unique paths, regular non-executable
members, duplicate-JSON-key rejection, required install-preview fields,
Archive Center compatibility, evidence/source hashes and references, and the
allowlisted ancillary-file checksums.

The profile name is deliberate: this is not a general Draft 2020-12 JSON
Schema engine and does not claim complete parity with every schema keyword.
Successful preview means that the implemented install-preview security and
semantic checks passed. It does not verify publisher signatures, grant trust,
install data, mutate MariaDB or Chroma, or call an LLM, embedding provider or
search provider. Until a cryptographic trust verifier exists, a manifest that
claims `trusted_publisher` is rejected as unverifiable and `invalid_signature`
is always rejected. `unsigned_local` remains the only trust state that can pass
this preview profile.

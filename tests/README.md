# Tests

Current public version: **4.5.0**. Tests exercise the runtime source, configuration,
turn lifecycle, memory retrieval/delivery, storage and installation/update contracts.
Internal test-run exports and user session data are not distributed with this tree.

Use local Git commits/tags and verified backup bundles for prior-version regression
comparisons. Keep historical reports and development commits local; do not
republish old history to perform a comparison. Public releases include reviewed
current source and required synthetic tests only.

| Scope | Location |
| --- | --- |
| Go memory, routes, provider transport and storage | Production-owner tests in go-service/internal/ |
| Host adapter, HUD and JavaScript routes | [js-route-variant-smoke](../go-service/cmd/js-route-variant-smoke/) |
| Cold-start Host fixture | [worldline-coldstart-probe.cjs](fixtures/worldline-coldstart-probe.cjs) |
| Settings browser checks | [preprocessing-ui-smoke.cjs](../ops/preprocessing-ui-smoke.cjs) |
| Source, DB, vector and platform CI | [.github/workflows](../.github/workflows/) |

From the source root:

    node --check "Archive Center.js"

From go-service:

    go test ./... -count=1

For a Host-adapter change:

    go test ./cmd/js-route-variant-smoke -count=1

Use the affected owner's tests first. Compare candidate arrival and actual delivery
separately from model prose. Include basic memory, Publisher only, preprocessing only
and both, plus empty/partial/failed supplementary responses when relevant.
Source tests, controlled fixtures, package identity and actual RisuAI/provider behavior
are separate evidence. No runtime or user data belongs in this directory.

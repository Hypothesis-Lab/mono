# Make stateless integration tests run in parallel

## Objective
Split each `test/suites/<suite>/` package into `StatefulSuite` and `StatelessSuite` so that read-only / non-mutating tests can run concurrently via `t.Parallel()`, while DB-mutating tests keep the per-test reset from `harness.IntegrationSuite`.

## Done
- `test/suites/audience/`
  - Added `suites.go` with `StatefulSuite` and `StatelessSuite` structs + constructors.
  - Renamed existing `Suite` receiver methods to `*StatefulSuite`.
  - Not-found / bad-UUID tests (`TestGetAudienceNotFound`, `TestGetAudienceBadUUID`, `TestCreateProductAudienceProductNotFound`, `TestListProductAudiencesProductNotFound`, `TestUpdateAudienceNotFound`, `TestDeleteAudienceNotFound`) kept in `negative_tests.go` as `*StatefulSuite` methods — they register a user (a DB write), so per the "registers a user = stateful" rule they cannot run parallel-safe.
- `test/suites/file/`
  - Added `suites.go` with `StatefulSuite` and `StatelessSuite` structs + constructors.
  - Renamed existing `Suite` receiver methods to `*StatefulSuite`.
  - All current file tests mutate DB/S3 state, so no tests were moved to `StatelessSuite`.
- `test/suites/hypothesis/`
  - Added `suites.go` with `StatefulSuite` and `StatelessSuite` structs + constructors.
  - Renamed existing `Suite` receiver methods to `*StatefulSuite`.
  - All current hypothesis tests require a registered user or mutate DB/S3 state, so no tests were moved to `StatelessSuite`.
- `test/suites/product/`
  - Added `suites.go` with `StatefulSuite` and `StatelessSuite` structs + constructors.
  - Renamed existing `Suite` receiver methods to `*StatefulSuite`.
  - All current product tests require a registered user (every test authenticates) or mutate DB/S3 state, so no tests were moved to `StatelessSuite`.
- `test/suites_test.go`: suite labels `AuthStateful` / `AuthStateless`, `AudienceStateful`, `FileStateful`, `HypothesisStateful`, `HTTPStateless`, `ProductStateful`; `FileStateful` uses `file.NewStatefulSuite()`, `HypothesisStateful` uses `hypothesis.NewStatefulSuite()`, `ProductStateful` uses `product.NewStatefulSuite()`.
- `go vet`, `task lint`, `task test:unit`, and full `go test ./test/...` all pass.

## Notes
- Truly stateless = no authenticated request needed (e.g. `TestEndpointsRequireAuth`, `TestNonExistentRouteReturns404`). Any test that registers a user OR mutates DB/S3 state is stateful — registering a user writes to the `users` table, and a concurrent `IntegrationSuite` reset (or another suite's `SetupSuite` insert colliding on the same email) would break it. Do NOT parallelize user-registering tests.
- Keep test functions in their existing files (e.g. `tests.go`, `negative_tests.go`); do not extract them into a separate `stateless.go` file. Just change the receiver to `*StatelessSuite` and add `t.Parallel()` where appropriate.
- If a test registers a user, then it's stateful.

## Remaining
(none — every suite now uses the StatefulSuite/StatelessSuite split, and only genuinely stateless tests are parallelized.)

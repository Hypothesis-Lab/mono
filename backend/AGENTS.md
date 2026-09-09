when installing packages, don't guess their versions, install latest

after making changes, run linter, tests, and API generation (USE commands from Taskfile.yml if possible, e.g. when not running a specific test) (docker socket is available, so you can and should run integration tests)

when running go test or linter commands, filter out 'go: downloading'

make sure that members that don't really need to be exported are not exported

handlers shouldn't leak service layer error messages to users (in responses)

after altering protected routes set, add them to test/suites/auth/unauthenticated.go

no unnecessary comments in go code

if added new tables, add them to test/harness/db.go to be cleared in tests

prefer no defaults unless really makes sense

ALL errors in handlers should be logged; if it's an operation on an object, its id should be included

# Unit tests
- assert exact returned fields in returned structs, e.g. "expecting only {A, B} in this struct", this is to catch field drift (something was deleted or added, a test should catch that)
- assert full error strings (if error is not very complicated), not just presence of an error

# Error messages
- all error messages will be logged at some point
- this is not about http respones, this is about go errors
- thus all error messages should be distinct, in order to find the exact place in code where that error occured
- 2 different code paths shouldn't return the exact same error; as one solution, they should annotate that error

# Style guide
## Ordering
```
imports
consts
vars 
enums
type aliases
structs relating to "class" A
"class" struct A
structs relating to "class" B
"class" struct B
constructors
public methods
private methods
public functions
private functions
```

vars and consts should be grouped unless have reason to be ungrouped

# Repo map

Standard `cmd/` + `internal/` Go layout, wired with uber/fx dependency injection (see `cmd/app/main.go`).

- `cmd/app/main.go` — entrypoint; wires fx providers and invokes `*http.Server` (and transitively `*db.DB`) to force construction of lifecycle-managed types.
- `internal/config/` — env-based config (`Config`: Port, DatabaseURL, JWTSecret, S3Endpoint, S3Region, S3Bucket, S3AccessKeyID, S3SecretAccessKey). All values are required and read from env vars with no defaults.
- `internal/db/` — `schema.sql` (embedded), `DB` struct, and `NewDB` (registers lifecycle hook that creates `*pgxpool.Pool`, applies schema, and seeds on start; closes pool on stop).
- `test/` — integration tests. They spin up Postgres, SeaweedFS, and the app container via `testcontainers-go`, and reset the database and S3 bucket after each test case via the harness.
- `internal/logger/` — zap logger + fx module.
- `internal/repositories/` — DB access; raw structs (no JSON tags) returned to the API layer.
- `internal/services/` — business logic (e.g. `UserAuthService` for registration/login).
- `internal/api/` — OpenAPI 3 spec + oapi-codegen generated strict-server `StrictServerInterface` / `ServerInterface` / models (`gen.go`, generated; do not edit).
- `internal/handlers/` — `handlers.Server`, the `api.StrictServerInterface` implementation; wrapped via `handlers.NewServerInterface` for the router.
- `internal/http/` — chi router, HTTP server lifecycle, and auth middleware.

Stack: chi, pgx/v5, zap, fx, oapi-codegen. Flow: repo (db structs) -> `handlers.Server` (generated response structs) -> json.


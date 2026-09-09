# Rules for writing tests

don't mutate env.Client

## databases reset in integration tests
databases reset before each test case, so when seeding users email should be the same in all new test cases if possible (having different emails is a smell), this applies to other seeded entities

```
func (s *IntegrationSuite) SetupTest() {
	resetIntegrationState(s.T())
}

func (s *IntegrationSuite) SetupSubTest() {
	resetIntegrationState(s.T())
}
```

for subtests s.Run should be used instead of s.T().Run

## Assert full structs, not field by field

bad:
```
assert.Require(t, resp.A, 1)
assert.Require(t, resp.B, 1)
```

good:
```
assert.Require(t, resp, Resp{A: 1, B: 2})
```


## Avoid adding helpers
Prefer inline, only add helpers if there is a good reason and they're well thought out

## Other
assert exact returned fields in responses, e.g. "expecting only {A, B} in this response", this is to catch field drift (something was deleted or added, a test should catch that)

assert full response bodies when possible, using jsoneq

tests that test routes that modify db contents should verify db contents (use RowAsStrings from harness/dbrow.go)

when testing non-create API methods that require existing instances in db, create that instance via db, nov via create-API

one test should test one endpoint, not multiple

some of the useful available helpers:
func RegisterUser(t *testing.T, env *Env, email, password string) string
func UserIDFromToken(t *testing.T, token string) string

## Negative tests
negative tests test only ownership: it should test that the route does not return and does not act upon objects that do not belong to the authenticated user

all routes should have ownership tests

validation tests are not ownership tests, for example

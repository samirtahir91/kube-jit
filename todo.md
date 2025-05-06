Todo:
- refactor login for providers as duplicated in get profile
- refactor k8s.go to use structs, include groupName in struct

stretch
- refresh token?
  - max session env var option
  - deprecate tokenExpiry in frontend

test
  unit
  bdd
  e2e Selenium or cypress or other

  mock oauth?
  bypass oauth?

  e2e/integration 
  real login, grab cookie, b64 string input value in github workflow?

Todo:
- time in email notification is mizzing zone, sending utc - basicall need a timzrone env var, and api needs to convert timezone in all messages
  - same with contorller
  - time in controller is displayed as utc
- test all providers
- refactor login for providers as duplicated in get profile
- refactor k8s.go to use structs, include groupName in struct

stretch
- refresh token
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

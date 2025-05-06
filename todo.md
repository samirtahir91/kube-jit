todo
- ui                 (filters.approverNames === '' || historicalRequest.approverNames.some(name => name.toLowerCase().includes(filters.approverNames.toLowerCase()))) &&
  - handle null approverNames list

- ui card needs to fill width, maybe some padding on bigger screens
- ui request end date needs to be on top of footer
- time in email is mizzing zone, sending utc
- time in controller is displayed as utc
- fix bug controller startup race errors
- test all providers
- refactor login for providers as duplicated in get profile
- refactor k8s.go to use structs, include groupName in struct
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

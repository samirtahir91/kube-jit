# kube-jit-gh-teams
A open source application to implement self-service access with Kubernetes RBAC, integrated with GitHub Teams.

## ToDo
- gke workload identity cluster type
- cache clientsets api per cluster
## cwa
- admin tab for searching all records
  - locked to admin teams or local admin?
- Add azure and Google sso options
- use similar cookie for all?

## api
- optimise db queries and indexing
- ddos rate limit protection
  - add pgBouncer if using more than 1 replica
  - use env vars for db and default db con pool if env var pg bouncer true
- db connection pool limit and timeout


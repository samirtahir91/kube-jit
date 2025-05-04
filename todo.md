todo

- add self approve toggle env var
- clean expired non approved requests from db
- add env var option for cookie samesite attribute - for strict or lax
- json logging?
- log important errors
- remove and debut logging 
- add log for new jit, approved, rejected - context of requestData
- set types to either gke or aks in helm for downstream clusters
- azure scope minimise
- https
- domain on email input preconfigured
- email notifications on status request approve reject and end
- after request validate namespaces before submitting
- get approvers from annotations of ns, if not found select platform group to approve 
- fix │ [GIN-debug] [WARNING] You trusted all proxies, this is NOT safe. We recommend you │
- upload request ooption


test
  unit
  bdd
  e2e Selenium 

## Sprint:
- nice alert on history and approver pages
- add to history search records approved by you.
- nicer looking approve/reject button
- increase select check box area in approve/reject
- try to cache clusters on startup and jitgroups (but dont fail)
- email notifications to requestors (depends if they have email configured in profile)
- env var for db debug mode


## Priority:
- it should show all approvers and for the namespace they approved in history and pending.
- for multi approver, get all approvers not just last - change approvaers names and ids to string and update
- if no group found admin only or allowed approvers can approve
- copy changes from azure permissions to google and github
  - make it common function to call
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

Poc option to toggle annotation ns approval.
- Approver.kube-jit.io/group_id = GROUP ID
- on request api Iterate through namespaces and get approvers.
- get groups
- add multi approver to request
- if multi show pending approvers
- each group will need to approve
- think about logic in record here
- requires cluster role on downstream clusters to get ns
- nice alert on history and approver pages
- add history of approved reqs for approvers
- nicer looking approve/reject button
- env var for db debug mode

  - it should show all approvers and for the namespace they approved in history
- for multi approver, get all approvers not just last
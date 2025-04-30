todo

- seperate cookie for groups and is approvers
- cookie split logicif cookie is bigger than 4k

- add self approve toggle env var

- poc option to toggle annotation ns approval.
- Approver.kube-jit.io/group_id = GROUP ID
- on request api Iterate through namespaces and get approvers.
- get groups
- add multi approver to request
- if multi show pending approvers
- each group will need to approve
- think about logic in record here
- requires cluster role on downstream clusters to get ns

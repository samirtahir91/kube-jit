## Required secrets
The deployment spec will load the secrets for cookie and HMAC encryption, DB and Github App credentials into ENV vars.

Create a secret in the same namespace, it must be named `kube-jit-api-secrets`

You need to define keys as below, and the values for them:
- dbUser
- dbPassword
- hmac-secret
- cookie-secret
- ghAppClientSecret

i.e.
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kube-jit-api-secrets
  namespace: kube-jit-api
type: Opaque
data:
  dbUser: cG9zdGdyZXMK
  dbPassword: bXlzZWNyZXRwYXNzd29yZAo=
  hmac-secret: bXlzZWNyZXRwYXNzd29yZAo=
  cookie-secret: bXlzZWNyZXRwYXNzd29yZAo=
  ghAppClientSecret: bXlzZWNyZXRwYXNzd29yZAo=
```
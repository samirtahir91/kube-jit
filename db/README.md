```sh
docker run --name postgres -e POSTGRES_PASSWORD=mysecretpassword -d -p 5432:5432 -v postgres-data:/var/lib/postgresql/data postgres

# The table is created and auto migrated by GORM in Go Gin API
```

Helm
```sh
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm install my-postgres bitnami/postgresql

export POSTGRES_PASSWORD=$(kubectl get secret --namespace default my-postgres-postgresql -o jsonpath="{.data.postgres-password}" | base64 -d)
```
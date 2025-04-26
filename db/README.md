```sh
docker run --name postgres -e POSTGRES_PASSWORD=mysecretpassword -d -p 5432:5432 -v postgres-data:/var/lib/postgresql/data postgres

# The table is created and auto migrated by GORM in Go Gin API
```

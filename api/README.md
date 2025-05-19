# kube-jit-api

### Open API Docs
For full API docs you can run the api and get the v2 or v3 docs

Authenticated routes use http cookies, you can copy them from a logged in browser session if wanting to experiment/debug stuff with curl/cmdline.

- v2 - <exposed-host/url>/kube-jit-api/swagger/index.html#
- v3 - <exposed-host/url>/kube-jit-api/swagger-ui/

### Logging and Debugging
- By default, logs are JSON formatted, and log level is set to info and error.
- Set environment variable `DEBUG_LOG` to `true` for API debug level logs.
- Set environment variable `DB_DEBUG` to `true` for degub level GORM logs (DB operations)

### Unit Testing
```sh
make test
```

### Build the API
```sh
make build
```

### Run the API
If running locally, you will need to export env variables as required.
```sh
export SMTP_PORT=xxx
export SMTP_HOST=xxx
export SMTP_FROM=xxx
export OAUTH_REDIRECT_URI=xxx
export ALLOWED_DOMAIN=xxx
export OAUTH_CLIENT_SECRET=xxx
export OAUTH_CLIENT_ID=xxx
export OAUTH_PROVIDER=xxx
export API_NAMESPACE=xxx
export HMAC_SECRET=xxx 

# run
make run
```

**Generate coverage html report:**
```sh
go tool cover -html=cover.out -o coverage.html
```

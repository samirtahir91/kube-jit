# kube-jit-api

### Open API Docs
For full API docs you can run the api and get the v2 or v3 docs

API docs need to be regenerated as per [guide](./kube-jit/docs/README.md)

Authenticated routes use http cookies, you can copy them from a logged in browser session if wanting to experiment/debug stuff with curl/cmdline.

- v2 - <exposed-host/url>/kube-jit-api/swagger/index.html#
- v3 - <exposed-host/url>/kube-jit-api/swagger-ui/

### Logging and Debugging
- By default, logs are JSON formatted, and log level is set to info and error.
- Set environment variable `DEBUG_LOG` to `true` for API debug level logs.
- Set environment variable `DB_DEBUG` to `true` for degub level GORM logs (DB operations)


### Prerequisites
- go version v1.23.0+
- docker version 17.03+.

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

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

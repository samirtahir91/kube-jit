# OpenAPI 3 Spec Generation for This Project

This project uses [swaggo/swag](https://github.com/swaggo/swag) to generate a Swagger 2.0 (OpenAPI 2.0) spec from Go code comments.  
To use OpenAPI 3.x features or tooling, you must first generate the Swagger 2.0 spec, then convert it to OpenAPI 3.x.

## Steps

### 1. Generate Swagger 2.0 Spec with swaggo

From the project root, run:

```sh
swag init -g cmd/main.go
```

This will generate `docs/swagger.json` (Swagger 2.0 format).

---

### 2. Convert Swagger 2.0 to OpenAPI 3.x with Node.js (swagger2openapi)

```sh
npm install -g swagger2openapi
swagger2openapi docs/swagger.json -o docs/openapi3.yaml
```

---

### 3. Use the OpenAPI 3 Spec

- The converted file will be at `docs/openapi3.yaml`.
- Serve this file in your API or use it with Swagger UI, Redoc, or other OpenAPI 3.x tools.

---

**Note:**  
Every time you update your Go API comments, repeat these steps to keep your OpenAPI 3 spec up to date.

---

## API Documentation URLs

- **Swagger 2.0 (v2) UI:** [yourdomain]/kube-jit-api/swagger/index.html#
- **OpenAPI 3.x UI:** [yourdomain]/kube-jit-api/swagger-ui/
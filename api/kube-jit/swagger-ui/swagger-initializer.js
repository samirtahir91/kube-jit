window.onload = function() {
  window.ui = SwaggerUIBundle({
    url: window.location.origin + "/kube-jit-api/docs/openapi3.yaml",
    dom_id: '#swagger-ui'
  });
};

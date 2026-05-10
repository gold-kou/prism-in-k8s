package registry

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const buildContextFileMode fs.FileMode = 0o600

const dockerfile = `FROM stoplight/prism:5.15.10
COPY openapi.yaml /app/openapi.yaml
COPY openapi-sample.yaml /app/openapi-sample.yaml
COPY empty_check_and_copy.sh /app/empty_check_and_copy.sh
RUN chmod +x /app/empty_check_and_copy.sh && /app/empty_check_and_copy.sh
CMD ["mock", "-h", "0.0.0.0", "-p", "80", "-m", "false", "/app/openapi.yaml"]
`

const openapiSample = `# https://swagger.io/docs/specification/v3_0/basic-structure/
openapi: 3.0.0
info:
  title: Sample API
  description: Optional multiline or single-line description in CommonMark or HTML.
  version: 0.1.9

servers:
  - url: http://api.example.com/v1
    description: Optional server description, e.g. Main (production) server
  - url: http://staging-api.example.com
    description: Optional server description, e.g. Internal staging server for testing

paths:
  /users:
    get:
      summary: Returns a list of users.
      description: Optional extended description in CommonMark or HTML.
      responses:
        "200":
          description: A JSON array of user names
          content:
            application/json:
              schema:
                type: array
                items:
                  type: string
`

const emptyCheckScript = `#!/bin/sh

if [ ! -s /app/openapi.yaml ]; then
  echo "openapi.yaml is empty or does not exist. Copying openapi-sample.yaml to openapi.yaml."
  cp /app/openapi-sample.yaml /app/openapi.yaml
else
  echo "openapi.yaml is not empty."
fi
`

func prepareBuildContext(openapiPath string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "prism-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	absOpenAPI, err := filepath.Abs(openapiPath)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("failed to resolve openapi path: %w", err)
	}

	openapiContent, err := os.ReadFile(absOpenAPI)
	if err != nil {
		cleanup()
		return "", fmt.Errorf("failed to read openapi file %s: %w", absOpenAPI, err)
	}

	files := map[string][]byte{
		"Dockerfile":              []byte(dockerfile),
		"openapi.yaml":            openapiContent,
		"openapi-sample.yaml":     []byte(openapiSample),
		"empty_check_and_copy.sh": []byte(emptyCheckScript),
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), content, buildContextFileMode); err != nil {
			cleanup()
			return "", fmt.Errorf("failed to write %s: %w", name, err)
		}
	}

	return tmpDir, nil
}

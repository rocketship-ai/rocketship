## This directory contains all of the Dockerfiles needed to run Rocketship at scale.

### 🐳 Docker Quick Start

```bash
# Pull the image
docker pull rocketshipai/rocketship:latest

# Run a test suite by mounting a directory containing your test to the container
# Use TEST_FILE or TEST_DIR to specify the rocketship.yaml file or directory
docker run -v "$(pwd)/examples:/tests" \
  -e TEST_FILE=/tests/simple-http/rocketship.yaml \
  rocketshipai/rocketship:latest
```

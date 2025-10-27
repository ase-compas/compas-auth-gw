# GitHub Actions Workflows

This repository uses GitHub Actions for continuous integration and deployment.

## Workflows

### Build and Push (`build-and-push.yml`)

Triggered on:
- Push to `main` or `develop` branches
- Pull requests to `main` or `develop` branches
- Tags matching `v*.*.*` pattern

**Jobs:**
1. **Build**: Builds Docker image and pushes to GitHub Container Registry (ghcr.io)
   - On PRs: Only builds the image (no push)
   - On push/tags: Builds and pushes to ghcr.io
   
2. **Test**: Runs Go unit tests with race detection and coverage
   - Uploads coverage to Codecov (on non-PR events)
   
3. **Lint**: Runs golangci-lint for code quality checks

**Image Tags:**
- `main` branch → `latest` tag
- `develop` branch → `develop` tag
- Pull requests → `pr-<number>` tag
- Git tags → Version tags (e.g., `v1.2.3`, `1.2`, `1`)
- Commits → `<branch>-<sha>` tag

### Release (`release.yml`)

Triggered on:
- GitHub release creation

**Job:**
- Builds multi-platform Docker images (linux/amd64, linux/arm64)
- Pushes to both Docker Hub and GitHub Container Registry
- Creates version tags (e.g., `1.2.3`, `1.2`, `1`, `latest`)

## Docker Registries

### GitHub Container Registry (ghcr.io)

Images are automatically pushed to ghcr.io using the GitHub token:

```bash
docker pull ghcr.io/ase-compas/compas-auth-gw:latest
docker pull ghcr.io/ase-compas/compas-auth-gw:v1.0.0
```

### Docker Hub

Release images are pushed to Docker Hub (requires secrets configuration):

```bash
docker pull <username>/compas-auth-gw:latest
docker pull <username>/compas-auth-gw:v1.0.0
```

## Required Secrets

Configure these secrets in your repository settings:

### For Docker Hub Publishing
- `DOCKER_HUB_USERNAME`: Your Docker Hub username
- `DOCKER_HUB_TOKEN`: Docker Hub access token (create at https://hub.docker.com/settings/security)

### For Codecov (Optional)
- `CODECOV_TOKEN`: Codecov project token (optional, works without for public repos)

## Usage Examples

### Pull Latest Development Image

```bash
docker pull ghcr.io/ase-compas/compas-auth-gw:develop
```

### Pull Specific Version

```bash
docker pull ghcr.io/ase-compas/compas-auth-gw:v1.2.3
```

### Run from GitHub Container Registry

```bash
docker run -d \
  --name compas-auth-gw \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  ghcr.io/ase-compas/compas-auth-gw:latest
```

## Local Testing

Test the Docker build locally:

```bash
# Build only
docker build -t compas-auth-gw:test .

# Build with buildx (multi-platform)
docker buildx build --platform linux/amd64,linux/arm64 -t compas-auth-gw:test .
```

## Workflow Status

Check the status of workflows at:
- https://github.com/ase-compas/compas-auth-gw/actions

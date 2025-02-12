# Release Process

This document describes the process for releasing a new version of the project.

## Docs
The docs are automatically published whenever a PR updates the docs and the PR is merged into the main branch. The docs are published to the `gh-pages` branch, which is the source for the Github Pages site.

## Docker images
The Docker image latest tag always points to the latest released version.
The `main` tag points to the latest commit on the main branch.

If you push a tag `vX.Y.Z` to the repository, the Docker image with the tag `vX.Y.Z`
is built and pushed to Docker Hub. Afterwards, the `latest` tag is updated to point to the new version.

Use the commands below to create a new release.

Check the latest version tag:

```sh
git pull --tags
git tag -l | grep -E "^v[0-9]" | sort -t "." -k1,1n -k2,2n -k3,3n
```

This may show `v0.14.0`. If that's the case then tag the next version with
`v0.15.0` by running:

```sh
git checkout main
git pull origin main
git tag v0.15.0
git push --tags
```

You can go to the GitHub action page and see that a new action has been triggered. This action to build and publish needs to succeed.

Afterwards you can pull the new image:

```sh
docker pull substratusai/kubeai:v0.15.0
```

## Helm Chart
The Helm chart only gets released when a git tag is pushed to the repository with
the format `helm-v*`.

The appVersion in the Helm chart does not have to point to the latest released version. This allows us to first publish a new version of the Docker image without updating the Helm chart. The Helm chart is updated when we are ready to release a new version.

This is important when a new appVersion isn't compatible with the current Helm chart.
In those cases, we can first merge the PR, thoroughly test, release new container image, and then in a separate PR update the Helm chart and the appVersion.

Steps to release a new version:
1. Create a new PR and update the helm chart versions and appVersion.
2. Merge the PR into main.
3. Create a new tag from main branch with the format `helm-vX.Y.Z`. Make sure to match the tag to the helm chart version. Push the tag.

After merging to main:

```sh
git pull origin main
git tag helm-v0.12.0
git push --tags
```
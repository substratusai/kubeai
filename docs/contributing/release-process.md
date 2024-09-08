# Release Process

This document describes the process for releasing a new version of the project.

## Docs
The docs are automatically published whenever a PR updates the docs and the PR is merged into the main branch. The docs are published to the `gh-pages` branch, which is the source for the Github Pages site.

## Docker images
The Docker image latest tag always points to the latest released version.
The `main` tag points to the latest commit on the main branch.

If you push a tag `vX.Y.Z` to the repository, the Docker image with the tag `vX.Y.Z`
is built and pushed to the Docker Hub. Afterwards, the `latest` tag is updated to point to the new version.

## Helm Chart
The Helm chart is automatically published whenever a PR updates the chart version
and the PR is merged into the main branch. This triggers the Github action
workflow defined in `.github/workflows/release-helm-chart-publish-docs.yml`.

The appVersion in the Helm chart does not have to point to the latest released version. This allows us to first publish a new version of the Docker image without updating the Helm chart. The Helm chart is updated when we are ready to release a new version.

This is important when a new appVersion isn't compatible with the current Helm chart.
In those cases, we can first merge the PR, thoroughly test, release new container image, and then in a separate PR update the Helm chart and the appVersion.

So merging PRs that update the Helm chart version should be done with care.


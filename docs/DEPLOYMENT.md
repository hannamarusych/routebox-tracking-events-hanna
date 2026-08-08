# Deployment Guide

## Overview

This service deploys to AWS ECS via the shared Jenkins pipeline used across the RouteBox platform, defined in the `routebox-jenkins` shared library.

## Pipeline

The `Jenkinsfile` in this repository imports the shared library and calls:

```
deployToEcs(service: 'tracking-events', env: ...)
```

This function knows about a service-specific special case: it wires the long-lived AWS credentials into the container from Secrets Manager. See [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) for why this exists.

## Deployment Steps

1. A merge to `main` triggers the Jenkins pipeline.
2. The pipeline builds the Docker image from the repository `Dockerfile`.
3. The image is pushed to the container registry.
4. `deployToEcs` updates the ECS service definition and triggers a rolling deployment.
5. New tasks pull configuration and secrets from Secrets Manager, including `CARRIER_SIGNATURE_SECRETS` and the AWS access key pair.

## Environments

Deploys target the dev AWS environment first. Promotion to further environments follows the same shared pipeline pattern used by other RouteBox services.

## Secret Rotation

AWS credentials for this service are rotated quarterly via the `tracking-events-rotate-keys` Jenkins job, which rotates the keys in Secrets Manager and triggers a redeploy so running tasks pick up the new values. Do not disable this job without understanding the AWS authentication tradeoff described in [TROUBLESHOOTING.md](./TROUBLESHOOTING.md).

## Rollback

Roll back by redeploying the previous known-good image tag through the same Jenkins pipeline.

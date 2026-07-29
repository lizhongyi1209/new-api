# Production release process

Production releases use `custom` as the only release branch. A hotfix may be developed elsewhere, but its commit history must be merged back into `custom` before another production deployment. Do not deploy a detached HEAD or a commit that is not on `origin/custom`.

## Required flow

1. Merge or cherry-pick the change into `custom` and push it.
2. Wait for the `custom` push CI workflow to pass.
3. Run `scripts/deploy-production.sh check`.
4. Run `scripts/deploy-production.sh build`. This builds and verifies the candidate without changing the running container.
5. Review the revision reported by the script and deploy with:

   ```bash
   DEPLOY_CONFIRM=DEPLOY_NEW_API_PRODUCTION scripts/deploy-production.sh deploy
   ```

The deploy command refuses a dirty worktree, the wrong branch, a local-only commit, or a candidate that does not descend from the latest production tag. It builds an image labelled with the full Git commit, waits for container health, checks that `/api/status` reports that commit, verifies the Kling Omni and Motion Control routes do not fall through to HTML, and then pushes an immutable annotated production tag.

Every Docker build performed by the script runs `docker builder prune -af` afterward, including failed builds.

## Version and rollback

The image stores `org.opencontainers.image.revision` and `org.opencontainers.image.created`. The same `git_commit` and `build_time` values are returned by `/api/status`, so the running binary can be matched to its source without relying on a mutable image name.

Production tags use `production-YYYYMMDD-HHMM-<12-character-sha>`. Never move or reuse one. To roll back, build the selected production tag into an image, verify its OCI revision label, and switch the service to that image. Do not reset or rewrite `custom` to perform a rollback; follow with a normal revert or corrective commit on `custom`.

## Hotfix history rule

Emergency deployments must still receive an immutable production tag. Before the next normal deployment, merge the hotfix branch into `custom` even if equivalent code was independently reimplemented. This preserves ancestry and prevents a valid production feature from disappearing during a later build.

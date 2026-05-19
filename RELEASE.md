# Release Process

## Prerequisites
- Access to Docker Hub (`deepbi` org)
- Access to build server (`root@65.109.2.88`)
- Access to helm repo (`deep-bi/druid-operator-helm-repo`)

## Steps

### 1. Prepare the release branch
```bash
git checkout master
git checkout -b release/<version>
# cherry-pick or merge feature branches
```

### 2. Regenerate artifacts
After any changes to `apis/druid/v1alpha1/druid_types.go`:
```bash
make generate    # updates zz_generated.deepcopy.go
make manifests   # updates CRD YAMLs in config/crd/bases/ and chart/crds/
make api-docs    # updates docs/api_specifications/druid.md
```

### 3. Update chart metadata
In `chart/Chart.yaml`:
- Bump `version` (chart version, e.g. `0.4.1`)
- Set `appVersion` to the release tag (e.g. `v1.3.2`)

In `chart/values.yaml`:
- Set `image.repository` to `deepbi/druid-operator`
- Verify `kube_rbac_proxy.image.repository` is `registry.k8s.io/kubebuilder/kube-rbac-proxy`

### 4. Build and push the Docker image
```bash
# Push branch to GitHub
git push origin release/<version>

# Build on remote server
ssh root@65.109.2.88
cd /tmp && rm -rf druid-operator
git clone -b release/<version> --depth 1 https://github.com/deep-bi/druid-operator.git
cd druid-operator
docker build -t deepbi/druid-operator:<tag> .
docker push deepbi/druid-operator:<tag>
```

### 5. Package and publish the Helm chart
```bash
# Clone helm repo
git clone git@github.com:deep-bi/druid-operator-helm-repo.git /tmp/druid-operator-helm-repo

# Package
helm package chart/ -d /tmp/druid-operator-helm-repo/helm-releases/

# Regenerate index
cd /tmp/druid-operator-helm-repo
helm repo index . --url https://charts.deep.bi

# Push
git add -A
git commit -m "release druid-operator chart <chart-version> (appVersion <tag>)"
git push origin main
```

### 6. Tag the release
```bash
git tag <tag>
git push origin <tag>
```

## Cherry-picking from upstream
When cherry-picking commits from `datainfrahq/druid-operator`:
```bash
git remote add upstream https://github.com/datainfrahq/druid-operator.git
git fetch upstream <branch>
git cherry-pick <commit> --no-commit
# Fix import paths: datainfrahq/druid-operator -> apache/druid-operator
# Resolve conflicts, then commit
```

## Version History

| Tag    | Chart | Changes |
|--------|-------|---------|
| v1.3.2 | 0.4.1 | Fix default healthcheck probe logic, DefaultProbes *bool, PVC annotation reconciliation, volume attribute class name, kube-rbac-proxy image fix |
| v1.3.0 | 0.3.9 | Previous release |

---
name: '[INTERNAL] Release'
about: Release a version
title: 'Release Version v$version'
labels: ''
assignees: ''

---

# Release Workflow

- [ ] If there are icinga-go-library changes: Release a new version
- [ ] Manually trigger Dependabot, should include icinga-go-library, if updated
- [ ] Update `internal/version.go`
- [ ] If schema upgrade: Ensure only one, correctly named upgrade file exists per DBMS
- [ ] If schema upgrade: Verify that a schema upgrade and a new import are identical
- [ ] Check that the `.mailmap` and `AUTHORS` files are up to date
- [ ] Update `CHANGELOG.md`
- [ ] Create and push a signed tag for the version
- [ ] Build packages
- [ ] Create release on GitHub
- [ ] Update public docs
- [ ] Announce release

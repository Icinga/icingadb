---
name: '[INTERNAL] Release'
about: Release a version
title: 'Release Version v$version'
labels: ''
assignees: ''

---

# Release Workflow

- [ ] Check that the `.mailmap` and `AUTHORS` files are up to date
- [ ] Update `internal/version.go`
- [ ] Update `CHANGELOG.md`
- [ ] Create and push a signed tag for the version
- [ ] Build packages
- [ ] Create release on GitHub
- [ ] Update public docs
- [ ] Announce release

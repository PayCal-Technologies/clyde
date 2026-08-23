# Clyde GitHub Visibility Action Plan

This plan tracks the repository changes that make Clyde easier to discover,
evaluate, trust, and share.

## Current Baseline

| Area | Status |
| --- | --- |
| Public repository | Complete |
| README with official links | Complete |
| Terminal screenshot | Complete |
| Release tags | Complete |
| Test workflow | Complete |
| License | Added in this visibility pass |
| Community profile files | Added in this visibility pass |
| Issue and PR templates | Added in this visibility pass |
| Examples index | Added in this visibility pass |
| Social preview asset | Added in this visibility pass |
| GitHub topics/homepage | Update through GitHub metadata |

## Priority Actions

1. Keep the README front-loaded with the product pitch, install command,
   screenshot, docs links, examples, and release badges.
2. Maintain a complete GitHub community profile: `LICENSE`, `SECURITY.md`,
   `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, issue templates, and PR template.
3. Keep `examples/` copy-pasteable for human users and AI/coding agents.
4. Publish clear GitHub Releases with upgrade notes, changed commands,
   verification commands, and links to the official help site.
5. Set repository metadata:
   - Homepage: `https://paycaltech.com/clyde/`
   - Topics: `go`, `cli`, `mcp`, `ollama`, `notebooklm`, `local-first`,
     `ai-tools`, `repository-analysis`, `developer-tools`, `automation`
6. Upload `.github/social-preview.png` in GitHub repository settings as the
   social preview image.
7. Review search snippets after every release: README opening paragraph,
   release title, topics, and homepage should all use the same product language.

## Recurring Release Checklist

- [ ] README version and install instructions are current.
- [ ] `CHANGELOG.md` has user-facing release notes.
- [ ] `TESTING.md` reflects the test suite.
- [ ] `examples/` still runs with the current command surface.
- [ ] Help pages and screenshots reflect current CLI output.
- [ ] GitHub release notes link to `https://paycaltech.com/clyde/help/`.
- [ ] Repository topics and homepage still match the project.

For v0.2.7, the README, changelog, testing guide, examples, and bundled help
were updated before tagging the release.

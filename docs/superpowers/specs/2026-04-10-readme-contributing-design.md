# Design: README and CONTRIBUTING Documentation

**Date**: 2026-04-10
**Status**: Approved
**Author**: Peter O'Connor + Claude

## Purpose

Create onboarding documentation for junior developers at Stack Overflow joining the slack-cli project. Two files: README.md (what it is, how it works, how to use it) and CONTRIBUTING.md (how to develop and contribute).

## README.md Structure

1. **What is slack-cli** — one paragraph: CLI wrapping the Slack Web API, 73 auto-generated commands
2. **Quick Start** — clone, build, set token, run a command
3. **How It Works** — Mermaid diagram of the pipeline: SDK introspection → generated registry → command builder → dispatch → output
4. **Architecture** — directory map with one-line descriptions
5. **Usage Examples** — 3-4 real commands showing common patterns
6. **Global Flags** — table of the 8 persistent flags

## CONTRIBUTING.md Structure

1. **Development Setup** — prerequisites, env vars, first build
2. **How to Add a New Slack API Method** — step-by-step code gen pipeline walkthrough
3. **How to Override a Generated Command** — when/how to use override mechanism
4. **Testing** — conventions (table-driven, go-cmp, no testify), how to run
5. **Code Style** — golangci-lint, naming conventions
6. **PR Process** — BDD red-green-refactor, PRs <300 lines, reviews before merge

## Audience

Internal Stack Overflow developers, primarily junior engineers onboarding to the project.

## Decisions

- Mermaid diagrams for architecture visualization (renders in GitHub)
- No Code of Conduct / CLA sections (internal project)
- Contribution workflow follows global engineering standards (BDD, <300 line PRs)

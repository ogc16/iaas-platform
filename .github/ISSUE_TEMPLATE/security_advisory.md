name: Security advisory
description: Report a security vulnerability found in the IaaS Platform
labels: ["security"]
title: "[SECURITY] "
body:
  - type: markdown
    attributes:
      value: |
        :warning: **Prefer private disclosure.** Security-sensitive issues should be
        reported through GitHub's private advisory flow at
        https://github.com/ogc16/iaas-platform/security/advisories/new rather than
        as a public issue. Use this template only when the information is already
        public or you prefer a public tracker. See [SECURITY.md](../../SECURITY.md).
  - type: input
    id: component
    attributes:
      label: Affected component
      description: Auth, API keys, organizations, compute, billing, dashboard, config, or dependency.
      placeholder: e.g. auth / JWT
    validations:
      required: true
  - type: textarea
    id: impact
    attributes:
      label: Impact
      description: What an attacker can achieve, and under what conditions (auth required?).
      placeholder: e.g. An authenticated member can read another organization's invoices via ...
    validations:
      required: true
  - type: textarea
    id: steps
    attributes:
      label: Reproduction
      description: Exact steps or API calls, plus the affected version / commit.
      render: bash
    validations:
      required: true
  - type: textarea
    id: fix
    attributes:
      label: Proposed fix (optional)
      description: Patch sketch or mitigation ideas.
  - type: checkboxes
    id: checks
    attributes:
      label: Before submitting
      options:
        - label: I confirmed this is not already reported (or the advisory is public)
          required: true

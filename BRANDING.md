# SPECTRE WORKS brand policy and brand-asset permission

Policy version: 1 — 2026-11-12. Publisher: **SPECTRE WORKS LLC**.

## Software rights and brand rights

The installer, controller and dashboard may be used, modified and redistributed,
including commercially, under their applicable software licenses. The standard
Apache License 2.0 is in `LICENSE`; retained upstream MIT terms are in
`licenses/upstream-MIT.txt`. This policy does not amend either license or add a
restriction to the use, modification or distribution of the software code.

The **SPECTRE WORKS** name, company emblem and **Rebuilt for Bitcoin** brand line
identify SPECTRE WORKS as the source. **KD BOX Control**, **KD BOX XBT Mod** and
**KD BOX XBT Installer**, when used as this project's product identity, also
identify the official SPECTRE WORKS products. No registration is asserted here.
Rights belonging to other parties, descriptive hardware references, and the
Bitcoin/XBT name are not claimed by SPECTRE WORKS.

The company emblem in `controller/web/spectre-logo.png`, graphical wordmark in
`flasher/web/spectre-wordmark.png`, and `controller/web/favicon-32.png` /
`favicon-48.png` are reserved brand assets, not Apache- or MIT-licensed artwork
under this version's licensing terms.
Copyright (c) 2026 SPECTRE WORKS LLC. The permission below governs copies of those
assets; trademark rights remain separate from copyright permission.

## Official, unmodified copies

You may copy and display the supplied emblem and product identity as part of:

- Complete, unmodified official source or release distributions, including
  identical builds made from that source with the documented build tooling.
- Private installations and private modifications that are not redistributed
  or offered as a branded service to others.
- Accurate reference to the official project, including a screenshot of its
  unmodified interface, without implying endorsement of a different product.

This limited, nonexclusive, royalty-free brand-asset permission does not authorize
extraction for unrelated products, merchandise, or a separate company identity.
Keep notices intact and identify yourself separately when redistributing an
official copy or providing independent services. Do not imply reseller,
certification, support, partnership or endorsement status that was not granted.

## Modified redistributions must use their own branding

Unless SPECTRE WORKS gives separate written permission, a modified version that
you distribute or offer as your own product or service must use a distinct
product identity. Under this policy no permission is granted to brand those
versions as official SPECTRE WORKS software. In particular:

1. Replace the SPECTRE WORKS emblem, company wordmark and brand line in the
   dashboard, installer, launch screens, package presentation and marketing.
2. Use your own product names instead of the official product identities above;
   do not substitute a confusingly similar name or logo.
3. Identify your publisher and support destination. Do not present SPECTRE WORKS
   store/support links as if SPECTRE WORKS operates or supports your derivative.
4. Describe the modification as unofficial and make its origin clear. You may
   truthfully say it is based on this project without using its emblem as your
   product logo or suggesting endorsement.

These requirements concern permission to use the reserved brand identity and
asset. They do not restrict unbranded forks, commercial software use, or rights
independently granted by the code licenses or applicable law.

## Keep legal attribution

Rebranding does **not** mean removing copyrights, licenses, NOTICE contents,
source provenance or truthful attribution. Retain them where required by the
applicable licenses. It is appropriate to identify SPECTRE WORKS in a legal
credits page while using your own visible product branding. The limited origin
and NOTICE uses permitted by Apache License 2.0 section 6 remain available.
Statutory rights, including applicable nominative uses, are not displaced.

Internal compatibility paths such as `/usr/config/kdbox-xbt`, backup schemas and
protocol identifiers need not be renamed merely to rebrand a fork. Changing
those paths can break installed-device recovery.

## Implementing a rebranded build

Review `controller/web/index.html`, `controller/web/licenses.html`, `controller/version.go`,
`flasher/branding.py`, the installer page/launchers and package metadata. Replace
the company emblem and update the asset hash in `tools/release_audit.py`; keep
that safety allowlist rather than disabling the distribution audit. Replace the
SPECTRE WORKS favicons and installer wordmark as well as the main emblem. Preserve
third-party license texts and source attribution, and rebuild the payload/notices.

## Permissions and earlier grants

Request permission through the repository's issue tracker or an established
SPECTRE WORKS business contact. A request is not permission; an exception must be
an express written grant by SPECTRE WORKS LLC. Permission for one version or use
is not an endorsement of later modified versions.

This policy does not revoke rights previously granted under an earlier license,
relicense third-party material, or make a representation that all possible uses
of a descriptive term are enforceable trademarks. The policy and asset scope
should be reviewed by qualified counsel before commercial publication.

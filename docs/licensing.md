# Licensing, attribution and brand use

KD BOX XBT Installer and KD BOX Control use **Apache License 2.0** for SPECTRE
WORKS-owned software and documentation in this version. Retained upstream code
continues under **MIT**. Fonts and other components retain their own terms.
The company emblem, installer wordmark, website-derived favicons and product
identity have a separate brand policy.

| Material | Applicable terms | Distributed text |
| --- | --- | --- |
| SPECTRE WORKS original code and documentation | Apache-2.0 | [LICENSE](../LICENSE) |
| Retained upstream application code | MIT | [Original MIT notice](../licenses/upstream-MIT.txt) |
| Publisher and modification attribution | License-required notices | [NOTICE](../NOTICE) |
| SPECTRE WORKS emblem, wordmark, favicons and product identity | Separate brand-asset permission and trademark policy | [BRANDING.md](../BRANDING.md) |
| Chakra Petch Regular and Bold | OFL-1.1 | [Font license](../licenses/chakra-petch-OFL.txt) |
| Go runtime and included components | Their retained licenses/notices | Build-specific `controller/web/open-source.txt` |

## Why this model

Apache 2.0 supports commercial use, modification and redistribution, includes an
express patent grant and reserves trademark rights in section 6. Its standard
text is unchanged. The branding policy separately grants use of the company
emblem in official unmodified distributions and requires modified redistributions
to use their own identity. An unbranded fork may remain commercial or proprietary
where the applicable software licenses permit it; this is not a copyleft policy.

A code-license change alone does not mandate a new product name. The brand policy
addresses the source-identification problem without inserting a custom
restriction into Apache 2.0. Copyright/license notices and truthful origin
references must remain even when visible product branding changes. It does not
claim control over Bitcoin/XBT or ordinary hardware descriptions.

Only rights owned or validly licensable by SPECTRE WORKS can be granted here.
The upstream MIT copyright and permission notice remains unchanged and is bundled
with source and compiled distributions. Earlier grants remain effective; changing
or removing a download does not retroactively revoke them. Review the final
trademark policy and asset ownership with qualified counsel before publication.

## Delivery in both products

**Licenses & attribution** opens a readable component overview and a complete
local notice download. The installer serves the same page locally; the controller
embeds it and the generated notice text, and the installer copies them to the
miner's original web port. No external website is required to read the terms.

The build generates `controller/web/open-source.txt` **before** embedding it. It
contains NOTICE, the standard Apache text, the original MIT text, the brand policy,
the font license and the actual Go toolchain notice inventory. Payload and package
gates require those files. `DISTRIBUTION-COMPONENTS.json` identifies the separate
licenses and reserved brand asset; it does not describe the whole product as MIT.

## Desktop dependencies

Prepared packages contain editable installer/application source and the ARM
controller. They do not contain `.venv`, uv, Python, Paramiko or a frozen desktop
executable. Launchers download checksum-pinned uv and obtain an isolated Python
environment and dependencies. A JavaScript-capable browser is required; Tk is not.
Downloaded components retain their own licenses. Later launches preserve an
operator's replacement libraries rather than reinstalling them unconditionally.

Paramiko is LGPL-licensed; its exact release and transitive dependencies have
separate terms. The environment inventory in `build/` records tested versions and
reported license metadata. It is not a full legal audit or a claim that those
libraries are included in the product archive. Operators can replace libraries:

```sh
# Use the installer's virtual-environment Python for your OS.
python -m ensurepip
python -m pip install --no-deps /path/to/modified-paramiko-source
python -m flasher
```

## Controller and other distribution formats

The controller is built without CGO or external Go modules. The build includes the
actual toolchain's LICENSE, PATENTS and discovered copyright/notice files. This
conservative inventory can include components not linked in a particular binary;
it does not mean the Go compiler ships on the miner. Adding external modules
requires a dependency/license review before packaging.

Frozen installers, offline library bundles and complete system images require a
separate inventory of the **exact artifact**, corresponding notices/sources and
any applicable library-replacement or debugging rights. A restrictive EULA cannot
override rights that third-party licenses grant. This repository does not clear
arbitrary repackagings or grant rights to Goldshell firmware, mining engines,
bootloaders, signatures or keys. Vendor images and engines are not bundled.
Preinstalled-device sales need a separate review of the software already there.

## Primary references

- [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0.html)
- [Apache trademark policy: separation of code and product identity](https://www.apache.org/foundation/marks/)
- [Mozilla software distribution and trademark policy](https://www.mozilla.org/en-US/foundation/trademarks/distribution-policy/)
- [Paramiko project and licensing](https://www.paramiko.org/)
- [GNU LGPL 2.1](https://www.gnu.org/licenses/old-licenses/lgpl-2.1.html)
- [Go license](https://go.dev/LICENSE)

This describes the repository's implemented distribution policy. It is not legal
clearance for every third-party right, business model or future artifact.

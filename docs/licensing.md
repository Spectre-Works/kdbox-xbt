# Licensing and redistribution

## Application and attribution

The upstream code is MIT-licensed. `LICENSE` retains the complete original text,
including `Copyright (c) 2026 Paperclip Flasher contributors`. `NOTICE` identifies
the derivative, its initial upstream revision, and SPECTRE WORKS modifications.
Original modifications and the new generic chip icon are also MIT-licensed.
Do not replace upstream attribution with a claim that SPECTRE WORKS wrote it all.
The MIT text must accompany copies/substantial portions, including binary and
preinstalled-device distributions. It need not be a promotional dashboard logo.

The build puts the notices in `controller/web/open-source.txt` before compiling.
The file is embedded in the Go binary, copied to the miner's web directory, and
reachable through **Licenses & attribution**. The desktop installer opens the
same local license page. A build is rejected if the required notice payload is
missing. The complete default product ZIP also includes `LICENSE` and `NOTICE`.

## Default distribution: no bundled desktop runtime

The supported package contains editable application source and a Go ARM
controller. It does not copy a local `.venv`, interpreter, Paramiko, Tcl/Tk,
cryptographic libraries, or PyInstaller into the product ZIP. Python and Tk are
installed by the operator; the first-run scripts obtain Python libraries through
pip into that operator's local environment. Subsequent launches do not reinstall
or overwrite a modified/replacement Paramiko library.

Paramiko is a separate LGPL-licensed dependency, not MIT. The upstream release
license controls the exact terms. Its transitive dependencies also retain their
own licenses. Merely adding this project's MIT file to a frozen executable would
not address those obligations. The old PyInstaller one-file distribution has been
removed from this fork's default workflow, rather than labeled compliant without
an exact dependency/source inventory and a replacement/rebuild mechanism.

The environment JSON in `build/` records versions and reported license metadata
used during testing. It is not a complete legal audit, and those recorded Python
packages are not thereby included in the product ZIP.

Operators can replace a library in their own environment, for example:

```sh
# Use .venv/bin/python on Linux or .venv\Scripts\python.exe on Windows.
python -m pip install --no-deps /path/to/modified-paramiko-source
python -m flasher
```

Use the virtual environment's Python, not an unrelated global interpreter. A
modified dependency still needs to provide the APIs this application expects.

## Compiled controller

The controller uses the Go standard library, with CGO disabled and no external
Go modules at this revision. Build tooling retains the actual Go toolchain's
LICENSE/PATENTS and other discovered license/copyright/notice files in the
bundled notice text. This conservative inventory may include components that
are not linked; it is not a claim that every listed toolchain component ships.
New external Go modules fail the build gate until their distribution obligations
are reviewed and incorporated. The exact toolchain and payload hashes are recorded
in `build/payload.json`; the ZIP includes the application source used by that build.

## Redistributing a different format

Before distributing a frozen installer, a bundled Python environment, an offline
wheelhouse, or a complete system image, inventory **that exact artifact**. Include
required copyrights/license texts, corresponding sources and the library
modification/replacement or rebuild facilities required by applicable licenses.
Review Python, Tcl/Tk, Paramiko, its cryptographic dependencies, native libraries,
and packaging tools. Do not add an EULA that prohibits rights those licenses grant,
such as applicable reverse engineering for debugging library modifications.
Supply source alongside the release rather than relying on an unmaintained offer.
This repository does not certify arbitrary repackagings.

Preinstalling only this controller/UI on a used miner does not cause the desktop
Paramiko library to be installed there. Nevertheless, vendor firmware/mining
engines and other software already on the miner have separate terms. This MIT
license does not grant rights to distribute vendor firmware, sign images, use
vendor keys, or make hardware-warranty/compatibility claims. Retain applicable
vendor notices; review any whole-device/image redistribution separately.

## Branding

The UI uses a text SPECTRE WORKS wordmark and a new, generic chip favicon. It
follows the site's neutral palette but copies no third-party font file or logo
artwork. Paperclip artwork is removed from the current source. A known upstream
logo on an upgraded miner is removed only after its verified bytes are backed up;
a custom or unrecognized logo is left alone.

Software licensing does not grant trademark rights. Do not imply endorsement by
Paperclip contributors or Goldshell. Describe the supported hardware accurately.
SPECTRE WORKS branding is used for the project owner's authorized derivative.

## Primary references

- Upstream grant: [LICENSE](../LICENSE)
- [Paramiko licensing](https://www.paramiko.org/)
- [Paramiko release/source](https://pypi.org/project/paramiko/5.1.0/)
- [GNU LGPL 2.1](https://www.gnu.org/licenses/old-licenses/lgpl-2.1.html)
- [GNU FAQ: distributing LGPL libraries](https://www.gnu.org/licenses/gpl-faq.html)
- [PyInstaller's exception and dependency responsibilities](https://pyinstaller.org/en/stable/license.html)
- [Go license](https://go.dev/LICENSE)

This document describes the engineering/distribution policy implemented here,
not a legal opinion clearing every third-party right or every future artifact.

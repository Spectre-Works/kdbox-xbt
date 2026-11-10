# Firmware compatibility

> **Evidence provenance:** The results below are preserved upstream reports from
> `connorslab/paperclip-flasher` at `5589e0604596722896266f7f0d565c1cc4484118`.
> They describe upstream tests on one original KD BOX, not new SPECTRE WORKS
> tests. This fork remains a hardware-unvalidated preview until the local
> installation checklist has been completed and results recorded.


The 0.1.0 prerelease supports original Goldshell KDBox firmware **2.2.1** only.
The 0.1.1 prerelease supports **2.0.6**, plus **2.0.5, 2.0.7q, 2.0.8, 2.0.9, 2.1.0, 2.1.1, 2.1.3, 2.2.0 and 2.2.3** after installation, accepted-share,
pool-display and reboot tests on an original KDBox / MCB_V5_4.

Other Goldshell models and control boards are not covered by these results.
A bundled mining engine or a known SSH password does not establish compatibility.

## Firmware inventory

All images below came from the [official Goldshell firmware repository](https://github.com/goldshellminer/firmware/tree/master/KDA-Miner/Goldshell-KDBox).
Their firmware payload signatures were verified against the vendor compatibility public key.

| Firmware | Installation and mining | SSH access |
| --- | --- | --- |
| 2.0.5 | Passed, including reboot | Legacy password matches firmware hash |
| 2.0.6 | Passed; reboot at 500 MHz and 740 MHz thermal run | Legacy password matches firmware hash |
| 2.0.7q | Passed, including reboot | Legacy password matches firmware hash |
| 2.0.8 | Passed, including reboot | Legacy password matches firmware hash |
| 2.0.9 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.1.0 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.1.1 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.1.3 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.2.0 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.2.1 | Passed, including reboot | 2.2.1 password matches firmware hash |
| 2.2.3 | Passed, including fan trial and reboot recovery | Newer password verified over SSH |
| 2.2.4 | Unsupported: stock SSH access unavailable | Password matches firmware hash; SSH enable hook removed |

Firmware 2.2.3 uses a read-only root filesystem and a separate fan controller.
The installer preserves read-only boot behavior and gives Paperclip sole control
of fan output. Rebooting during an unconfirmed tuning trial restored the previous
settings and mining resumed with accepted shares. Restoring the installation
backup also passed: the stock web page and startup configuration returned,
SSH remained recoverable, and the root filesystem booted read-only.

Firmware 2.2.4 removes the SSH-enable mechanism used by 2.2.3. It is rejected by
the installer. A stock-updater downgrade from 2.2.4 to the verified original KDBox
2.2.3 image succeeded on the test device, but downgrading is not automated by
Paperclip Flasher and carries the usual firmware flashing risks.

## Firmware 2.0.6 cooling

Installation defaults to 740 MHz, 430 mV and 100% fans. A staged clock test
followed by ten minutes at 740 MHz reached about 709 GH/s with accepted shares;
the highest observed chip temperature was 83°C. A separate installation and
reboot test passed at 500 MHz.

Shutdown remains set to 80°C average or 85°C chip temperature on this firmware.
These short tests do not guarantee thermal stability in a warmer environment.
Lower the clock if the device reaches its thermal limit; existing confirmed
settings are preserved when reinstalling.

## Scope of testing

An accepted share confirms that the tested miner submitted work accepted by its pool.
It does not guarantee long-term stability across every hardware revision or operating
environment. Use the installation backup and the conservative default tuning settings.

The installer does not upgrade or downgrade stock firmware. Do not flash a different
model's image or assume that a newer version preserves SSH or recovery access.

## Image checksums

SHA-256 of the downloaded CPB files:

```text
8d61a9c2ee41ae395480440e094a99e4d4043069da5d4579f522110344ca94d7  firmware-Goldshell-KDBox-2.0.5.cpb
3443b3235083a9a8fb8ab3e42bc75224b3a14b17ea814840d06ffdc89cd96d0a  firmware-Goldshell-KDBox-2.0.6.cpb
e461ce94fe799e6719f3733be1d47f354ce501fd486b7d6837cbfedb98461494  firmware-Goldshell-KDBox-2.0.7q.cpb
32740565bc88f5e130009568165886bb821f3844a85e855863f0ce20ea0e6526  firmware-Goldshell-KDBox-2.0.8.cpb
d86ea0621e96cc34c79500a898e8e47213a6154aa89e4e97fc02417e04008cbc  firmware-Goldshell-KDBox-2.0.9.cpb
4ed4e0345c9c6c754c04212e58c165ac7072dc70b5564aefbb34a011d2090004  firmware-Goldshell-KDBox-2.1.0.cpb
a4fbf61113bf627421e2d88ffd9c8b670bcbb26f3d5ea46006cb5339e66c6baf  firmware-Goldshell-KDBox-2.1.1.cpb
184b78aeb04646220b2a47776692af440550a4efdb6d9ea5ebe9eb0e4683df96  firmware-Goldshell-KDBox-2.1.3.cpb
a1dbfb6992c8508e7b971e343d44f6c3406b47e57c12906458ab6f550c5b7347  firmware-Goldshell-KDBox-2.2.0.cpb
5984af95c4434f2b20987548560263eb0b361674309e541f881551dcca55e87d  firmware-Goldshell-KDBox-2.2.1.cpb
dff68452c2102ff09978ed365868fde0044848a9dd9165fcadca44f7d9a7537c  firmware-Goldshell-KDBox-2.2.3.cpb
ed5a09cd7894d1bbff2b4c36a59dccfdfce606e2f084a2146fea4db8688b877f  firmware-Goldshell-KDBox-2.2.4.cpb
```

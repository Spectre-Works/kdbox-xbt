# Firmware compatibility

**SPECTRE WORKS installation base: stock 2.2.1** on the original KD BOX /
MCB_V5_4 / ARMv7. The installer can start SSH over the LAN and verifies
the stock management build over SSH. **Avoid 2.2.4.** Its management process
stops SSH at startup and the 2.2.3 SSH enable mechanism was removed. Its mining
and fan-control binaries also changed. [Stock firmware and SSH](customer-firmware.md)

## Current installation policy

New installs require the original KD BOX / MCB_V5_4 / ARMv7 and stock 2.2.1.
Defaults are 740 MHz / 430 mV, a 75°C hottest-chip limit, Automatic performance
off and a 30% normal fan floor. The learned spike reserve is capped at 3°C.
See the current [operating policy](automatic-performance.md) and
[clock qualification procedure](clock-characterization.md). Invalid nonces do
not trigger a safety stop; only automatic performance uses their error budget
to qualify or reduce clock.

## Remote SSH activation

**2.2.1 is the newest official original KD BOX image found with the ordinary
dashboard maintenance route for starting SSH.** Offline inspection traced
`PUT /mcb/protect` to the same stock SSH start command in 2.1.3, 2.2.0, and
2.2.1. It returned without starting SSH in 2.2.3 and 2.2.4. Goldshell's
original KD BOX image directory lists no 2.2.2. [Official 2.2.1 image](https://github.com/goldshellminer/firmware/blob/master/KDA-Miner/Goldshell-KDBox/firmware-Goldshell-KDBox-2.2.1.cpb) · [Original KD BOX image directory](https://github.com/goldshellminer/firmware/tree/master/KDA-Miner/Goldshell-KDBox)


## Firmware scope

Only stock 2.2.1 on the pinned original KD BOX / MCB_V5_4 is accepted for
installation. Firmware images are obtained separately from Goldshell; checksums
below identify vendor files and do not expand the supported installation base.
See the [offline firmware comparison](firmware-2.2.4-investigation.md) for why
later releases do not preserve the required remote SSH activation path.

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

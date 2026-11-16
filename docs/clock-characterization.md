# Fixed-voltage clock qualification

Automatic performance qualifies a ceiling for the individual miner; it does not
use a benchmark from another unit as a universal maximum. The fixed default is
740 MHz / 430 mV. Voltage stays at 430 mV during automatic clock adjustments.
Customers cannot select numeric clock or voltage values.

Automatic mode commands full fan output and uses complete fresh chip telemetry,
the selected hottest-chip limit, accepted work and the bounded hardware-error
budget to qualify or reject clock increases. Failed qualification backs off;
thermal, fan, telemetry and watchdog faults retain their safety stops.

The private configuration stores the qualified ceiling. Turning automatic mode
off restores the defined default operating point. Qualification results depend
on power delivery, cooling, environment and the individual hardware; a finite
run cannot establish an absolute silicon maximum or indefinite reliability.
See [automatic performance](automatic-performance.md) for the current algorithm.
Unit-specific measurements and qualification records belong outside this repo.

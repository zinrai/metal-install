#!/bin/sh
# Pin NIC names to PCI bus addresses for 02-00-00-00-00-00.
set -eu

cat > /etc/udev/rules.d/70-persistent-net.rules <<'RULES'
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:03:00.0", NAME="ens1f0"
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:03:00.1", NAME="ens1f1"
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:03:00.2", NAME="ens1f2"
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:03:00.3", NAME="ens1f3"
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:04:00.0", NAME="ens2f0"
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:04:00.1", NAME="ens2f1"
RULES

#!/bin/sh
# Pin NIC names to PCI bus addresses for 02-00-00-00-00-00.
set -eu

cat > /etc/udev/rules.d/70-persistent-net.rules <<'RULES'
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:01:00.0", NAME="eno1"
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:01:00.1", NAME="eno2"
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:01:00.2", NAME="eno5"
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:01:00.3", NAME="eno6"
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:02:00.0", NAME="eno3"
SUBSYSTEM=="net", ACTION=="add", KERNELS=="0000:02:00.1", NAME="eno4"
RULES

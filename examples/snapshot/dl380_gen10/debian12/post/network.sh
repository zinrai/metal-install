#!/bin/sh
# Configure systemd-networkd for 02-00-00-00-00-00.
set -eu


cat > /etc/systemd/network/bond0.netdev <<'NETDEV'
[NetDev]
Name=bond0
Kind=bond
[Bond]
Mode=802.3ad
MIIMonitorSec=100ms
NETDEV

cat > /etc/systemd/network/bond0.network <<NETWORK
[Match]
Name=bond0
[Network]
Address=192.0.2.99/24
Gateway=192.0.2.1
DNS=192.0.2.53
NETWORK

cat > /etc/systemd/network/ens1f0.network <<MEMBER
[Match]
Name=ens1f0
[Network]
Bond=bond0
MEMBER

cat > /etc/systemd/network/ens1f1.network <<MEMBER
[Match]
Name=ens1f1
[Network]
Bond=bond0
MEMBER

cat > /etc/systemd/network/bond1.netdev <<'NETDEV'
[NetDev]
Name=bond1
Kind=bond
[Bond]
Mode=802.3ad
MIIMonitorSec=100ms
NETDEV

cat > /etc/systemd/network/bond1.network <<NETWORK
[Match]
Name=bond1
[Network]
NETWORK

cat > /etc/systemd/network/ens2f0.network <<MEMBER
[Match]
Name=ens2f0
[Network]
Bond=bond1
MEMBER

cat > /etc/systemd/network/ens2f1.network <<MEMBER
[Match]
Name=ens2f1
[Network]
Bond=bond1
MEMBER


systemctl enable systemd-networkd

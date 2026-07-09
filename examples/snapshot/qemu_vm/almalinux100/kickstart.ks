# Kickstart for RHEL-family.
# Rendered per-node. This file contains only declarations. All post-install
# logic lives in scripts that are fetched and executed from %post.

rootpw --iscrypted '$6$rounds=4096$snapshot$dummy'
eula --agreed
keyboard --vckeymap=jp106
lang en_US.UTF-8
network --bootproto=dhcp --activate

clearpart --all --initlabel --drives=vda
part biosboot --fstype=biosboot --size=1 --ondisk=vda
part /boot --fstype=xfs --size=1024 --ondisk=vda
part swap --size=2048 --ondisk=vda
part / --fstype=xfs --grow --ondisk=vda


services --disabled=auditd,rpcbind,rpcbind.socket
firewall --disabled
selinux --disabled
text
timezone Asia/Tokyo --utc
timesource --ntp-server ntp.nict.jp
zerombr
bootloader

# Power off when the install finishes.
poweroff

%packages
@core
openssh-server
%end

%post --erroronfail
set -eu
BASE=http://192.0.2.1/configs/02-00-00-00-00-00/post
curl -fsSL $BASE/ssh.sh | sh
%end

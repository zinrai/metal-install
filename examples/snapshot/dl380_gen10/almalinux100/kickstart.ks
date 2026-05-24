# Kickstart for RHEL-family.
# Rendered per-node. This file contains only declarations; all
# post-install logic lives in scripts that are fetched and executed
# from %post.
#
# Field references:
#   .OS.*       from os/<id>.yml
#   .Machine.*  from machines/<id>.yml
#   .Spec.*     from InstallSpec
#   .Env.*      from env.yml

rootpw --iscrypted '$6$rounds=4096$snapshot$dummy'
eula --agreed
keyboard --vckeymap=jp106 --xlayouts='jp106'
lang en_US.UTF-8
network --bootproto=dhcp --activate

clearpart --all --drives=/dev/sda
part /boot/efi --fstype=efi --size=512  --ondisk=/dev/sda
part /boot     --fstype=xfs --size=1024 --ondisk=/dev/sda
part swap                   --size=8192 --ondisk=/dev/sda
part /         --fstype=xfs --grow      --ondisk=/dev/sda


services --disabled=auditd,rpcbind,rpcbind.socket
firewall --disabled
selinux --disabled
graphical
timezone Asia/Tokyo --utc
timesource --ntp-server ntp.nict.jp
zerombr
bootloader
poweroff

%packages
@development
python3
bash-completion
wget
net-tools
bind-utils
pciutils
chrony
ca-certificates
lsscsi
OpenIPMI
ipmitool
nfs-utils
glibc-langpack-ja
-postfix
-biosdevname
%end

%post --erroronfail
set -eu
BASE=http://192.0.2.1/configs/02-00-00-00-00-00/post
curl -fsSL $BASE/udev.sh     | sh
curl -fsSL $BASE/network.sh  | sh
curl -fsSL $BASE/ssh.sh      | sh
curl -fsSL $BASE/complete.sh | sh
%end

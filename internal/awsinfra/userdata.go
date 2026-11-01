package awsinfra

const UserData = `
#!/bin/bash
set -euxo pipefail

# Assume Amazon Linux 2023 (dnf, systemd, ec2-user)
dnf install -y docker git
systemctl enable --now docker

# Create a 2 GiB swap, builds on a 1 GiB instance will OOM without it
dd if=/dev/zero of=/swapfile bs=1M count=2048
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab

mkdir -p /etc/zeroku /etc/caddy /var/lib/zeroku

docker pull caddy:2

# Signal to the CLI that the box is ready for bootstrap
touch /var/lib/zeroku/.ready
`

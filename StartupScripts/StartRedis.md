# Redis Multi-Host Deployment Script

This repository provides a simple bash script (`redis_deploy.sh`) to launch multiple Redis instances across different remote servers using SSH. Each instance runs on a unique port and data directory.

---

## Requirements

- Redis installed on all target servers.
- SSH access (passwordless recommended, via SSH keys).
- Bash available on the machine where you run the script.

---

## Usage

```bash
./redis_deploy.sh "<ip1 ip2 ip3 ...>" <base_port> <data_dir>

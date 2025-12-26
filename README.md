# Brezno

**A transparent wrapper around Linux encryption tools for managing LUKS2 encrypted containers.**

Brezno provides similar functionality to VeraCrypt, but with a different approach: instead of implementing its own crypto stack, it's a convenient CLI wrapper around standard Linux utilities (cryptsetup, dm-crypt, losetup).

**Philosophy:** Your encrypted containers should be accessible with standard Linux commands even if this tool disappears tomorrow. Brezno simply makes the common operations easier.

**Tradeoff:** Since dm-crypt and cryptsetup require root privileges, all Brezno commands must be run with `sudo`. This is a fundamental requirement of the underlying Linux encryption stack, not a limitation of this tool.

---

**About the name:** *Brezno* means "abyss" in Slovenian.

> *"In the abyss, no one can read your data."*
> *(Well, hopefully you can — with standard Linux tools.)*

---

## Features

- **Create** encrypted containers with LUKS2 encryption
- **Mount/Unmount** containers with simple commands
- **Resize** containers to expand storage capacity (online resize)
- **Password** management - change passwords or switch to/from keyfiles
- **List** active containers with detailed information
- **Interactive mode** - prompts for missing parameters
- **CLI flag mode** - fully scriptable with all parameters as flags
- **No state files** - discovers mounted containers by querying system state
- **Standard tools** - uses cryptsetup, dm-crypt, losetup (no custom crypto)
- **Portable** - works on any Linux distribution with dm-crypt support

## Installation

### Prerequisites

Ensure you have the following system tools installed:

```bash
# Debian/Ubuntu
sudo apt install cryptsetup util-linux

# Fedora/RHEL
sudo dnf install cryptsetup util-linux

# Arch
sudo pacman -S cryptsetup util-linux
```

### Download pre-built binary

Download the latest release from [GitHub Releases](https://github.com/orkaa/brezno/releases):

```bash
# Download for your architecture (amd64 or arm64)
wget https://github.com/orkaa/brezno/releases/latest/download/brezno-linux-amd64

# Make it executable
chmod +x brezno-linux-amd64

# Install to /usr/local/bin (optional)
sudo mv brezno-linux-amd64 /usr/local/bin/brezno
```

### Build from source

```bash
# Clone the repository
git clone https://github.com/orkaa/brezno.git
cd brezno

# Build the binary
go build -o brezno ./cmd/brezno

# Install to /usr/local/bin (optional)
sudo mv brezno /usr/local/bin/
```

## Usage

All commands require root privileges (run with `sudo`).

### Create a container

```bash
# Interactive mode
sudo brezno create /data/secrets.img

# With flags
sudo brezno create /data/secrets.img --size 5G --filesystem ext4

# With keyfile instead of password
sudo brezno create /data/secrets.img --size 5G --keyfile ~/.keys/secret.key
```

**Supported filesystems:** ext4 (default), xfs, btrfs

### Mount a container

```bash
# Interactive mode
sudo brezno mount /data/secrets.img /mnt/secrets

# With flags
sudo brezno mount /data/secrets.img /mnt/secrets

# With keyfile
sudo brezno mount /data/secrets.img /mnt/secrets --keyfile ~/.keys/secret.key

# Read-only mount
sudo brezno mount /data/secrets.img /mnt/secrets --readonly
```

### Unmount a container

```bash
# By container path
sudo brezno unmount /data/secrets.img

# By mount point
sudo brezno unmount /mnt/secrets

# By mapper name
sudo brezno unmount secrets_img

# Force unmount
sudo brezno unmount /data/secrets.img --force
```

### Resize a container

```bash
# Interactive mode (container must be mounted)
sudo brezno resize /data/secrets.img

# With flags
sudo brezno resize /data/secrets.img --size 20G

# With keyfile
sudo brezno resize /data/secrets.img --size 20G --keyfile ~/.keys/secret.key

# Skip confirmation prompt
sudo brezno resize /data/secrets.img --size 20G --yes
```

**Requirements:**
- Container must be mounted before resizing
- New size must be larger than current size
- Sufficient disk space must be available for expansion

**Supported filesystems:** ext4, xfs, btrfs (all support online resize)

### Change container password

```bash
# Change password (interactive prompts)
sudo brezno password /data/secrets.img

# Change from password to keyfile
sudo brezno password /data/secrets.img --new-keyfile ~/.keys/secret.key

# Change from keyfile to password
sudo brezno password /data/secrets.img --keyfile ~/.keys/old.key

# Change from one keyfile to another
sudo brezno password /data/secrets.img --keyfile ~/.keys/old.key --new-keyfile ~/.keys/new.key

# Automated password change (for scripts)
echo -e "old_password\nnew_password\nnew_password" | sudo brezno password /data/secrets.img --password-stdin
```

**Requirements:**
- Container must be unmounted before changing credentials
- Supports all transitions: password↔password, password↔keyfile, keyfile↔password, keyfile↔keyfile

### List active containers

```bash
# Table format (default)
sudo brezno list

# JSON format (for scripting)
sudo brezno list --json
```

## How it works

Brezno creates and manages standard LUKS2 encrypted containers:

1. **Container file** - A regular file containing LUKS2 encrypted data
2. **Loop device** - Attached to the container file (`losetup`)
3. **Device mapper** - Opens LUKS container (`cryptsetup luksOpen`)
4. **Filesystem** - Mounted from `/dev/mapper/<name>` (`mount`)

**Discovery:** Instead of storing state files, Brezno queries the system:
- `dmsetup ls --target crypt` → find active mappers
- `dmsetup table` → get backing loop devices
- `losetup -l -J` → map loop devices to container files
- `/proc/mounts` → find mount points

This makes Brezno containers **fully portable** - they can be managed with standard LUKS tools even without Brezno.

## Examples

### Quick start

```bash
# Create a 5GB encrypted container
sudo brezno create ~/secure.img --size 5G

# Mount it
sudo brezno mount ~/secure.img ~/mnt/secure

# Use it
echo "secret data" | sudo tee ~/mnt/secure/secret.txt

# List mounted containers
sudo brezno list

# Unmount
sudo brezno unmount ~/secure.img
```

### Using keyfiles

```bash
# Generate a keyfile
dd if=/dev/urandom of=~/.keys/mykey bs=1024 count=1
chmod 600 ~/.keys/mykey

# Create container with keyfile
sudo brezno create /data/secure.img --size 10G --keyfile ~/.keys/mykey

# Mount with keyfile
sudo brezno mount /data/secure.img /mnt/secure --keyfile ~/.keys/mykey
```

### Resizing containers

```bash
# Create and mount a container
sudo brezno create ~/data.img --size 5G
sudo brezno mount ~/data.img ~/mnt/data

# Use the container and run out of space
# ... time passes ...

# Resize to 20GB (container must be mounted)
sudo brezno resize ~/data.img --size 20G

# Continue using with more space available
# Unmount when done
sudo brezno unmount ~/data.img
```

## Global flags

- `--verbose` / `-v` - Show debug information and executed commands
- `--quiet` / `-q` - Suppress non-error output
- `--no-color` - Disable colored output

## Architecture

```
brezno/
├── cmd/brezno/          # Main entry point
└── internal/
    ├── cli/             # Command implementations
    ├── container/       # Container operations (LUKS, loop, mount, discovery)
    ├── system/          # System utilities (executor, parser, cleanup)
    └── ui/              # User interface (logger, prompts, tables)
```

## Future features

Planned for future releases:

- `brezno backup` - Backup LUKS header
- `brezno restore` - Restore LUKS header from backup
- `brezno verify` - Verify container integrity
- `brezno info` - Show detailed container information (LUKS version, cipher, key slots, etc.)

## Security

- Uses standard Linux encryption (LUKS2 with AES)
- No custom cryptography implementation
- Relies on well-audited system tools (cryptsetup, dm-crypt)
- Passwords never logged or stored
- Containers are standard LUKS format (portable, auditable)

## License

MIT License

## Contributing

Contributions welcome! Please open an issue or pull request.

## Credits

Built with:
- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework
- [golang.org/x/term](https://golang.org/x/term) - Terminal utilities
- [fatih/color](https://github.com/fatih/color) - Colored output

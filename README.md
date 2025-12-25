# Brezno

A CLI utility for managing LUKS2 encrypted containers on Linux, similar to VeraCrypt but CLI-only.

## Features

- **Create** encrypted containers with LUKS2 encryption
- **Mount/Unmount** containers with simple commands
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

### Build from source

```bash
# Clone the repository
git clone https://github.com/nace/brezno.git
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

### List active containers

```bash
# Table format (default)
sudo brezno list

# Verbose format
sudo brezno list --verbose

# JSON format
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

## Global flags

- `--verbose` - Show detailed progress information
- `--quiet` - Suppress non-error output
- `--debug` - Show all executed commands
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

- `brezno resize` - Expand container size
- `brezno password` - Change container password/keyfile
- `brezno backup` - Backup LUKS header
- `brezno verify` - Verify container integrity
- `brezno info` - Show detailed container information

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

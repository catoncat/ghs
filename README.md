# GitHub Account Switcher

A command-line tool for managing multiple GitHub accounts on a single machine.

## Installation

Using Go:
```bash
go install github.com/catoncat/ghs@latest
```

Or build from source:
```bash
# Build the program
go build -o ghs

# Move to system path
sudo mv ghs /usr/local/bin/
```

## Commands

### Add Account
```bash
ghs add
# Interactive setup for new GitHub account
# - Set account alias, username, name, email
# - Configure SSH key (auto-generate if needed)
# - Link GPG key if available
```

### Clone Repository
```bash
# Clone repository and auto-configure if it's yours
# - Uses SSH configuration
# - Sets up Git user info
# - Configures GPG signing if key exists
ghs clone https://github.com/owner/repo.git
```

### Switch Account
```bash
# Switch repository configuration:
# - Updates Git user info
# - Sets GPG signing key if available
cd your-repository
ghs switch work
```

### Other Commands
```bash
ghs list     # List all accounts
ghs current  # Show current repository's git configuration
ghs help     # Show help information
```

## Config Files
- Program config: `~/.github-switcher.json`
- SSH config: `~/.ssh/config`

## SSH Configuration

Each account has its own Host configuration:
```
# GitHub account: work
Host github.com-work
    HostName github.com
    User git
    IdentityFile ~/.ssh/id_rsa_work
    IdentitiesOnly yes
```


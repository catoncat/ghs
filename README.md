# GitHub Account Switcher

A command-line tool for managing multiple GitHub accounts on a single machine.

## Installation

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
```

### Clone Repository
```bash
# Clone repository and auto-configure if it's yours
ghs clone https://github.com/owner/repo.git
```

### Switch Account
```bash
# Switch repository to use specified account
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


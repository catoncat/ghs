package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// GitHubAccount represents a GitHub account configuration
type GitHubAccount struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	SSHKeyPath string `json:"ssh_key_path"`
}

// Config represents the application configuration
type Config struct {
	Accounts map[string]GitHubAccount `json:"accounts"`
}

// SSHConfigTemplate represents the template for SSH config
const SSHConfigTemplate = `# GitHub account: {{.Username}}
Host github.com-{{.Username}}
    HostName github.com
    User git
    IdentityFile {{.SSHKeyPath}}
    IdentitiesOnly yes

`

var (
	configPath    string
	sshConfigPath string
)

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error getting home directory:", err)
		os.Exit(1)
	}
	configPath = filepath.Join(homeDir, ".github-switcher.json")
	sshConfigPath = filepath.Join(homeDir, ".ssh", "config")
}

func loadConfig() Config {
	config := Config{}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Println("Error reading config file:", err)
		}
		return config
	}

	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Println("Error parsing config file:", err)
		return Config{}
	}

	return config
}

func saveConfig(config Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0600)
}

func updateSSHConfig(accounts map[string]GitHubAccount) error {
	// Read existing config
	existingConfig, err := os.ReadFile(sshConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read SSH config file: %v", err)
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp(filepath.Dir(sshConfigPath), "ssh_config_tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Parse existing config to find our managed section
	var otherConfig string
	if len(existingConfig) > 0 {
		lines := strings.Split(string(existingConfig), "\n")
		inManagedSection := false
		for _, line := range lines {
			if strings.Contains(line, "# GitHub account:") {
				inManagedSection = true
				continue
			}
			// If we see a line that's not empty and doesn't start with whitespace,
			// we're out of the previous section
			if inManagedSection && line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				inManagedSection = false
			}
			if !inManagedSection {
				otherConfig += line + "\n"
			}
		}
	}

	// Create template
	tmpl, err := template.New("sshconfig").Parse(SSHConfigTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse SSH config template: %v", err)
	}

	// Write other configs first
	if otherConfig != "" {
		if !strings.HasSuffix(otherConfig, "\n\n") {
			otherConfig += "\n"
		}
		if _, err := tmpFile.WriteString(otherConfig); err != nil {
			return fmt.Errorf("failed to write existing config: %v", err)
		}
	}

	// Write config for each account
	for alias, account := range accounts {
		// Validate SSH key path
		if account.SSHKeyPath == "" {
			fmt.Printf("Warning: Skipping SSH config for account '%s' due to empty key path\n", alias)
			continue
		}

		// Check if SSH key exists
		if _, err := os.Stat(account.SSHKeyPath); os.IsNotExist(err) {
			fmt.Printf("Warning: SSH key not found for account '%s' at %s\n", alias, account.SSHKeyPath)
			continue
		}

		if err := tmpl.Execute(tmpFile, account); err != nil {
			return fmt.Errorf("failed to write SSH config: %v", err)
		}
	}

	// Close the temporary file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %v", err)
	}

	// Create backup of existing config if it exists
	if len(existingConfig) > 0 {
		backupPath := sshConfigPath + ".bak"
		if err := os.WriteFile(backupPath, existingConfig, 0600); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}
	}

	// Move temporary file to SSH config
	if err := os.Rename(tmpFile.Name(), sshConfigPath); err != nil {
		return fmt.Errorf("failed to update SSH config: %v", err)
	}

	return nil
}

// findGPGKeyID finds the GPG key ID for the given email
func findGPGKeyID(email string) (string, error) {
	cmd := exec.Command("gpg", "--list-secret-keys", "--keyid-format", "LONG", email)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list GPG keys: %v", err)
	}

	// Parse the output to find the key ID
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "sec") {
			// The key ID is in the format: sec   rsa4096/KEYID
			parts := strings.Split(line, "/")
			if len(parts) >= 2 {
				keyID := strings.Split(parts[1], " ")[0]
				return keyID, nil
			}
		}
	}

	return "", fmt.Errorf("no GPG key found for email: %s", email)
}

// configureGPGKey configures git to use the GPG key for the given email
func configureGPGKey(email string) error {
	keyID, err := findGPGKeyID(email)
	if err != nil {
		return err
	}

	// Set signing key
	if err := exec.Command("git", "config", "--global", "user.signingkey", keyID).Run(); err != nil {
		return fmt.Errorf("failed to set git user.signingkey: %v", err)
	}

	// Enable commit signing
	if err := exec.Command("git", "config", "--global", "commit.gpgsign", "true").Run(); err != nil {
		return fmt.Errorf("failed to enable commit signing: %v", err)
	}

	fmt.Printf("Configured GPG key %s for email %s\n", keyID, email)
	return nil
}

func addAccount(config Config) Config {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter account alias (e.g., work, personal): ")
	alias, _ := reader.ReadString('\n')
	alias = strings.TrimSpace(alias)

	fmt.Print("Enter GitHub username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("Enter your name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	fmt.Print("Enter your email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	homeDir, _ := os.UserHomeDir()
	defaultKeyPath := filepath.Join(homeDir, ".ssh", fmt.Sprintf("id_rsa_%s", username))

	fmt.Printf("Enter SSH key path (default: %s): ", defaultKeyPath)
	keyPath, _ := reader.ReadString('\n')
	keyPath = strings.TrimSpace(keyPath)
	if keyPath == "" {
		keyPath = defaultKeyPath
	}

	// Convert to absolute path if relative
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(homeDir, ".ssh", keyPath)
	}

	// If key doesn't exist, generate it
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		fmt.Printf("SSH key not found. Generate new key at %s? [Y/n]: ", keyPath)
		genKey, _ := reader.ReadString('\n')
		genKey = strings.ToLower(strings.TrimSpace(genKey))
		if genKey == "" || genKey == "y" || genKey == "yes" {
			// Ensure directory exists
			keyDir := filepath.Dir(keyPath)
			if err := os.MkdirAll(keyDir, 0700); err != nil {
				fmt.Printf("Error creating directory: %v\n", err)
				return config
			}

			cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "4096", "-C", email, "-f", keyPath, "-N", "")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Printf("Error generating SSH key: %v\n", err)
				return config
			}
			fmt.Printf("\nSSH key generated. Add this public key to GitHub:\n")
			fmt.Printf("cat %s.pub\n", keyPath)
		}
	}

	// Verify SSH key exists after all operations
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		fmt.Printf("Error: SSH key not found at %s\n", keyPath)
		fmt.Println("Please ensure the SSH key exists before adding the account.")
		return config
	}

	config.Accounts[alias] = GitHubAccount{
		Name:       name,
		Email:      email,
		Username:   username,
		SSHKeyPath: keyPath,
	}

	if err := updateSSHConfig(config.Accounts); err != nil {
		fmt.Printf("Error updating SSH config: %v\n", err)
	}

	fmt.Printf("\nAccount '%s' added successfully.\n", alias)
	fmt.Println("\nTo clone repositories, use:")
	fmt.Printf("git clone git@github.com-%s:owner/repo.git\n", username)
	return config
}

func switchToAccount(config Config, alias string) error {
	account, exists := config.Accounts[alias]
	if !exists {
		return fmt.Errorf("account '%s' not found", alias)
	}

	// Check if current directory is a git repository
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return fmt.Errorf("current directory is not a git repository")
	}

	// Configure git user.name and user.email for current repository
	if err := exec.Command("git", "config", "user.name", account.Name).Run(); err != nil {
		return fmt.Errorf("failed to set git user.name: %v", err)
	}

	if err := exec.Command("git", "config", "user.email", account.Email).Run(); err != nil {
		return fmt.Errorf("failed to set git user.email: %v", err)
	}

	// Configure GPG key for current repository
	keyID, err := findGPGKeyID(account.Email)
	if err != nil {
		fmt.Printf("Warning: Failed to find GPG key: %v\n", err)
		fmt.Println("You may need to set up GPG keys manually.")
	} else {
		// Set signing key for current repository
		if err := exec.Command("git", "config", "user.signingkey", keyID).Run(); err != nil {
			fmt.Printf("Warning: Failed to set git user.signingkey: %v\n", err)
		} else {
			// Enable commit signing for current repository
			if err := exec.Command("git", "config", "commit.gpgsign", "true").Run(); err != nil {
				fmt.Printf("Warning: Failed to enable commit signing: %v\n", err)
			} else {
				fmt.Printf("Configured GPG key %s for email %s\n", keyID, account.Email)
			}
		}
	}

	fmt.Printf("Switched to GitHub account: %s (%s, %s) for current repository\n", alias, account.Name, account.Email)
	return nil
}

func listAccounts(config Config) {
	fmt.Println("Available GitHub accounts:")
	if len(config.Accounts) == 0 {
		fmt.Println("  No accounts configured yet.")
		return
	}

	for alias, account := range config.Accounts {
		fmt.Printf(" %-15s (%s, %s)\n", alias, account.Name, account.Email)
	}
}

// extractRepoInfo extracts owner and repo name from GitHub URL
func extractRepoInfo(url string) (owner, repo string, err error) {
	// Handle SSH URL format: git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@github.com:") {
		parts := strings.Split(strings.TrimPrefix(url, "git@github.com:"), "/")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid SSH URL format")
		}
		owner = parts[0]
		repo = strings.TrimSuffix(parts[1], ".git")
		return
	}

	// Handle HTTPS URL format: https://github.com/owner/repo.git
	if strings.HasPrefix(url, "https://github.com/") {
		parts := strings.Split(strings.TrimPrefix(url, "https://github.com/"), "/")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid HTTPS URL format")
		}
		owner = parts[0]
		repo = strings.TrimSuffix(parts[1], ".git")
		return
	}

	return "", "", fmt.Errorf("unsupported URL format")
}

// cloneRepo clones a repository with the appropriate configuration
func cloneRepo(config Config, url string, dir string) error {
	owner, repo, err := extractRepoInfo(url)
	if err != nil {
		return fmt.Errorf("failed to parse repository URL: %v", err)
	}

	// Check if the owner matches any of our accounts
	var matchedAccount string
	var matchedAlias string
	for alias, account := range config.Accounts {
		if account.Username == owner {
			// Verify SSH key exists
			if _, err := os.Stat(account.SSHKeyPath); os.IsNotExist(err) {
				return fmt.Errorf("SSH key not found for account '%s' at %s", alias, account.SSHKeyPath)
			}
			matchedAccount = account.Username
			matchedAlias = alias
			break
		}
	}

	// Prepare clone command
	var cloneCmd *exec.Cmd
	if matchedAccount != "" {
		// If owner matches one of our accounts, use SSH config
		sshURL := fmt.Sprintf("git@github.com-%s:%s/%s.git", matchedAccount, owner, repo)
		fmt.Printf("Using SSH configuration for account '%s'\n", matchedAlias)
		cloneCmd = exec.Command("git", "clone", sshURL)
	} else {
		// If owner doesn't match, use original URL
		fmt.Println("No matching account found, using original URL")
		cloneCmd = exec.Command("git", "clone", url)
	}

	// Set target directory if specified
	if dir != "" {
		cloneCmd.Args = append(cloneCmd.Args, dir)
	}

	// Run clone command
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		if matchedAccount != "" {
			fmt.Println("\nIf you're seeing SSH key errors, try:")
			fmt.Println("1. Start ssh-agent:")
			fmt.Println("   eval \"$(ssh-agent -s)\"")
			fmt.Printf("2. Add your SSH key:\n")
			fmt.Printf("   ssh-add %s\n", config.Accounts[matchedAlias].SSHKeyPath)
			fmt.Println("\nOr verify your SSH configuration:")
			fmt.Printf("1. Test SSH connection:\n")
			fmt.Printf("   ssh -T git@github.com-%s\n", matchedAccount)
			fmt.Printf("2. Check if the key exists:\n")
			fmt.Printf("   ls -l %s\n", config.Accounts[matchedAlias].SSHKeyPath)
		}
		return fmt.Errorf("failed to clone repository: %v", err)
	}

	// If we matched an account, configure the repository
	if matchedAccount != "" {
		// Change to the cloned directory
		targetDir := dir
		if targetDir == "" {
			targetDir = repo
		}
		if err := os.Chdir(targetDir); err != nil {
			return fmt.Errorf("failed to change to repository directory: %v", err)
		}

		// Switch to the matched account in the repository
		if err := switchToAccount(config, matchedAlias); err != nil {
			fmt.Printf("Warning: Failed to configure repository: %v\n", err)
		}
	}

	return nil
}

func getCurrentAccount() error {
	// Check if current directory is a git repository
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return fmt.Errorf("current directory is not a git repository")
	}

	// Get current git user name
	nameCmd := exec.Command("git", "config", "user.name")
	name, err := nameCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get git user.name: %v", err)
	}

	// Get current git user email
	emailCmd := exec.Command("git", "config", "user.email")
	email, err := emailCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get git user.email: %v", err)
	}

	// Get current git signing key
	keyCmd := exec.Command("git", "config", "user.signingkey")
	key, _ := keyCmd.Output() // Ignore error as signing key is optional

	fmt.Printf("Current repository configuration:\n")
	fmt.Printf("Name:  %s", name)
	fmt.Printf("Email: %s", email)
	if len(key) > 0 {
		fmt.Printf("GPG:   %s", key)
	}

	return nil
}

func showHelp() {
	fmt.Println("GitHub Account Switcher - Commands:")
	fmt.Println("  add                    Add a new GitHub account and configure SSH")
	fmt.Println("  list                   List all configured accounts")
	fmt.Println("  switch <alias>         Switch to the specified account in current repository")
	fmt.Println("  current                Show current repository's git configuration")
	fmt.Println("  clone <url> [dir]      Clone a repository, automatically using SSH config if owner matches an account")
	fmt.Println("  help                   Show this help information")
	fmt.Println("\nExample SSH clone command:")
	fmt.Println("  git clone git@github.com-username:owner/repo.git")
}

func main() {
	config := loadConfig()

	if len(os.Args) < 2 {
		showHelp()
		return
	}

	command := os.Args[1]
	var err error

	switch command {
	case "list":
		listAccounts(config)

	case "add":
		config = addAccount(config)
		err = saveConfig(config)

	case "current":
		if err := getCurrentAccount(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "switch":
		if len(os.Args) < 3 {
			fmt.Println("Usage: github-switcher switch <alias>")
			os.Exit(1)
		}
		if err := switchToAccount(config, os.Args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		if err := saveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

	case "clone":
		if len(os.Args) < 3 {
			fmt.Println("Usage: github-switcher clone <repo-url> [directory]")
			os.Exit(1)
		}
		url := os.Args[2]
		dir := ""
		if len(os.Args) > 3 {
			dir = os.Args[3]
		}
		if err := cloneRepo(config, url, dir); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "help":
		showHelp()

	default:
		fmt.Printf("Unknown command: %s\n", command)
		showHelp()
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
// Package vault implements the `mooncake vault` CLI — manage Age-encrypted
// secrets stored in a vault directory (typically committed to the config repo).
package vault

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

// Command returns the `mooncake vault` command tree.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "vault",
		Usage: "Manage Age-encrypted secrets stored in a vault directory",
		Description: "Store secrets encrypted at rest in your config repo.\n\n" +
			"Secrets are Age-encrypted files resolved at apply time via `!secret vault:<path>`.\n\n" +
			"Environment variables:\n" +
			"  MOONCAKE_VAULT_IDENTITY  path to the Age identity file (default: ~/.config/mooncake/vault-identity.txt)\n" +
			"  MOONCAKE_VAULT_DIR       path to the vault directory   (default: ~/.config/mooncake/vault/)",
		Subcommands: []*cli.Command{
			initCmd(),
			addCmd(),
			listCmd(),
			pubkeyCmd(),
		},
	}
}

// initCmd generates a new Age identity and writes it to the identity file.
func initCmd() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Generate a new Age identity (private key)",
		Description: "Creates a new Age X25519 identity and writes it to\n" +
			"$MOONCAKE_VAULT_IDENTITY (default: ~/.config/mooncake/vault-identity.txt).\n\n" +
			"Print the corresponding public key (recipient) with `mooncake vault pubkey`.\n" +
			"Add the public key to your vault (or share with collaborators) so they can\n" +
			"encrypt secrets for you with `mooncake vault add`.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Overwrite an existing identity file",
			},
		},
		Action: func(c *cli.Context) error {
			path, err := identityPath()
			if err != nil {
				return err
			}
			if !c.Bool("force") {
				if _, err := os.Stat(path); err == nil {
					return fmt.Errorf("identity file already exists at %s (use --force to overwrite)", path)
				}
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return fmt.Errorf("create identity dir: %w", err)
			}
			id, err := age.GenerateX25519Identity()
			if err != nil {
				return fmt.Errorf("generate identity: %w", err)
			}
			// Write with mode 0600 — contains the private key.
			if err := os.WriteFile(path, []byte(id.String()+"\n"), 0o600); err != nil {
				return fmt.Errorf("write identity: %w", err)
			}
			fmt.Fprintf(c.App.Writer, "Identity written to %s\n", path)
			fmt.Fprintf(c.App.Writer, "Public key (recipient): %s\n", id.Recipient().String())
			return nil
		},
	}
}

// addCmd reads a secret from stdin (echo-off) and encrypts it to <vaultDir>/<name>.age.
func addCmd() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add or update an encrypted secret",
		ArgsUsage: "<name>",
		Description: "Prompts for the secret value (echo disabled), encrypts it with your\n" +
			"Age identity's public key, and writes it to <MOONCAKE_VAULT_DIR>/<name>.age.\n\n" +
			"The <name> may include subdirectories, e.g. `db/password`.\n" +
			"Reference the secret in YAML as `!secret vault:db/password`.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "recipient",
				Aliases: []string{"r"},
				Usage:   "Additional Age recipient public key (may be repeated)",
				Action:  nil,
			},
		},
		Action: func(c *cli.Context) error {
			name := c.Args().First()
			if name == "" {
				return errors.New("usage: mooncake vault add <name>")
			}
			if strings.Contains(name, "..") {
				return errors.New("vault add: name must not contain '..'")
			}

			// Collect recipients: always include this identity's public key.
			ids, err := loadIdentities()
			if err != nil {
				return err
			}
			var recips []age.Recipient
			for _, id := range ids {
				if x, ok := id.(*age.X25519Identity); ok {
					recips = append(recips, x.Recipient())
				}
			}
			// Additional recipients from --recipient flags.
			for _, pub := range c.StringSlice("recipient") {
				r, err := age.ParseX25519Recipient(pub)
				if err != nil {
					return fmt.Errorf("invalid recipient %q: %w", pub, err)
				}
				recips = append(recips, r)
			}
			if len(recips) == 0 {
				return errors.New("vault add: no recipients available (run: mooncake vault init)")
			}

			secret, err := promptSecret(c.App.ErrWriter, fmt.Sprintf("Enter secret for vault:%s: ", name))
			if err != nil {
				return err
			}

			ct, err := security.AgeEncryptBytes([]byte(secret), recips...)
			if err != nil {
				return fmt.Errorf("encrypt: %w", err)
			}

			dir, err := vaultDir()
			if err != nil {
				return err
			}
			outPath := filepath.Join(dir, filepath.Clean(name)+".age")
			if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
				return fmt.Errorf("create vault subdir: %w", err)
			}
			if err := os.WriteFile(outPath, ct, 0o600); err != nil {
				return fmt.Errorf("write secret: %w", err)
			}
			fmt.Fprintf(c.App.Writer, "Written: %s\n", outPath)
			return nil
		},
	}
}

// listCmd prints all .age files in the vault directory.
func listCmd() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List secrets in the vault directory",
		Action: func(c *cli.Context) error {
			dir, err := vaultDir()
			if err != nil {
				return err
			}
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				fmt.Fprintln(c.App.Writer, "(vault directory is empty or does not exist)")
				return nil
			}
			return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() || !strings.HasSuffix(path, ".age") {
					return nil
				}
				rel, _ := filepath.Rel(dir, path)
				name := strings.TrimSuffix(rel, ".age")
				fmt.Fprintln(c.App.Writer, name)
				return nil
			})
		},
	}
}

// pubkeyCmd prints the public key (recipient) of the current identity.
func pubkeyCmd() *cli.Command {
	return &cli.Command{
		Name:  "pubkey",
		Usage: "Print the public key (Age recipient) of the current identity",
		Action: func(c *cli.Context) error {
			ids, err := loadIdentities()
			if err != nil {
				return err
			}
			for _, id := range ids {
				if x, ok := id.(*age.X25519Identity); ok {
					fmt.Fprintln(c.App.Writer, x.Recipient().String())
				}
			}
			return nil
		},
	}
}

// promptSecret prompts on stderr and reads with echo disabled.
func promptSecret(errw io.Writer, prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Non-TTY: read from stdin directly (useful in scripts with pipe).
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		s := strings.TrimRight(string(raw), "\r\n")
		if s == "" {
			return "", errors.New("vault add: empty secret")
		}
		return s, nil
	}
	fmt.Fprint(errw, prompt)
	raw, err := term.ReadPassword(fd)
	fmt.Fprintln(errw)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	s := strings.TrimRight(string(raw), "\r\n")
	if s == "" {
		return "", errors.New("vault add: empty secret")
	}
	return s, nil
}

func identityPath() (string, error) {
	if p := os.Getenv(security.VaultIdentityEnv); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("cannot resolve home directory")
	}
	return filepath.Join(home, ".config", "mooncake", "vault-identity.txt"), nil
}

func vaultDir() (string, error) {
	dir := os.Getenv(security.VaultDirEnv)
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("cannot resolve home directory")
		}
		dir = filepath.Join(home, ".config", "mooncake", "vault")
	}
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}
	return filepath.Abs(dir)
}

func loadIdentities() ([]age.Identity, error) {
	path, err := identityPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("vault identity not found at %s (run: mooncake vault init)", path)
		}
		return nil, fmt.Errorf("open identity: %w", err)
	}
	defer f.Close()
	return age.ParseIdentities(f)
}

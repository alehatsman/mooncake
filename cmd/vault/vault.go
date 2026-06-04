// Package vault implements the `mooncake vault` CLI — manage Age-encrypted
// secrets stored in a vault directory (typically committed to the config repo).
package vault

import (
	"bufio"
	"bytes"
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

// recipientsFile is the name of the committable recipients list inside the vault dir.
const recipientsFile = "recipients.txt"

// Command returns the `mooncake vault` command tree.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "vault",
		Usage: "Manage Age-encrypted secrets stored in a vault directory",
		Description: "Store secrets encrypted at rest in your config repo.\n\n" +
			"Quick start:\n" +
			"  mooncake vault init                    # generate identity (once per machine)\n" +
			"  mooncake vault recipients add $(mooncake vault pubkey) --name $(hostname)\n" +
			"  mooncake vault add db/password         # encrypts for all registered recipients\n\n" +
			"Add a new machine:\n" +
			"  # on new machine: mooncake vault init && mooncake vault pubkey\n" +
			"  mooncake vault recipients add AGE1... --name newmachine\n" +
			"  mooncake vault rekey                   # re-encrypt all secrets for new recipient\n" +
			"  git add vault/ && git commit && git push\n\n" +
			"Environment variables:\n" +
			"  MOONCAKE_VAULT_IDENTITY  path to Age identity file (default: ~/.config/mooncake/vault-identity.txt)\n" +
			"  MOONCAKE_VAULT_DIR       path to vault directory   (default: ~/.config/mooncake/vault/)",
		Subcommands: []*cli.Command{
			initCmd(),
			addCmd(),
			listCmd(),
			pubkeyCmd(),
			rekeyCmd(),
			recipientsCmd(),
		},
	}
}

// initCmd generates a new Age identity and writes it to the identity file.
func initCmd() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Generate a new Age identity (private key) for this machine",
		Description: "Creates a new Age X25519 identity and writes it to\n" +
			"$MOONCAKE_VAULT_IDENTITY (default: ~/.config/mooncake/vault-identity.txt).\n\n" +
			"Next steps:\n" +
			"  mooncake vault recipients add $(mooncake vault pubkey) --name $(hostname)\n" +
			"  git add vault/recipients.txt && git commit",
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
			if err := os.WriteFile(path, []byte(id.String()+"\n"), 0o600); err != nil {
				return fmt.Errorf("write identity: %w", err)
			}
			fmt.Fprintf(c.App.Writer, "Identity written to %s\n", path)
			fmt.Fprintf(c.App.Writer, "Public key: %s\n\n", id.Recipient().String())
			fmt.Fprintf(c.App.Writer, "Next: register this machine as a recipient:\n")
			fmt.Fprintf(c.App.Writer, "  mooncake vault recipients add %s --name <alias>\n", id.Recipient().String())
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
		Description: "Prompts for the secret value (echo disabled), encrypts it for all\n" +
			"registered recipients (vault/recipients.txt) plus any --recipient overrides,\n" +
			"and writes it to <MOONCAKE_VAULT_DIR>/<name>.age.\n\n" +
			"The <name> may include subdirectories, e.g. `db/password`.\n" +
			"Reference the secret in YAML as `!secret vault:db/password`.",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    "recipient",
				Aliases: []string{"r"},
				Usage:   "Additional Age recipient public key (repeatable; supplements recipients.txt)",
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

			dir, err := vaultDir()
			if err != nil {
				return err
			}

			recips, err := collectRecipients(dir, c.StringSlice("recipient"))
			if err != nil {
				return err
			}

			secret, err := promptSecret(c.App.ErrWriter, fmt.Sprintf("Enter secret for vault:%s: ", name))
			if err != nil {
				return err
			}

			ct, err := security.AgeEncryptBytes([]byte(secret), recips...)
			if err != nil {
				return fmt.Errorf("encrypt: %w", err)
			}

			outPath := filepath.Join(dir, filepath.Clean(name)+".age")
			if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
				return fmt.Errorf("create vault subdir: %w", err)
			}
			if err := os.WriteFile(outPath, ct, 0o600); err != nil {
				return fmt.Errorf("write secret: %w", err)
			}
			fmt.Fprintf(c.App.Writer, "Written: %s (%d recipient(s))\n", outPath, len(recips))
			return nil
		},
	}
}

// rekeyCmd re-encrypts all .age files in the vault for the current recipients list.
func rekeyCmd() *cli.Command {
	return &cli.Command{
		Name:  "rekey",
		Usage: "Re-encrypt all secrets for the current recipients list",
		Description: "Decrypts every .age file in the vault using your identity, then\n" +
			"re-encrypts each one for all recipients in vault/recipients.txt.\n\n" +
			"Run after `vault recipients add` or `vault recipients remove` to apply the change.\n" +
			"Requires your identity to be able to decrypt the existing secrets.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Print which files would be rekeyed without writing anything",
			},
		},
		Action: func(c *cli.Context) error {
			dir, err := vaultDir()
			if err != nil {
				return err
			}

			recips, err := collectRecipients(dir, nil)
			if err != nil {
				return err
			}

			ids, err := loadIdentities()
			if err != nil {
				return err
			}

			dryRun := c.Bool("dry-run")
			var count int
			err = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if info.IsDir() || !strings.HasSuffix(path, ".age") {
					return nil
				}
				rel, _ := filepath.Rel(dir, path)
				count++
				if dryRun {
					fmt.Fprintf(c.App.Writer, "would rekey: %s\n", strings.TrimSuffix(rel, ".age"))
					return nil
				}
				if err := rekeyFile(path, ids, recips); err != nil {
					return fmt.Errorf("rekey %s: %w", rel, err)
				}
				fmt.Fprintf(c.App.Writer, "rekeyed: %s\n", strings.TrimSuffix(rel, ".age"))
				return nil
			})
			if err != nil {
				return err
			}
			if count == 0 {
				fmt.Fprintln(c.App.Writer, "(no secrets found in vault)")
				return nil
			}
			if !dryRun {
				fmt.Fprintf(c.App.Writer, "\n%d secret(s) rekeyed for %d recipient(s)\n", count, len(recips))
				fmt.Fprintln(c.App.Writer, "Don't forget: git add vault/ && git commit")
			}
			return nil
		},
	}
}

// rekeyFile decrypts path with ids and re-encrypts in place for recips.
func rekeyFile(path string, ids []age.Identity, recips []age.Recipient) error {
	ct, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	r, err := age.Decrypt(bytes.NewReader(ct), ids...)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}
	pt, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read plaintext: %w", err)
	}
	newCT, err := security.AgeEncryptBytes(pt, recips...)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	return os.WriteFile(path, newCT, 0o600)
}

// recipientsCmd returns the `vault recipients` subcommand tree.
func recipientsCmd() *cli.Command {
	return &cli.Command{
		Name:  "recipients",
		Usage: "Manage the vault recipients list (vault/recipients.txt)",
		Subcommands: []*cli.Command{
			recipientsAddCmd(),
			recipientsListCmd(),
			recipientsRemoveCmd(),
		},
	}
}

func recipientsAddCmd() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Add a recipient to the vault",
		ArgsUsage: "<AGE1... pubkey>",
		Description: "Appends the public key to vault/recipients.txt.\n" +
			"Run `vault rekey` afterwards to apply the change to existing secrets.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "name",
				Usage: "Human-readable alias stored as a comment (e.g. hostname)",
			},
		},
		Action: func(c *cli.Context) error {
			pubkey := c.Args().First()
			if pubkey == "" {
				return errors.New("usage: mooncake vault recipients add <AGE1... pubkey>")
			}
			if _, err := age.ParseX25519Recipient(pubkey); err != nil {
				return fmt.Errorf("invalid public key: %w", err)
			}

			dir, err := vaultDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return fmt.Errorf("create vault dir: %w", err)
			}

			recs, err := readRecipientsFile(dir)
			if err != nil {
				return err
			}
			for _, r := range recs {
				if r.pubkey == pubkey {
					fmt.Fprintf(c.App.Writer, "already registered: %s\n", pubkey)
					return nil
				}
			}
			recs = append(recs, namedRecipient{name: c.String("name"), pubkey: pubkey})
			if err := writeRecipientsFile(dir, recs); err != nil {
				return err
			}
			label := pubkey
			if n := c.String("name"); n != "" {
				label = n + " (" + pubkey + ")"
			}
			fmt.Fprintf(c.App.Writer, "Added: %s\n", label)
			fmt.Fprintln(c.App.Writer, "Run `mooncake vault rekey` to re-encrypt existing secrets for this recipient.")
			return nil
		},
	}
}

func recipientsListCmd() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List registered recipients",
		Action: func(c *cli.Context) error {
			dir, err := vaultDir()
			if err != nil {
				return err
			}
			recs, err := readRecipientsFile(dir)
			if err != nil {
				return err
			}
			if len(recs) == 0 {
				fmt.Fprintln(c.App.Writer, "(no recipients registered)")
				return nil
			}
			for _, r := range recs {
				if r.name != "" {
					fmt.Fprintf(c.App.Writer, "%-30s  %s\n", r.name, r.pubkey)
				} else {
					fmt.Fprintln(c.App.Writer, r.pubkey)
				}
			}
			return nil
		},
	}
}

func recipientsRemoveCmd() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "Remove a recipient from the vault",
		ArgsUsage: "<AGE1... pubkey>",
		Description: "Removes the public key from vault/recipients.txt.\n" +
			"Run `vault rekey` afterwards to re-encrypt existing secrets without this recipient.",
		Action: func(c *cli.Context) error {
			pubkey := c.Args().First()
			if pubkey == "" {
				return errors.New("usage: mooncake vault recipients remove <AGE1... pubkey>")
			}
			dir, err := vaultDir()
			if err != nil {
				return err
			}
			recs, err := readRecipientsFile(dir)
			if err != nil {
				return err
			}
			var kept []namedRecipient
			removed := false
			for _, r := range recs {
				if r.pubkey == pubkey {
					removed = true
					continue
				}
				kept = append(kept, r)
			}
			if !removed {
				return fmt.Errorf("recipient not found: %s", pubkey)
			}
			if err := writeRecipientsFile(dir, kept); err != nil {
				return err
			}
			fmt.Fprintf(c.App.Writer, "Removed: %s\n", pubkey)
			fmt.Fprintln(c.App.Writer, "Run `mooncake vault rekey` to re-encrypt existing secrets without this recipient.")
			return nil
		},
	}
}

// listCmd prints all secret names in the vault directory.
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
			var count int
			err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() || !strings.HasSuffix(path, ".age") {
					return nil
				}
				rel, _ := filepath.Rel(dir, path)
				fmt.Fprintln(c.App.Writer, strings.TrimSuffix(rel, ".age"))
				count++
				return nil
			})
			if err != nil {
				return err
			}
			if count == 0 {
				fmt.Fprintln(c.App.Writer, "(no secrets found)")
			}
			return nil
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

// ---- recipients file ----

type namedRecipient struct {
	name   string
	pubkey string
}

// readRecipientsFile parses $VAULT_DIR/recipients.txt.
// Format: optional `# name` comment line followed by `AGE1...` pubkey line,
// or bare `AGE1...` lines. Unknown lines are silently skipped.
func readRecipientsFile(dir string) ([]namedRecipient, error) {
	path := filepath.Join(dir, recipientsFile)
	f, err := os.Open(path) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open recipients: %w", err)
	}
	defer f.Close()

	var recs []namedRecipient
	var pendingName string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			pendingName = ""
			continue
		}
		if strings.HasPrefix(line, "#") {
			pendingName = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "age1") {
			recs = append(recs, namedRecipient{name: pendingName, pubkey: line})
			pendingName = ""
			continue
		}
		pendingName = ""
	}
	return recs, sc.Err()
}

// writeRecipientsFile writes recs to $VAULT_DIR/recipients.txt.
func writeRecipientsFile(dir string, recs []namedRecipient) error {
	path := filepath.Join(dir, recipientsFile)
	var sb strings.Builder
	sb.WriteString("# Vault recipients — commit this file.\n")
	sb.WriteString("# Each recipient can decrypt all secrets with their private key.\n")
	sb.WriteString("# Add:    mooncake vault recipients add <pubkey> --name <alias>\n")
	sb.WriteString("# Rekey:  mooncake vault rekey && git add vault/ && git commit\n\n")
	for _, r := range recs {
		if r.name != "" {
			fmt.Fprintf(&sb, "# %s\n", r.name)
		}
		fmt.Fprintf(&sb, "%s\n\n", r.pubkey)
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

// collectRecipients builds the recipient list for encrypt operations:
// all entries from recipients.txt plus own identity's pubkey (if not already
// present) plus any extra pubkeys.
func collectRecipients(dir string, extraPubkeys []string) ([]age.Recipient, error) {
	recs, err := readRecipientsFile(dir)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var recips []age.Recipient
	for _, r := range recs {
		if seen[r.pubkey] {
			continue
		}
		p, err := age.ParseX25519Recipient(r.pubkey)
		if err != nil {
			return nil, fmt.Errorf("recipients.txt: invalid key %q: %w", r.pubkey, err)
		}
		recips = append(recips, p)
		seen[r.pubkey] = true
	}

	// Always include own identity so the operator can decrypt their own secrets
	// even without a recipients.txt entry.
	ids, _ := loadIdentities()
	for _, id := range ids {
		if x, ok := id.(*age.X25519Identity); ok {
			pub := x.Recipient().String()
			if !seen[pub] {
				recips = append(recips, x.Recipient())
				seen[pub] = true
			}
		}
	}

	for _, pub := range extraPubkeys {
		if seen[pub] {
			continue
		}
		p, err := age.ParseX25519Recipient(pub)
		if err != nil {
			return nil, fmt.Errorf("invalid recipient %q: %w", pub, err)
		}
		recips = append(recips, p)
		seen[pub] = true
	}

	if len(recips) == 0 {
		return nil, errors.New("no recipients — run: mooncake vault init && mooncake vault recipients add $(mooncake vault pubkey)")
	}
	return recips, nil
}

// ---- helpers ----

func promptSecret(errw io.Writer, prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
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

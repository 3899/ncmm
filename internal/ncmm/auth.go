package ncmm

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/internal/filelock"
	"github.com/3899/ncmm/internal/webui"
	webauth "github.com/3899/ncmm/internal/webui/auth"
	"github.com/3899/ncmm/pkg/utils"
	"github.com/spf13/cobra"
)

const skipConfigAnnotation = "ncmm.skip-config"

type authPasswordOptions struct {
	file  string
	stdin bool
}

func newAuthCommand(root *Root) *cobra.Command {
	command := &cobra.Command{
		Use:   "auth",
		Short: "Recover WebUI administrator authentication locally",
	}
	command.AddCommand(newAuthResetPasswordCommand(root), newAuthClearCommand(root))
	return command
}

func newAuthResetPasswordCommand(root *Root) *cobra.Command {
	opts := authPasswordOptions{}
	command := &cobra.Command{
		Use:         "reset-password",
		Short:       "Set a new administrator password and revoke every browser session",
		Annotations: map[string]string{skipConfigAnnotation: "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			password, err := readRecoveryPassword(cmd, opts)
			if err != nil {
				return err
			}
			home := webAuthHome(root)
			instanceLock, err := acquireStoppedWebUI(home)
			if err != nil {
				return err
			}
			defer instanceLock.Close()
			if err := webauth.RecoverPassword(cmd.Context(), filepath.Join(home, webauth.DefaultStoreName), password); err != nil {
				return err
			}
			root.cmd.Println("WebUI administrator password updated; all browser sessions were revoked.")
			return nil
		},
	}
	command.Flags().StringVar(&opts.file, "password-file", "", "read the new password from a file")
	command.Flags().BoolVar(&opts.stdin, "password-stdin", false, "read the new password from standard input")
	return command
}

func newAuthClearCommand(root *Root) *cobra.Command {
	confirmed := false
	command := &cobra.Command{
		Use:         "clear",
		Short:       "Clear the WebUI administrator password and browser sessions",
		Annotations: map[string]string{skipConfigAnnotation: "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !confirmed {
				return fmt.Errorf("refusing to clear authentication without --yes")
			}
			home := webAuthHome(root)
			instanceLock, err := acquireStoppedWebUI(home)
			if err != nil {
				return err
			}
			defer instanceLock.Close()
			if err := webauth.ClearStore(cmd.Context(), filepath.Join(home, webauth.DefaultStoreName)); err != nil {
				return err
			}
			root.cmd.Println("WebUI authentication cleared. Restart ncmm web to perform initial setup.")
			return nil
		},
	}
	command.Flags().BoolVar(&confirmed, "yes", false, "confirm clearing all WebUI authentication state")
	return command
}

func readRecoveryPassword(cmd *cobra.Command, opts authPasswordOptions) (string, error) {
	sources := 0
	if opts.file != "" {
		sources++
	}
	if opts.stdin {
		sources++
	}
	if os.Getenv("NCMM_WEB_ADMIN_PASSWORD") != "" {
		sources++
	}
	if sources > 1 {
		return "", fmt.Errorf("choose only one password source")
	}
	if opts.file != "" {
		data, err := os.ReadFile(opts.file)
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(data), "\r\n"), nil
	}
	if opts.stdin {
		line, err := bufio.NewReader(io.LimitReader(cmd.InOrStdin(), 4096)).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	if password := os.Getenv("NCMM_WEB_ADMIN_PASSWORD"); password != "" {
		return password, nil
	}
	stdin, ok := cmd.InOrStdin().(*os.File)
	if !ok || !stdinIsTerminal(stdin) {
		return "", fmt.Errorf("provide --password-file, --password-stdin, or NCMM_WEB_ADMIN_PASSWORD")
	}
	fmt.Fprint(cmd.ErrOrStderr(), "New administrator password: ")
	first, err := readTerminalPassword(stdin)
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return "", err
	}
	fmt.Fprint(cmd.ErrOrStderr(), "Confirm administrator password: ")
	second, err := readTerminalPassword(stdin)
	fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return "", err
	}
	if first != second {
		return "", fmt.Errorf("administrator password confirmation does not match")
	}
	return first, nil
}

func webAuthHome(root *Root) string {
	return filepath.Clean(utils.Ternary(root.Opts.Home != "", root.Opts.Home, config.HomeDir))
}

func acquireStoppedWebUI(home string) (*filelock.Lock, error) {
	if err := os.MkdirAll(home, 0755); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	lock, err := filelock.TryAcquire(filepath.Join(resolved, webui.InstanceLockFilename))
	if errors.Is(err, filelock.ErrLocked) {
		return nil, fmt.Errorf("WebUI is running for home %q; stop it before using auth recovery", resolved)
	}
	if err != nil {
		return nil, err
	}
	return lock, nil
}

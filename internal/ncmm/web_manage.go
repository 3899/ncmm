package ncmm

import (
	"context"
	"fmt"
	"time"

	"github.com/3899/ncmm/internal/webui"
	"github.com/spf13/cobra"
)

func newWebStatusCommand(root *Root) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show the WebUI instance for this home",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{skipConfigAnnotation: "true"},
		RunE: func(_ *cobra.Command, _ []string) error {
			info, running, err := webui.InspectInstance(webAuthHome(root))
			if err != nil {
				return err
			}
			if !running {
				root.cmd.Println("WebUI is not running.")
				return nil
			}
			root.cmd.Printf("WebUI is running: pid=%d listen=%s instance=%s started=%s version=%s\n",
				info.PID, info.Listen, info.InstanceID, info.StartedAt.Local().Format(time.RFC3339), info.Version)
			return nil
		},
	}
}

func newWebStopCommand(root *Root) *cobra.Command {
	return &cobra.Command{
		Use:         "stop",
		Short:       "Stop the WebUI instance for this home",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{skipConfigAnnotation: "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			home := webAuthHome(root)
			info, running, err := webui.InspectInstance(home)
			if err != nil {
				return err
			}
			if !running {
				root.cmd.Println("WebUI is not running.")
				return nil
			}
			if info.PID <= 0 || info.InstanceID == "" {
				return fmt.Errorf("WebUI instance metadata is incomplete; refusing to stop an unknown process")
			}
			if err := stopWebProcess(info.PID); err != nil {
				return fmt.Errorf("stop WebUI process %d: %w", info.PID, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			for {
				current, stillRunning, inspectErr := webui.InspectInstance(home)
				if inspectErr != nil {
					return inspectErr
				}
				if !stillRunning || current.InstanceID != info.InstanceID {
					root.cmd.Printf("WebUI stopped: pid=%d instance=%s\n", info.PID, info.InstanceID)
					return nil
				}
				select {
				case <-ctx.Done():
					return fmt.Errorf("timed out waiting for WebUI process %d to stop", info.PID)
				case <-time.After(100 * time.Millisecond):
				}
			}
		},
	}
}

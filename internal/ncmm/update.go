package ncmm

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func newUpdateCommand(root *Root) *cobra.Command {
	var apply bool
	var outputJSON bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for and apply ncmm updates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var state UpdateState
			var err error
			if apply {
				state, err = root.ApplyAvailableUpdate()
			} else {
				state, err = root.CheckForUpdatesNow()
			}
			if err != nil {
				return err
			}
			if outputJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(state)
			}
			cmd.Printf("当前版本: %s\n最新版本: %s\n状态: %s\n", state.CurrentVersion, state.LatestVersion, state.UpdateStatus)
			return nil
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "download and install the available update")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "print update state as JSON")
	return cmd
}

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage profiles",
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List profiles",
	RunE:  runProfileList,
}

var profileCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileCreate,
}

var profileRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileRm,
}

func init() {
	profileCreateCmd.Flags().String("description", "", "Profile description")
	profileCmd.AddCommand(profileListCmd, profileCreateCmd, profileRmCmd)
	rootCmd.AddCommand(profileCmd)
}

func runProfileList(cmd *cobra.Command, args []string) error {
	client := newClient()
	pl, err := client.ProfileList()
	if err != nil {
		outputError(2, err.Error())
		return nil
	}

	if jsonOutput {
		outputResult(pl)
		return nil
	}

	if len(pl.Profiles) == 0 {
		fmt.Println("No profiles configured")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tTUNNELS")
	for _, p := range pl.Profiles {
		fmt.Fprintf(w, "%s\t%s\t%d\n", p.Name, p.Description, p.TunnelCount)
	}
	w.Flush()
	return nil
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	desc, _ := cmd.Flags().GetString("description")
	client := newClient()
	if err := client.ProfileCreate(args[0], desc); err != nil {
		outputError(1, err.Error())
		return nil
	}
	outputResult(fmt.Sprintf("Created profile %q", args[0]))
	return nil
}

func runProfileRm(cmd *cobra.Command, args []string) error {
	client := newClient()
	if err := client.ProfileRemove(args[0]); err != nil {
		outputError(1, err.Error())
		return nil
	}
	outputResult(fmt.Sprintf("Removed profile %q", args[0]))
	return nil
}

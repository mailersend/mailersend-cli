package profile

import (
	"fmt"
	"sort"

	"github.com/mailersend/mailersend-cli/internal/config"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage authentication profiles",
}

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE:  runList,
}

var switchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Switch active profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runSwitch,
}

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemove,
}

func init() {
	addCmd.Flags().String("token", "", "API token for this profile")
	Cmd.AddCommand(addCmd, listCmd, switchCmd, removeCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	token, _ := cmd.Flags().GetString("token")

	if token == "" && prompt.IsInteractive() {
		var err error
		token, err = prompt.Input("API Token", "mlsn_...")
		if err != nil {
			return err
		}
	}
	if token == "" {
		return fmt.Errorf("--token is required")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if _, exists := cfg.Profiles[name]; exists {
		if prompt.IsInteractive() {
			ok, err := prompt.Confirm(fmt.Sprintf("Profile %q already exists. Overwrite?", name))
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
		}
	}

	cfg.Profiles[name] = config.Profile{APIToken: token}
	if cfg.ActiveProfile == "" {
		cfg.ActiveProfile = name
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	output.Success(fmt.Sprintf("Profile %q added.", name))
	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	jsonFlag, _ := cmd.Root().PersistentFlags().GetBool("json")
	if jsonFlag {
		profiles := make([]map[string]interface{}, 0, len(cfg.Profiles))
		for name, p := range cfg.Profiles {
			profiles = append(profiles, map[string]interface{}{
				"name":      name,
				"active":    name == cfg.ActiveProfile,
				"has_token": p.APIToken != "",
				"has_oauth": p.OAuthToken != "",
			})
		}
		return output.JSON(profiles)
	}

	if len(cfg.Profiles) == 0 {
		fmt.Println("No profiles configured. Run 'mailersend profile add <name>' to create one.")
		return nil
	}

	names := make([]string, 0, len(cfg.Profiles))
	for n := range cfg.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)

	var rows [][]string
	for _, name := range names {
		p := cfg.Profiles[name]
		active := ""
		if name == cfg.ActiveProfile {
			active = "*"
		}
		method := "token"
		if p.OAuthToken != "" {
			method = "oauth"
		}
		rows = append(rows, []string{active, name, method})
	}

	output.Table([]string{"", "NAME", "METHOD"}, rows)
	return nil
}

func runSwitch(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	cfg.ActiveProfile = name
	if err := config.Save(cfg); err != nil {
		return err
	}

	output.Success(fmt.Sprintf("Switched to profile: %s", name))
	return nil
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	if prompt.IsInteractive() {
		ok, err := prompt.Confirm(fmt.Sprintf("Remove profile %q?", name))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}

	delete(cfg.Profiles, name)
	if cfg.ActiveProfile == name {
		cfg.ActiveProfile = ""
		for n := range cfg.Profiles {
			cfg.ActiveProfile = n
			break
		}
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	output.Success(fmt.Sprintf("Profile %q removed.", name))
	return nil
}

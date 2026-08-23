package clyde

import "strings"

type commandInfo struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Summary  string   `json:"summary"`
	Access   string   `json:"access"`
	Network  string   `json:"network"`
	Syntax   string   `json:"syntax"`
	Examples []string `json:"examples"`
}

var subcommandCatalog = []commandInfo{
	{Name: "config init", Category: "Configuration", Summary: "Write Clyde's default configuration file.", Access: "Writes local config", Network: "None", Syntax: "clyde config init", Examples: []string{"clyde config init"}},
	{Name: "config path", Category: "Configuration", Summary: "Print the config file path Clyde will use.", Access: "Read-only", Network: "None", Syntax: "clyde config path", Examples: []string{"clyde config path"}},
	{Name: "config show", Category: "Configuration", Summary: "Print the effective Clyde configuration as JSON.", Access: "Read-only", Network: "None", Syntax: "clyde config show", Examples: []string{"clyde config show"}},
	{Name: "bundle verify", Category: "Repository Bundles", Summary: "Verify a reviewed Clyde bundle and print its upload approval digest.", Access: "Read-only", Network: "None", Syntax: "clyde bundle verify BUNDLE_DIR", Examples: []string{"clyde bundle verify .clyde/out"}},
}

func commandCatalog() []commandInfo {
	commands := registeredCommands()
	catalog := make([]commandInfo, 0, len(commands)+len(subcommandCatalog))
	for _, command := range commands {
		catalog = append(catalog, command.Info)
	}
	catalog = append(catalog, subcommandCatalog...)
	return catalog
}

func topLevelCommandList() string {
	return strings.Join(topLevelCommandNames(), " ")
}

func topLevelCommandUsageList() string {
	return strings.Join(topLevelCommandNames(), ",")
}

func supportedShellList() string {
	return strings.Join(supportedCompletionShells(), ", ")
}

func supportedShellChoiceList() string {
	return strings.Join(supportedCompletionShells(), "|")
}

func supportedShell(name string) bool {
	_, ok := completionScriptForShell(name)
	return ok
}

func commandByName(name string) (commandInfo, bool) {
	for _, command := range commandCatalog() {
		if command.Name == name {
			return command, true
		}
	}
	return commandInfo{}, false
}

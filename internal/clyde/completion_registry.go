package clyde

type completionTarget struct {
	Names  []string
	Script string
}

func completionTargets() []completionTarget {
	return []completionTarget{
		{Names: []string{"bash"}, Script: bashCompletionScript},
		{Names: []string{"zsh"}, Script: zshCompletionScript},
		{Names: []string{"fish"}, Script: fishCompletionScript},
		{Names: []string{"powershell", "pwsh"}, Script: powerShellCompletionScript},
		{Names: []string{"elvish"}, Script: elvishCompletionScript},
		{Names: []string{"nushell", "nu"}, Script: nushellCompletionScript},
		{Names: []string{"xonsh"}, Script: xonshCompletionScript},
		{Names: []string{"tcsh"}, Script: tcshCompletionScript},
		{Names: []string{"clink"}, Script: clinkCompletionScript},
		{Names: []string{"yash"}, Script: yashCompletionScript},
		{Names: []string{"oil", "osh", "ysh"}, Script: oilCompletionScript},
	}
}

func supportedCompletionShells() []string {
	targets := completionTargets()
	shells := make([]string, 0, len(targets))
	for _, target := range targets {
		shells = append(shells, target.Names...)
	}
	return shells
}

func completionScriptForShell(shell string) (string, bool) {
	for _, target := range completionTargets() {
		for _, name := range target.Names {
			if name == shell {
				return target.Script, true
			}
		}
	}
	return "", false
}

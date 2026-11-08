// Package tools checks that required command-line tools are installed.
package tools

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"golang.org/x/mod/semver"

	"github.com/opikdev/starters/packages/cli/internal/manifest"
)

// Result is the outcome of checking one tool.
type Result struct {
	Name     string `json:"name"`
	Needs    string `json:"needs,omitempty"`
	Found    string `json:"found,omitempty"`
	OK       bool   `json:"ok"`
	Problem  string `json:"problem,omitempty"`
	Required bool   `json:"required"`
}

var versionPattern = regexp.MustCompile(`\d+\.\d+(\.\d+)?`)

// Check runs a tool's version command and compares it with the minimum version.
func Check(ctx context.Context, requirement manifest.Requirement) Result {
	result := Result{Name: requirement.Name, Needs: requirement.Version, Required: true}
	if _, err := exec.LookPath(requirement.Name); err != nil {
		result.Problem = "not installed"
		return result
	}

	args := requirement.VersionArgs
	if len(args) == 0 {
		args = []string{"--version"}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, requirement.Name, args...).CombinedOutput()
	found := versionPattern.FindString(string(output))
	result.Found = found

	if requirement.Version == "" {
		if err != nil {
			result.Problem = fmt.Sprintf("installed, but not working: %v", err)
			return result
		}
		result.OK = true
		return result
	}
	if found == "" {
		result.Problem = "installed, but its version couldn't be read"
		return result
	}
	if semver.Compare(canonical(found), canonical(requirement.Version)) < 0 {
		result.Problem = fmt.Sprintf("version %s is older than %s", found, requirement.Version)
		return result
	}
	result.OK = true
	return result
}

func canonical(version string) string {
	return semver.Canonical("v" + version)
}

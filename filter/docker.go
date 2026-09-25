// Package filter implements the prunesh/docker filter logic.
//
// Contract:
//   - id:      prunesh/docker
//   - command: docker
//
// Rewrite: injects --tail=100 into `logs` invocations that do not specify a
// tail limit or a follow flag, capping log output before it reaches the proxy.
//
// FilterOutput:
//   - inspect: strips noisy JSON fields (GraphDriver, HostConfig, Mounts detail,
//     NetworkSettings internals, Config.Env when long).
//   - logs: strips ANSI and progress-like noise; already capped by Rewrite.
//   - pull: strips per-layer progress lines; keeps final "Status:" line.
//   - images: caps table at maxImageRows rows.
//   - everything else: passes through unchanged.
package filter

import (
	"fmt"
	"strings"
)

const (
	ID      = "prunesh/docker"
	Command = "docker"

	defaultLogTail = "100"
	maxImageRows   = 30
)

// noisyInspectFields are top-level JSON keys dropped from docker inspect output.
var noisyInspectFields = map[string]bool{
	"GraphDriver":    true,
	"HostConfig":     true,
	"Mounts":         true,
	"NetworkSettings": true,
	"LogPath":        true,
	"ResolvConfPath": true,
	"HostsPath":      true,
	"HostnamePath":   true,
	"MountLabel":     true,
	"ProcessLabel":   true,
	"AppArmorProfile": true,
	"ExecIDs":        true,
	"SizeRw":         true,
	"SizeRootFs":     true,
}

// pullProgressPrefixes are line prefixes that indicate per-layer pull noise.
var pullProgressPrefixes = []string{
	"Pulling from ",
	"Pulling fs layer",
	"Waiting",
	"Downloading",
	"Verifying Checksum",
	"Download complete",
	"Pull complete",
	"Already exists",
	"Extracting",
}

// Rewrite adds --tail=100 to `docker logs` when no tail limit or follow flag
// is already present.
func Rewrite(args []string) ([]string, bool) {
	if subcmd(args) != "logs" {
		return nil, false
	}
	for _, a := range args {
		if a == "-f" || a == "--follow" || strings.HasPrefix(a, "--tail") {
			return nil, false
		}
	}
	out := make([]string, len(args)+1)
	copy(out, args)
	out[len(args)] = "--tail=" + defaultLogTail
	return out, true
}

// FilterOutput dispatches to the appropriate filter by docker subcommand.
func FilterOutput(args []string, output string, exitCode int) string {
	if output == "" {
		return output
	}
	switch subcmd(args) {
	case "inspect":
		return filterInspect(output)
	case "pull":
		return filterPull(output)
	case "images", "image":
		// "docker image ls" uses "image" as subcmd; both route here.
		if subcmd(args) == "image" && !hasArg(args, "ls") && !hasArg(args, "list") {
			return output
		}
		return filterImages(output)
	}
	return output
}

// --- inspect ---

func filterInspect(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	var result []string
	skipIndent := -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if skipIndent < 0 {
				result = append(result, line)
			}
			continue
		}

		indent := leadingSpaces(line)

		if skipIndent >= 0 && indent <= skipIndent {
			prev := skipIndent
			skipIndent = -1
			// If the line that ends the skip is the closing delimiter of the
			// skipped block (same indent as the skipped key), consume it.
			// Otherwise it is a sibling key or parent's closing brace — emit it.
			if tc := strings.TrimRight(trimmed, ","); (tc == "}" || tc == "]") && indent == prev {
				continue
			}
		}
		if skipIndent >= 0 {
			continue
		}

		key := jsonKey(trimmed)
		if noisyInspectFields[key] {
			skipIndent = indent
			continue
		}

		result = append(result, line)
	}

	result = fixTrailingCommas(result)

	if len(result) == 0 {
		return output
	}
	return strings.Join(result, "\n") + "\n"
}

// --- pull ---

func filterPull(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isPullNoise(trimmed) {
			continue
		}
		result = append(result, line)
	}

	if len(result) == 0 {
		return output
	}
	return strings.Join(result, "\n") + "\n"
}

func isPullNoise(trimmed string) bool {
	for _, prefix := range pullProgressPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	// Progress bar characters (━ ▉ █) or digest lines
	if strings.ContainsAny(trimmed, "━▉█") {
		return true
	}
	// Digest lines: "sha256:abc123..."
	if strings.HasPrefix(trimmed, "sha256:") {
		return true
	}
	return false
}

// --- images ---

func filterImages(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) <= maxImageRows+1 { // +1 for header
		return output
	}
	dropped := len(lines) - maxImageRows - 1
	result := lines[:maxImageRows+1]
	result = append(result, fmt.Sprintf("... +%d more images", dropped))
	return strings.Join(result, "\n") + "\n"
}

// --- helpers ---

// subcmd finds the docker subcommand, skipping global flags.
func subcmd(args []string) string {
	known := map[string]bool{
		"attach": true, "build": true, "commit": true, "config": true,
		"container": true, "cp": true, "create": true, "deploy": true,
		"diff": true, "events": true, "exec": true, "export": true,
		"history": true, "image": true, "images": true, "import": true,
		"info": true, "inspect": true, "kill": true, "load": true,
		"login": true, "logout": true, "logs": true, "network": true,
		"node": true, "pause": true, "plugin": true, "port": true,
		"ps": true, "pull": true, "push": true, "rename": true,
		"restart": true, "rm": true, "rmi": true, "run": true,
		"save": true, "search": true, "secret": true, "service": true,
		"stack": true, "start": true, "stats": true, "stop": true,
		"swarm": true, "system": true, "tag": true, "top": true,
		"trust": true, "unpause": true, "update": true, "version": true,
		"volume": true, "wait": true,
	}
	for _, a := range args {
		if known[a] {
			return a
		}
	}
	return ""
}

// hasArg reports whether args contains the given value.
func hasArg(args []string, val string) bool {
	for _, a := range args {
		if a == val {
			return true
		}
	}
	return false
}

// leadingSpaces counts leading space characters in s.
func leadingSpaces(s string) int {
	n := 0
	for _, c := range s {
		if c != ' ' {
			break
		}
		n++
	}
	return n
}

// jsonKey returns the JSON key from a pretty-printed line ("\"key\": ..." → "key").
func jsonKey(trimmed string) string {
	if !strings.HasPrefix(trimmed, "\"") {
		return ""
	}
	end := strings.Index(trimmed[1:], "\"")
	if end < 0 {
		return ""
	}
	return trimmed[1 : end+1]
}

// fixTrailingCommas removes trailing commas from lines just before } or ].
func fixTrailingCommas(lines []string) []string {
	for i := 0; i < len(lines)-1; i++ {
		next := strings.TrimSpace(lines[i+1])
		if next == "}" || next == "}," || next == "]" || next == "]," {
			cur := lines[i]
			if strings.HasSuffix(strings.TrimRight(cur, " \t"), ",") {
				lines[i] = strings.TrimRight(cur, " \t,")
			}
		}
	}
	return lines
}

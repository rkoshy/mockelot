package server

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// ProjectInfo is the result of scanning a directory for a package.json.
// Returned to the frontend so the user can pick a script to run.
type ProjectInfo struct {
	Name              string            `json:"name"`
	Scripts           map[string]string `json:"scripts"`            // all scripts from package.json
	SuggestedScripts  []string          `json:"suggested_scripts"`  // ordered list of likely dev-server scripts
	DetectedFramework string            `json:"detected_framework"` // e.g. "angular", "vue-vite", "react-cra", "next"
	PackageManager    string            `json:"package_manager"`    // "npm", "yarn", "pnpm", "bun"
	HasNodeModules    bool              `json:"has_node_modules"`
	Error             string            `json:"error,omitempty"` // non-empty if scan failed

	// Node version manager detection
	NvmAvailable     bool   `json:"nvm_available"`               // nvm installed (nvm.sh or nvm.exe on PATH)
	FnmAvailable     bool   `json:"fnm_available"`               // fnm binary on PATH
	VoltaAvailable   bool   `json:"volta_available"`             // volta binary on PATH
	NvmrcVersion     string `json:"nvmrc_version,omitempty"`     // .nvmrc content if found
	NodeVersionFile  string `json:"node_version_file,omitempty"` // .node-version content if found
	VoltaNodeVersion string `json:"volta_node_version,omitempty"` // package.json volta.node if found
	SuggestedManager string `json:"suggested_manager,omitempty"` // "", "nvm", "fnm" — best auto-suggestion
	SuggestedVersion string `json:"suggested_version,omitempty"` // suggested version string
}

// packageJSON is a minimal representation of package.json fields we care about.
type packageJSON struct {
	Name            string            `json:"name"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Volta           struct {
		Node string `json:"node"`
		Npm  string `json:"npm"`
	} `json:"volta"`
}

// ScanProjectDir reads a directory, parses package.json, and returns ProjectInfo.
// On error the Error field is set and the function still returns a value (not nil).
func ScanProjectDir(dir string) *ProjectInfo {
	info := &ProjectInfo{
		Scripts:        make(map[string]string),
		PackageManager: "npm",
	}

	// Detect package manager from lock files
	info.PackageManager = detectPackageManager(dir)

	// Detect version managers (independent of package.json)
	detectVersionManagers(info, dir)

	// Read package.json
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		info.Error = "Cannot read package.json: " + err.Error()
		return info
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		info.Error = "Cannot parse package.json: " + err.Error()
		return info
	}

	info.Name = pkg.Name
	if info.Name == "" {
		info.Name = filepath.Base(dir)
	}
	if pkg.Scripts != nil {
		info.Scripts = pkg.Scripts
	}

	// Volta field from package.json
	if pkg.Volta.Node != "" {
		info.VoltaNodeVersion = pkg.Volta.Node
	}

	// Detect framework
	allDeps := mergeDeps(pkg.Dependencies, pkg.DevDependencies)
	info.DetectedFramework = detectFramework(allDeps)

	// Suggest scripts
	info.SuggestedScripts = suggestScripts(pkg.Scripts, info.DetectedFramework)

	// Check node_modules
	_, err = os.Stat(filepath.Join(dir, "node_modules"))
	info.HasNodeModules = err == nil

	// Build auto-suggestion for manager + version
	buildManagerSuggestion(info)

	return info
}

// detectVersionManagers populates the nvm/fnm/volta availability and version hint fields.
func detectVersionManagers(info *ProjectInfo, dir string) {
	if runtime.GOOS == "windows" {
		// On Windows, nvm-windows is a binary on PATH
		if _, err := exec.LookPath("nvm"); err == nil {
			info.NvmAvailable = true
		}
	} else {
		// On Unix: check NVM_DIR env var or ~/.nvm/nvm.sh on disk
		nvmDir := os.Getenv("NVM_DIR")
		if nvmDir == "" {
			home, _ := os.UserHomeDir()
			nvmDir = filepath.Join(home, ".nvm")
		}
		if _, err := os.Stat(filepath.Join(nvmDir, "nvm.sh")); err == nil {
			info.NvmAvailable = true
		}
	}

	// fnm and volta are binaries on any platform
	if _, err := exec.LookPath("fnm"); err == nil {
		info.FnmAvailable = true
	}
	if _, err := exec.LookPath("volta"); err == nil {
		info.VoltaAvailable = true
	}

	// Read .nvmrc (project or any parent up to 3 levels)
	if v := readVersionFile(dir, ".nvmrc"); v != "" {
		info.NvmrcVersion = v
	}

	// Read .node-version
	if v := readVersionFile(dir, ".node-version"); v != "" {
		info.NodeVersionFile = v
	}
}

// readVersionFile reads a version hint file from the directory or up to 3 parent dirs.
func readVersionFile(dir, filename string) string {
	for i := 0; i < 4; i++ {
		if dir == "" || dir == "/" || dir == "." {
			break
		}
		p := filepath.Join(dir, filename)
		if data, err := os.ReadFile(p); err == nil {
			v := strings.TrimSpace(string(data))
			if v != "" {
				return v
			}
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// buildManagerSuggestion sets SuggestedManager and SuggestedVersion based on what's detected.
func buildManagerSuggestion(info *ProjectInfo) {
	// Priority: .nvmrc + nvm > .nvmrc + fnm > .node-version + fnm > .node-version + nvm > none
	if info.NvmrcVersion != "" {
		if info.NvmAvailable {
			info.SuggestedManager = "nvm"
			info.SuggestedVersion = info.NvmrcVersion
		} else if info.FnmAvailable {
			info.SuggestedManager = "fnm"
			info.SuggestedVersion = info.NvmrcVersion
		}
	} else if info.NodeVersionFile != "" {
		if info.FnmAvailable {
			info.SuggestedManager = "fnm"
			info.SuggestedVersion = info.NodeVersionFile
		} else if info.NvmAvailable {
			info.SuggestedManager = "nvm"
			info.SuggestedVersion = info.NodeVersionFile
		}
	}
	// If no version file found but manager is available, suggest manager without version
	// (user can fill in manually or it'll use the manager's default)
}

// detectPackageManager returns the package manager name by checking lock files.
func detectPackageManager(dir string) string {
	checks := []struct {
		file string
		pm   string
	}{
		{"bun.lockb", "bun"},
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err == nil {
			return c.pm
		}
	}
	return "npm" // default
}

// detectFramework identifies the JS framework from the dependency map.
func detectFramework(deps map[string]string) string {
	has := func(pkg string) bool {
		_, ok := deps[pkg]
		return ok
	}

	switch {
	case has("@angular/core"):
		return "angular"
	case has("nuxt"):
		return "nuxt"
	case has("next"):
		return "next"
	case has("@sveltejs/kit"):
		return "sveltekit"
	case has("gatsby"):
		return "gatsby"
	case has("@remix-run/react") || has("@remix-run/node"):
		return "remix"
	case has("astro"):
		return "astro"
	case has("react-scripts"):
		return "react-cra"
	case has("vue") && has("vite"):
		return "vue-vite"
	case has("vue"):
		return "vue"
	case has("react") && has("vite"):
		return "react-vite"
	case has("solid-js") && has("vite"):
		return "solid-vite"
	case has("vite"):
		return "vite"
	default:
		return ""
	}
}

// suggestScripts returns script names most likely to be dev-server scripts, in priority order.
func suggestScripts(scripts map[string]string, framework string) []string {
	// Priority list of script names to look for
	candidates := []string{"dev", "start", "serve", "develop"}

	// Framework-specific overrides at the front
	switch framework {
	case "angular":
		candidates = append([]string{"start"}, candidates...)
	case "react-cra":
		candidates = append([]string{"start"}, candidates...)
	case "nuxt":
		candidates = append([]string{"dev"}, candidates...)
	}

	seen := make(map[string]bool)
	var suggested []string
	for _, c := range candidates {
		if !seen[c] {
			if _, ok := scripts[c]; ok {
				suggested = append(suggested, c)
				seen[c] = true
			}
		}
	}

	// Add any remaining scripts that look dev-server-ish but aren't already included
	devKeywords := []string{"dev", "start", "serve", "watch", "local"}
	var rest []string
	for name := range scripts {
		if seen[name] {
			continue
		}
		for _, kw := range devKeywords {
			if containsStr(name, kw) {
				rest = append(rest, name)
				seen[name] = true
				break
			}
		}
	}
	sort.Strings(rest)
	suggested = append(suggested, rest...)

	return suggested
}

// mergeDeps merges two dependency maps into one.
func mergeDeps(a, b map[string]string) map[string]string {
	result := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

// containsStr reports whether s contains substr (case-insensitive substring).
func containsStr(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	sl, subl := []byte(s), []byte(substr)
	for i := 0; i <= len(sl)-len(subl); i++ {
		match := true
		for j := 0; j < len(subl); j++ {
			cs := sl[i+j]
			ct := subl[j]
			if cs >= 'A' && cs <= 'Z' {
				cs += 32
			}
			if ct >= 'A' && ct <= 'Z' {
				ct += 32
			}
			if cs != ct {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// InstallCommand returns the install command for the given package manager.
func InstallCommand(pm string) []string {
	switch pm {
	case "yarn":
		return []string{"yarn", "install"}
	case "pnpm":
		return []string{"pnpm", "install"}
	case "bun":
		return []string{"bun", "install"}
	default:
		return []string{"npm", "install"}
	}
}

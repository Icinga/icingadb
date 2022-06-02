package version

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

type VersionInfo struct {
	Version string
	Commit  string
}

// Version determines version and commit information based on multiple data sources:
//  - Version information dynamically added by `git archive` in the remaining to parameters.
//  - A hardcoded version number passed as first parameter.
//  - Commit information added to the binary by `go build`.
//
// It's supposed to be called like this in combination with setting the `export-subst` attribute for the corresponding
// file in .gitattributes:
//
//     var Version = version.Version("1.0.0-rc2", "$Format:%(describe)$", "$Format:%H$")
//
// When exported using `git archive`, the placeholders are replaced in the file and this version information is
// preferred. Otherwise the hardcoded version is used and augmented with commit information from the build metadata.
func Version(version, gitDescribe, gitHash string) *VersionInfo {
	const hashLen = 7 // Same truncation length for the commit hash as used by git describe.

	if !strings.HasPrefix(gitDescribe, "$") && !strings.HasPrefix(gitHash, "$") {
		if strings.HasPrefix(gitDescribe, "%") {
			// Only Git 2.32+ supports %(describe), older versions don't expand it but keep it as-is.
			// Fall back to the hardcoded version augmented with the commit hash.
			gitDescribe = version

			if len(gitHash) >= hashLen {
				gitDescribe += "-g" + gitHash[:hashLen]
			}
		}

		return &VersionInfo{
			Version: gitDescribe,
			Commit:  gitHash,
		}
	} else {
		commit := ""

		if info, ok := debug.ReadBuildInfo(); ok {
			modified := false

			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					commit = setting.Value
				case "vcs.modified":
					modified, _ = strconv.ParseBool(setting.Value)
				}
			}

			if len(commit) >= hashLen {
				version += "-g" + commit[:hashLen]

				if modified {
					version += "-dirty"
					commit += " (modified)"
				}
			}
		}

		return &VersionInfo{
			Version: version,
			Commit:  commit,
		}
	}
}

// Print writes verbose version output to stdout.
func (v *VersionInfo) Print() {
	fmt.Println("Icinga DB version:", v.Version)
	fmt.Println()

	fmt.Println("Build information:")
	fmt.Printf("  Go version: %s (%s, %s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if v.Commit != "" {
		fmt.Println("  Git commit:", v.Commit)
	}

	if r, err := readOsRelease(); err == nil {
		fmt.Println()
		fmt.Println("System information:")
		fmt.Println("  Platform:", r.Name)
		fmt.Println("  Platform version:", r.DisplayVersion())
	}
}

// osRelease contains the information obtained from the os-release file.
type osRelease struct {
	Name      string
	Version   string
	VersionId string
	BuildId   string
}

// DisplayVersion returns the most suitable version information for display purposes.
func (o *osRelease) DisplayVersion() string {
	if o.Version != "" {
		// Most distributions set VERSION
		return o.Version
	} else if o.VersionId != "" {
		// Some only set VERSION_ID (Alpine Linux for example)
		return o.VersionId
	} else if o.BuildId != "" {
		// Others only set BUILD_ID (Arch Linux for example)
		return o.BuildId
	} else {
		return "(unknown)"
	}
}

// readOsRelease reads and parses the os-release file.
func readOsRelease() (*osRelease, error) {
	for _, path := range []string{"/etc/os-release", "/usr/lib/os-release"} {
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // Try next path.
			} else {
				return nil, err
			}
		}

		o := &osRelease{
			Name: "Linux", // Suggested default as per os-release(5) man page.
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "#") {
				continue // Ignore comment.
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue // Ignore empty or possibly malformed line.
			}

			key := parts[0]
			val := parts[1]

			// Unquote strings. This isn't fully compliant with the specification which allows using some shell escape
			// sequences. However, typically quotes are only used to allow whitespace within the value.
			if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[0] == val[len(val)-1] {
				val = val[1 : len(val)-1]
			}

			switch key {
			case "NAME":
				o.Name = val
			case "VERSION":
				o.Version = val
			case "VERSION_ID":
				o.VersionId = val
			case "BUILD_ID":
				o.BuildId = val
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}

		return o, nil
	}

	return nil, errors.New("os-release file not found")
}

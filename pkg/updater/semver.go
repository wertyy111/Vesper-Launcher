package updater

import (
	"fmt"
	"strconv"
	"strings"
)

// SemVer represents a parsed semantic version with optional pre-release.
type SemVer struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      int
}

// ParseSemVer parses a version string like "1.0.65", "v1.0.65-beta.46", "1.0".
func ParseSemVer(v string) (SemVer, error) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")

	if v == "" {
		return SemVer{}, fmt.Errorf("empty version string")
	}

	res := SemVer{}
	parts := strings.SplitN(v, "-", 2)
	core := parts[0]

	if len(parts) > 1 {
		res.PreRelease = parts[1]
		// Try to extract build number if format is e.g. "beta.46"
		if strings.Contains(res.PreRelease, ".") {
			sub := strings.Split(res.PreRelease, ".")
			for i := len(sub) - 1; i >= 0; i-- {
				if n, err := strconv.Atoi(sub[i]); err == nil {
					res.Build = n
					break
				}
			}
		}
	}

	nums := strings.Split(core, ".")
	if len(nums) > 0 && nums[0] != "" {
		res.Major, _ = strconv.Atoi(nums[0])
	}
	if len(nums) > 1 && nums[1] != "" {
		res.Minor, _ = strconv.Atoi(nums[1])
	}
	if len(nums) > 2 && nums[2] != "" {
		res.Patch, _ = strconv.Atoi(nums[2])
	}

	return res, nil
}

// Compare compares two SemVer instances.
// Returns:
//
//	 1 if a > b
//	-1 if a < b
//	 0 if a == b
func (a SemVer) Compare(b SemVer) int {
	if a.Major != b.Major {
		if a.Major > b.Major {
			return 1
		}
		return -1
	}
	if a.Minor != b.Minor {
		if a.Minor > b.Minor {
			return 1
		}
		return -1
	}
	if a.Patch != b.Patch {
		if a.Patch > b.Patch {
			return 1
		}
		return -1
	}

	// Normal release is higher than pre-release of the same core version
	if a.PreRelease == "" && b.PreRelease != "" {
		return 1
	}
	if a.PreRelease != "" && b.PreRelease == "" {
		return -1
	}
	if a.PreRelease != "" && b.PreRelease != "" {
		if a.Build != b.Build {
			if a.Build > b.Build {
				return 1
			}
			return -1
		}
		return strings.Compare(a.PreRelease, b.PreRelease)
	}

	return 0
}

// IsNewer returns true if candidate is strictly newer than current.
func IsNewer(candidate, current string) bool {
	candVer, err1 := ParseSemVer(candidate)
	currVer, err2 := ParseSemVer(current)
	if err1 != nil || err2 != nil {
		return candidate != current && candidate != ""
	}
	return candVer.Compare(currVer) > 0
}

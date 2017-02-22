package check

import (
	"errors"
	"fmt"
)

// PackageQuery is a query for finding a package
type PackageQuery struct {
	Name    string
	Version string
}

func (p PackageQuery) String() string {
	return fmt.Sprintf("%s %s", p.Name, p.Version)
}

// The PackageCheck uses the operating system to determine whether a
// package is installed or is available for installation.
type PackageCheck struct {
	PackageQuery         PackageQuery
	PackageManager       PackageManager
	InstallationDisabled bool
}

// Check returns true if the package is installed. If installation is allowed,
// Check will also return true if the package is available for installation, but is not installed.
// Otherwise, Check returns false.
func (c PackageCheck) Check() (bool, error) {
	installed, err := c.PackageManager.IsInstalled(c.PackageQuery)
	if err != nil {
		return false, fmt.Errorf("failed to determine if package is installed: %v", err)
	}
	if installed {
		return true, nil
	}
	// If installation is not allowed, and the package is not installed, fail.
	if c.InstallationDisabled && !installed {
		return false, errors.New("Package is not installed, and package installation is disabled")
	}
	available, err := c.PackageManager.IsAvailable(c.PackageQuery)
	if err != nil {
		return false, fmt.Errorf("failed to determine if package is available for installation: %v", err)
	}
	return available, nil
}

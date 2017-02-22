package check

import "testing"

type stubPkgManager struct {
	installed bool
	available bool
}

func (m stubPkgManager) IsInstalled(q PackageQuery) (bool, error) {
	return m.installed, nil
}

func (m stubPkgManager) IsAvailable(q PackageQuery) (bool, error) {
	return m.available, nil
}

func TestPackageCheck(t *testing.T) {
	tests := []struct {
		installEnabled bool
		isInstalled    bool
		isAvailable    bool
		expected       bool
		errExpected    bool
	}{
		{
			installEnabled: true,
			isInstalled:    true,
			isAvailable:    true,
			expected:       true,
		},
		{
			installEnabled: true,
			isInstalled:    false,
			isAvailable:    true,
			expected:       true,
		},
		{
			installEnabled: true,
			isInstalled:    false,
			isAvailable:    false,
			expected:       false,
		},
		{
			installEnabled: false,
			isInstalled:    true,
			isAvailable:    true,
			expected:       true,
		},
		{
			installEnabled: false,
			isInstalled:    false,
			isAvailable:    true,
			expected:       false,
			errExpected:    true,
		},
		{
			installEnabled: false,
			isInstalled:    true,
			isAvailable:    false,
			expected:       true,
		},

		{
			installEnabled: false,
			isInstalled:    false,
			isAvailable:    false,
			expected:       false,
			errExpected:    true,
		},
	}

	for i, test := range tests {
		c := PackageCheck{
			PackageQuery:        PackageQuery{"somePkg", "someVersion"},
			PackageManager:      stubPkgManager{installed: test.isInstalled, available: test.isAvailable},
			InstallationAllowed: test.installEnabled,
		}
		ok, err := c.Check()
		if err != nil && !test.errExpected {
			t.Errorf("test #%d - unexpected error occurred: %v", i, err)
		}
		if ok != test.expected {
			t.Errorf("Test #%d - Expected %v, but got %v", i, test.expected, ok)
		}
	}
}

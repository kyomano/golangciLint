package test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"

	assert "github.com/stretchr/testify/require"

	"github.com/golangci/golangci-lint/pkg/exitcodes"
	"github.com/golangci/golangci-lint/pkg/lint/lintersdb"
)

var root = filepath.Join("..", "...")
var installOnce sync.Once

const noIssuesOut = ""

func installBinary(t assert.TestingT) {
	installOnce.Do(func() {
		cmd := exec.Command("go", "install", filepath.Join("..", "cmd", binName))
		assert.NoError(t, cmd.Run(), "Can't go install %s", binName)
	})
}

func checkNoIssuesRun(t *testing.T, out string, exitCode int) {
	assert.Equal(t, exitcodes.Success, exitCode)
	assert.Equal(t, noIssuesOut, out)
}

func TestNoCongratsMessage(t *testing.T) {
	out, exitCode := runGolangciLint(t, filepath.Join("..", "..."))
	assert.Equal(t, exitcodes.Success, exitCode, out)
	assert.Equal(t, "", out)
}

func TestCongratsMessageIfNoIssues(t *testing.T) {
	out, exitCode := runGolangciLint(t, root)
	checkNoIssuesRun(t, out, exitCode)
}

func TestAutogeneratedNoIssues(t *testing.T) {
	out, exitCode := runGolangciLint(t, filepath.Join(testdataDir, "autogenerated"))
	checkNoIssuesRun(t, out, exitCode)
}

func TestEmptyDirRun(t *testing.T) {
	out, exitCode := runGolangciLint(t, filepath.Join(testdataDir, "nogofiles"))
	assert.Equal(t, exitcodes.NoGoFiles, exitCode)
	assert.Contains(t, out, ": no go files to analyze")
}

func TestNotExistingDirRun(t *testing.T) {
	out, exitCode := runGolangciLint(t, filepath.Join(testdataDir, "no_such_dir"))
	assert.True(t, exitCode == exitcodes.WarningInTest || exitCode == exitcodes.IssuesFound)
	assert.Contains(t, out, `cannot find package \"./testdata/no_such_dir\"`)
}

func TestSymlinkLoop(t *testing.T) {
	out, exitCode := runGolangciLint(t, filepath.Join(testdataDir, "symlink_loop", "..."))
	checkNoIssuesRun(t, out, exitCode)
}

func TestDeadline(t *testing.T) {
	out, exitCode := runGolangciLint(t, "--deadline=1ms", root)
	assert.Equal(t, exitcodes.Timeout, exitCode)
	assert.Contains(t, strings.ToLower(out), "deadline exceeded: try increase it by passing --deadline option")
}

func runGolangciLint(t *testing.T, args ...string) (string, int) {
	installBinary(t)

	runArgs := append([]string{"run"}, args...)
	log.Printf("golangci-lint %s", strings.Join(runArgs, " "))
	cmd := exec.Command("golangci-lint", runArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			t.Logf("stderr: %s", exitError.Stderr)
			ws := exitError.Sys().(syscall.WaitStatus)
			return string(out), ws.ExitStatus()
		}

		t.Fatalf("can't get error code from %s", err)
		return "", -1
	}

	// success, exitCode should be 0 if go is ok
	ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
	return string(out), ws.ExitStatus()
}

func runGolangciLintWithYamlConfig(t *testing.T, cfg string, args ...string) string {
	out, ec := runGolangciLintWithYamlConfigWithCode(t, cfg, args...)
	assert.Equal(t, exitcodes.Success, ec)

	return out
}

func runGolangciLintWithYamlConfigWithCode(t *testing.T, cfg string, args ...string) (string, int) {
	f, err := ioutil.TempFile("", "golangci_lint_test")
	assert.NoError(t, err)
	f.Close()

	cfgPath := f.Name() + ".yml"
	err = os.Rename(f.Name(), cfgPath)
	assert.NoError(t, err)

	defer os.Remove(cfgPath)

	cfg = strings.TrimSpace(cfg)
	cfg = strings.Replace(cfg, "\t", " ", -1)

	err = ioutil.WriteFile(cfgPath, []byte(cfg), os.ModePerm)
	assert.NoError(t, err)

	pargs := append([]string{"-c", cfgPath}, args...)
	return runGolangciLint(t, pargs...)
}

func TestTestsAreLintedByDefault(t *testing.T) {
	out, exitCode := runGolangciLint(t, filepath.Join(testdataDir, "withtests"))
	assert.Equal(t, exitcodes.IssuesFound, exitCode)
	assert.Contains(t, out, "if block ends with a return")
}

func TestCgoOk(t *testing.T) {
	out, exitCode := runGolangciLint(t, "--enable-all", filepath.Join(testdataDir, "cgo"))
	checkNoIssuesRun(t, out, exitCode)
}

func TestCgoWithIssues(t *testing.T) {
	out, exitCode := runGolangciLint(t, "--enable-all", filepath.Join(testdataDir, "cgo_with_issues"))
	assert.Equal(t, exitcodes.IssuesFound, exitCode)
	assert.Contains(t, out, "Printf format %t has arg cs of wrong type")
}

func TestUnsafeOk(t *testing.T) {
	out, exitCode := runGolangciLint(t, "--enable-all", filepath.Join(testdataDir, "unsafe"))
	checkNoIssuesRun(t, out, exitCode)
}

func TestSkippedDirs(t *testing.T) {
	out, exitCode := runGolangciLint(t, "--print-issued-lines=false", "--no-config", "--skip-dirs", "skip_me", "-Egolint",
		filepath.Join(testdataDir, "skipdirs", "..."))
	assert.Equal(t, exitcodes.IssuesFound, exitCode)
	assert.Equal(t, out,
		"testdata/skipdirs/examples_no_skip/with_issue.go:8:9: if block ends with "+
			"a return statement, so drop this else and outdent its block (golint)\n")
}

func TestDeadcodeNoFalsePositivesInMainPkg(t *testing.T) {
	out, exitCode := runGolangciLint(t, "--no-config", "--disable-all", "-Edeadcode",
		filepath.Join(testdataDir, "deadcode_main_pkg"))
	checkNoIssuesRun(t, out, exitCode)
}

func TestIdentifierUsedOnlyInTests(t *testing.T) {
	out, exitCode := runGolangciLint(t, "--no-config", "--disable-all", "-Eunused",
		filepath.Join(testdataDir, "used_only_in_tests"))
	checkNoIssuesRun(t, out, exitCode)
}

func TestConfigFileIsDetected(t *testing.T) {
	checkGotConfig := func(out string, exitCode int) {
		assert.Equal(t, exitcodes.Success, exitCode, out)
		assert.Equal(t, "test\n", out) // test config contains InternalTest: true, it triggers such output
	}

	checkGotConfig(runGolangciLint(t, "testdata/withconfig/pkg"))
	checkGotConfig(runGolangciLint(t, "testdata/withconfig/..."))

	out, exitCode := runGolangciLint(t) // doesn't detect when no args
	checkNoIssuesRun(t, out, exitCode)
}

func inSlice(s []string, v string) bool {
	for _, sv := range s {
		if sv == v {
			return true
		}
	}

	return false
}

func getEnabledByDefaultFastLintersExcept(except ...string) []string {
	m := lintersdb.NewManager()
	ebdl := m.GetAllEnabledByDefaultLinters()
	ret := []string{}
	for _, lc := range ebdl {
		if lc.NeedsSSARepr {
			continue
		}

		if !inSlice(except, lc.Name()) {
			ret = append(ret, lc.Name())
		}
	}

	return ret
}

func getAllFastLintersWith(with ...string) []string {
	linters := lintersdb.NewManager().GetAllSupportedLinterConfigs()
	ret := append([]string{}, with...)
	for _, lc := range linters {
		if lc.NeedsSSARepr {
			continue
		}
		ret = append(ret, lc.Name())
	}

	return ret
}

func getEnabledByDefaultLinters() []string {
	ebdl := lintersdb.NewManager().GetAllEnabledByDefaultLinters()
	ret := []string{}
	for _, lc := range ebdl {
		ret = append(ret, lc.Name())
	}

	return ret
}

func getEnabledByDefaultFastLintersWith(with ...string) []string {
	ebdl := lintersdb.NewManager().GetAllEnabledByDefaultLinters()
	ret := append([]string{}, with...)
	for _, lc := range ebdl {
		if lc.NeedsSSARepr {
			continue
		}

		ret = append(ret, lc.Name())
	}

	return ret
}

func mergeMegacheck(linters []string) []string {
	if inSlice(linters, "staticcheck") &&
		inSlice(linters, "gosimple") &&
		inSlice(linters, "unused") {
		ret := []string{"megacheck"}
		for _, linter := range linters {
			if !inSlice([]string{"staticcheck", "gosimple", "unused"}, linter) {
				ret = append(ret, linter)
			}
		}

		return ret
	}

	return linters
}

func TestEnableAllFastAndEnableCanCoexist(t *testing.T) {
	out, exitCode := runGolangciLint(t, "--fast", "--enable-all", "--enable=typecheck")
	checkNoIssuesRun(t, out, exitCode)

	_, exitCode = runGolangciLint(t, "--enable-all", "--enable=typecheck")
	assert.Equal(t, exitcodes.Failure, exitCode)

}

func TestEnabledLinters(t *testing.T) {
	type tc struct {
		name           string
		cfg            string
		el             []string
		args           string
		noImplicitFast bool
	}

	cases := []tc{
		{
			name: "disable govet in config",
			cfg: `
			linters:
				disable:
					- govet
			`,
			el: getEnabledByDefaultFastLintersExcept("govet"),
		},
		{
			name: "enable golint in config",
			cfg: `
			linters:
				enable:
					- golint
			`,
			el: getEnabledByDefaultFastLintersWith("golint"),
		},
		{
			name: "disable govet in cmd",
			args: "-Dgovet",
			el:   getEnabledByDefaultFastLintersExcept("govet"),
		},
		{
			name: "enable gofmt in cmd and enable golint in config",
			args: "-Egofmt",
			cfg: `
			linters:
				enable:
					- golint
			`,
			el: getEnabledByDefaultFastLintersWith("golint", "gofmt"),
		},
		{
			name: "fast option in config",
			cfg: `
			linters:
				fast: true
			`,
			el:             getEnabledByDefaultFastLintersWith(),
			noImplicitFast: true,
		},
		{
			name: "explicitly unset fast option in config",
			cfg: `
			linters:
				fast: false
			`,
			el:             getEnabledByDefaultLinters(),
			noImplicitFast: true,
		},
		{
			name:           "set fast option in command-line",
			args:           "--fast",
			el:             getEnabledByDefaultFastLintersWith(),
			noImplicitFast: true,
		},
		{
			name: "fast option in command-line has higher priority to enable",
			cfg: `
			linters:
				fast: false
			`,
			args:           "--fast",
			el:             getEnabledByDefaultFastLintersWith(),
			noImplicitFast: true,
		},
		{
			name: "fast option in command-line has higher priority to disable",
			cfg: `
			linters:
				fast: true
			`,
			args:           "--fast=false",
			el:             getEnabledByDefaultLinters(),
			noImplicitFast: true,
		},
		{
			name:           "fast option combined with enable and enable-all",
			args:           "--enable-all --fast --enable=megacheck",
			el:             getAllFastLintersWith("megacheck"),
			noImplicitFast: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runArgs := []string{"-v"}
			if !c.noImplicitFast {
				runArgs = append(runArgs, "--fast")
			}
			if c.args != "" {
				runArgs = append(runArgs, strings.Split(c.args, " ")...)
			}
			out := runGolangciLintWithYamlConfig(t, c.cfg, runArgs...)
			el := mergeMegacheck(c.el)
			sort.StringSlice(el).Sort()
			expectedLine := fmt.Sprintf("Active %d linters: [%s]", len(el), strings.Join(el, " "))
			assert.Contains(t, out, expectedLine)
		})
	}
}

func TestEnabledPresetsAreNotDuplicated(t *testing.T) {
	out, _ := runGolangciLint(t, "--no-config", "-v", "-p", "style,bugs")
	assert.Contains(t, out, "Active presets: [bugs style]")
}

func TestDisallowedOptionsInConfig(t *testing.T) {
	type tc struct {
		cfg    string
		option string
	}

	cases := []tc{
		{
			cfg: `
				ruN:
					Args:
						- 1
			`,
		},
		{
			cfg: `
				run:
					CPUProfilePath: path
			`,
			option: "--cpu-profile-path=path",
		},
		{
			cfg: `
				run:
					MemProfilePath: path
			`,
			option: "--mem-profile-path=path",
		},
		{
			cfg: `
				run:
					Verbose: true
			`,
			option: "-v",
		},
	}

	for _, c := range cases {
		// Run with disallowed option set only in config
		_, ec := runGolangciLintWithYamlConfigWithCode(t, c.cfg)
		assert.Equal(t, exitcodes.Failure, ec)

		if c.option == "" {
			continue
		}

		args := []string{c.option, "--fast"}

		// Run with disallowed option set only in command-line
		_, ec = runGolangciLint(t, args...)
		assert.Equal(t, exitcodes.Success, ec)

		// Run with disallowed option set both in command-line and in config
		_, ec = runGolangciLintWithYamlConfigWithCode(t, c.cfg, args...)
		assert.Equal(t, exitcodes.Failure, ec)
	}
}

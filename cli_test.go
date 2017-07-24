package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/m4i/ssmenv/lib"
	"github.com/mattn/go-shellwords"
)

var region string
var debug bool
var svc *ssm.SSM

var initialParams = map[string][]*ssm.Parameter{
	"/": {
		_p("foo", "String", "v1"),
	},
	"/foo": {
		_p("/foo/bar", "String", "v1"),
		_p("/foo/bar/baz", "String", "v2"),
	},
	"/secure": {
		_p("/secure/password", "SecureString", "pwd"),
	},
	"/symbol": {
		_p("/symbol/fo_o", "String", "v1"),
		_p("/symbol/ba.r", "String", "v2"),
		_p("/symbol/ba-z", "String", "v3"),
	},
	"/capital": {
		_p("/capital/foo", "String", "v1"),
		_p("/capital/FOO", "String", "v2"),
		_p("/capital/Foo", "String", "v3"),
		_p("/capital/foO", "String", "v4"),
	},
	"/gt10": {
		_p("/gt10/p00", "String", "v00"),
		_p("/gt10/p01", "String", "v01"),
		_p("/gt10/p02", "String", "v02"),
		_p("/gt10/p03", "String", "v03"),
		_p("/gt10/p04", "String", "v04"),
		_p("/gt10/p05", "String", "v05"),
		_p("/gt10/p06", "String", "v06"),
		_p("/gt10/p07", "String", "v07"),
		_p("/gt10/p08", "String", "v08"),
		_p("/gt10/p09", "String", "v09"),
		_p("/gt10/p10", "String", "v10"),
	},
	"/json": {
		_p("/json/HasQuote", "String", `x"x`),
		_p("/json/Quoted", "String", `"x"`),
		_p("/json/Newline", "String", "x\nx"),
		_p("/json/Tab", "String", "x\tx"),
	},
	"/exc": {
		_p("/exc/Common/KEY", "String", "v1"),
		_p("/exc/AppA/KEY", "String", "v2"),
		_p("/exc/AppB/KEY", "String", "v3"),
	},
	"/rpl": {
		_p("/rpl/foo", "String", "v1"),
		_p("/rpl/bar", "String", "v2"),
		_p("/rpl/baz/foo", "String", "v3"),
		_p("/rpl/baz/bar", "String", "v4"),
	},
	"/empty": {},
}

func TestMain(m *testing.M) {
	lib.UseCommandInsteadOfExec = true

	// Avoid rate exceeded
	lib.MaxConnection = 2

	region = os.Getenv("SSMENV_TEST_REGION")
	if region == "" {
		fmt.Fprintln(os.Stderr, "$SSMENV_TEST_REGION is required. All parameters in this region will be deleted.")
		os.Exit(1)
	}

	debug = os.Getenv("SSMENV_TEST_DEBUG") == "1"

	svc = ssm.New(newSession(region, debug))

	os.Exit(m.Run())
}

func TestCLI_Run_root(t *testing.T) {
	testHelp(t, "ssmenv")
}

func TestCLI_Run_rootHelp(t *testing.T) {
	testHelp(t, "ssmenv --help")
}

func testHelp(t *testing.T, command string) {
	out, err := _runOut(command)
	if err != nil {
		t.Fatalf("err must be nil: %v", err)
	}
	long := (CLI{}).newRootCmd().Long
	if !strings.HasPrefix(out, long) {
		t.Errorf("\ngot:\n%v\nwant:\n%v\n...", out, long)
	}
}

func ExampleCLI_Run_rootVersion() {
	version = "vX.Y.Z"
	_run("ssmenv --version")
	// Output:
	// vX.Y.Z
}

func ExampleCLI_Run_execRoot() {
	_reset("/")
	_run("ssmenv exec env" + _unsetEnviron())
	_run("ssmenv exec --path / env" + _unsetEnviron())
	// Output:
	// foo=v1
	// foo=v1
}

func ExampleCLI_Run_getRoot() {
	_reset("/")
	_run("ssmenv get")
	_run("ssmenv get --path /")
	// Output:
	// /foo=v1
	// foo=v1
}

func ExampleCLI_Run_getRootByName() {
	_reset("/")
	_run("ssmenv get foo")
	_run("ssmenv get /foo")
	_run("ssmenv get --path / foo")
	// Output:
	// v1
	// v1
	// v1
}

func ExampleCLI_Run_setRoot() {
	_reset("/")
	_run("ssmenv set bar=v1")
	_run("ssmenv set bar=v1")
	_run("ssmenv set --path / bar=v1")
	// Output:
	// PUT /bar=v1
	// UNCHANGED /bar=v1
	// UNCHANGED /bar=v1
}

func ExampleCLI_Run_replaceRoot() {
	_reset("/")
	_run("ssmenv replace --path / bar=v1")
	_run("ssmenv replace --path / bar=v1")
	// Unordered output:
	// PUT /bar=v1
	// DELETE /foo
	// UNCHANGED /bar=v1
}

func ExampleCLI_Run_execWithPath() {
	_reset("/foo")
	_run("ssmenv exec --path /foo env" + _unsetEnviron())
	// Output:
	// bar=v1
}

func ExampleCLI_Run_getWithPath() {
	_reset("/foo")
	_run("ssmenv get --path /foo")
	// Output:
	// bar=v1
}

func ExampleCLI_Run_execWithPathAndRecursive() {
	_reset("/foo")
	_run("ssmenv exec --path /foo --recursive env" + _unsetEnviron())
	// Unordered output:
	// bar=v1
	// baz=v2
}

func ExampleCLI_Run_getWithPathAndRecursive() {
	_reset("/foo")
	_run("ssmenv get --path /foo --recursive")
	// Unordered output:
	// bar=v1
	// bar/baz=v2
}

func ExampleCLI_Run_getWithPathAndRecursiveAndExport() {
	_reset("/foo")
	_run("ssmenv get --path /foo --recursive --export")
	// Unordered output:
	// export bar=v1
	// export baz=v2
}

func ExampleCLI_Run_execSecure() {
	_reset("/secure")
	_run("ssmenv exec --path /secure env" + _unsetEnviron())
	// Output:
	// password=pwd
}

func ExampleCLI_Run_getSecure() {
	_reset("/secure")
	_run("ssmenv get --path /secure")
	// Output:
	// password@=pwd
}

func ExampleCLI_Run_getSecureExport() {
	_reset("/secure")
	_run("ssmenv get --path /secure --export")
	// Output:
	// export password=pwd
}

func ExampleCLI_Run_setSecure() {
	_reset("/empty")
	_run("ssmenv set --path /empty password@=pwd")
	_run("ssmenv set --path /empty password@=pwd")
	// Output:
	// PUT /empty/password@=****************
	// UNCHANGED /empty/password@=****************
}

func ExampleCLI_Run_execSymbol() {
	_reset("/symbol")
	_run("ssmenv exec --path /symbol env" + _unsetEnviron())
	// Unordered output:
	// fo_o=v1
	// ba.r=v2
	// ba-z=v3
}

func ExampleCLI_Run_getSymbol() {
	_reset("/symbol")
	_run("ssmenv get --path /symbol")
	// Unordered output:
	// fo_o=v1
	// ba.r=v2
	// ba-z=v3
}

func ExampleCLI_Run_getSymbolExport() {
	_reset("/symbol")
	_run("ssmenv get --path /symbol --export")
	// Unordered output:
	// export fo_o=v1
	// export ba_r=v2
	// export ba_z=v3
}

func ExampleCLI_Run_execCapital() {
	_reset("/capital")
	_run("ssmenv exec --path /capital env" + _unsetEnviron())
	// Unordered output:
	// foo=v1
	// FOO=v2
	// Foo=v3
	// foO=v4
}

func ExampleCLI_Run_getGreaterThan10() {
	_reset("/gt10")
	_run("ssmenv get --path /gt10")
	// Unordered output:
	// p00=v00
	// p01=v01
	// p02=v02
	// p03=v03
	// p04=v04
	// p05=v05
	// p06=v06
	// p07=v07
	// p08=v08
	// p09=v09
	// p10=v10
}

func ExampleCLI_Run_execJSON() {
	_reset("/json")
	_run("ssmenv exec --path /json env" + _unsetEnviron())
	// Unordered output:
	// HasQuote=x"x
	// Quoted="\"x\""
	// Newline="x\nx"
	// Tab="x\tx"
}

func ExampleCLI_Run_getJSON() {
	_reset("/json")
	_run("ssmenv get --path /json")
	// Unordered output:
	// HasQuote=x"x
	// Quoted="\"x\""
	// Newline="x\nx"
	// Tab="x\tx"
}

func ExampleCLI_Run_getJSONByName() {
	_reset("/json")
	_run("ssmenv get /json/HasQuote")
	fmt.Println("----")
	_run("ssmenv get /json/Quoted")
	fmt.Println("----")
	_run("ssmenv get /json/Newline")
	fmt.Println("----")
	_run("ssmenv get /json/Tab")
	// Output:
	// x"x
	// ----
	// "x"
	// ----
	// x
	// x
	// ----
	// x	x
}

func TestCLI_Run_setJSON(t *testing.T) {
	_reset("/empty")

	out, err := _runOut(`ssmenv set --path /empty HasQuote='x"x' Quoted='"\"x\""' Newline='"x\nx"' Tab='"x\tx"'`)
	if err != nil {
		t.Fatalf("err must be nil: %v", err)
	}
	want := `PUT /empty/HasQuote=x"x
PUT /empty/Quoted="\"x\""
PUT /empty/Newline="x\nx"
PUT /empty/Tab="x\tx"
`
	if !_unorderdMatch(out, want) {
		t.Fatalf("\ngot:\n%v\nwant:\n%v", out, want)
	}

	wants := []struct{ Name, Value string }{
		{"/empty/HasQuote", `x"x`},
		{"/empty/Quoted", `"x"`},
		{"/empty/Newline", "x\nx"},
		{"/empty/Tab", "x\tx"},
	}
	paramsByName := _get("/empty")
	for _, want := range wants {
		if *paramsByName[want.Name].Value != want.Value {
			t.Errorf("got: %#v, want: %#v", *paramsByName[want.Name].Value, want.Value)
		}
	}
}

func ExampleCLI_Run_execWithPaths() {
	_reset("/exc")
	_run("ssmenv exec --paths /exc/Common,/exc/AppA env" + _unsetEnviron())
	_run("ssmenv exec --paths /exc/AppA,/exc/Common env" + _unsetEnviron())
	// Output:
	// KEY=v2
	// KEY=v1
}

func ExampleCLI_Run_setWithoutPath() {
	_reset("/empty")
	_run("ssmenv set /empty/foo=v1 /empty/bar/baz=v2")
	for _, p := range _get("/empty") {
		fmt.Println(*p.Name, *p.Value)
	}
	// Unordered output:
	// PUT /empty/foo=v1
	// PUT /empty/bar/baz=v2
	// /empty/foo v1
	// /empty/bar/baz v2
}

func ExampleCLI_Run_setWithPath() {
	_reset("/empty")
	_run("ssmenv set --path /empty foo=v1 bar/baz=v2")
	for _, p := range _get("/empty") {
		fmt.Println(*p.Name, *p.Value)
	}
	// Unordered output:
	// PUT /empty/foo=v1
	// PUT /empty/bar/baz=v2
	// /empty/foo v1
	// /empty/bar/baz v2
}

func ExampleCLI_Run_replace() {
	_reset("/rpl")
	_run("ssmenv replace --path /rpl foo=n1 qux=n2")
	for _, p := range _get("/rpl") {
		fmt.Println(*p.Name, *p.Value)
	}
	// Unordered output:
	// PUT /rpl/foo=n1
	// PUT /rpl/qux=n2
	// DELETE /rpl/bar
	// /rpl/foo n1
	// /rpl/qux n2
	// /rpl/baz/bar v4
	// /rpl/baz/foo v3
}

func ExampleCLI_Run_replaceWithRecursive() {
	_reset("/rpl")
	_run("ssmenv replace --path /rpl --recursive foo=n1 qux=n2 baz/foo=n3 baz/qux=n4")
	for _, p := range _get("/rpl") {
		fmt.Println(*p.Name, *p.Value)
	}
	// Unordered output:
	// PUT /rpl/foo=n1
	// PUT /rpl/qux=n2
	// PUT /rpl/baz/foo=n3
	// PUT /rpl/baz/qux=n4
	// DELETE /rpl/bar
	// DELETE /rpl/baz/bar
	// /rpl/foo n1
	// /rpl/qux n2
	// /rpl/baz/foo n3
	// /rpl/baz/qux n4
}

func ExampleCLI_Run_stdin() {
	_reset("/empty")
	stdin := `
# comment lines begin with #
foo=v1
# skip blank lines

    # trim spaces
    bar=v2
	`
	_runIn("ssmenv set --path /empty", stdin)
	_runIn("ssmenv replace --path /empty", stdin)
	for _, p := range _get("/empty") {
		fmt.Println(*p.Name, *p.Value)
	}
	// Unordered output:
	// PUT /empty/foo=v1
	// PUT /empty/bar=v2
	// UNCHANGED /empty/foo=v1
	// UNCHANGED /empty/bar=v2
	// /empty/foo v1
	// /empty/bar v2
}

func TestCLI_Run_execErrPathAndPaths(t *testing.T) {
	testError(t, "ssmenv exec --path /x1 --paths /x2 env", ErrPathAndPaths)
}

func TestCLI_Run_execErrRequireCommand(t *testing.T) {
	testError(t, "ssmenv exec", lib.ErrRequireCommand)
}

func TestCLI_Run_execErrInvalidPath(t *testing.T) {
	testError(t, "ssmenv exec --path x1 env", lib.ErrInvalidPath{Path: "x1"})
}

func TestCLI_Run_getErrInvalidName(t *testing.T) {
	testError(t, "ssmenv get foo:bar", lib.ErrInvalidName{Name: "foo:bar"})
}

func TestCLI_Run_getErrRecursiveWithName(t *testing.T) {
	testError(t, "ssmenv get --recursive foo", ErrRecursiveWithName)
}

func TestCLI_Run_getErrExportWithName(t *testing.T) {
	testError(t, "ssmenv get --export foo", ErrExportWithName)
}

func TestCLI_Run_getErrTooManyArguments(t *testing.T) {
	testError(t, "ssmenv get x1 x2", ErrTooManyArguments)
}

func TestCLI_Run_setErrRequireNameAndValue(t *testing.T) {
	testError(t, "ssmenv set", lib.ErrRequireNameAndValue)
}

func TestCLI_Run_replaceErrRequireNameAndValue(t *testing.T) {
	testError(t, "ssmenv replace --path /x", lib.ErrRequireNameAndValue)
}

func TestCLI_Run_setErrInvalidExpression(t *testing.T) {
	testError(t, "ssmenv set foo", lib.ErrInvalidExpression{Expr: "foo"})
}

func TestCLI_Run_replaceErrRequirePath(t *testing.T) {
	testError(t, "ssmenv replace /x=v1", lib.ErrRequirePath)
}

func TestCLI_Run_getErrAbsNameWithPath(t *testing.T) {
	testError(t, "ssmenv get --path /x1 /x2", lib.ErrAbsNameWithPath{Path: "/x1", Name: "/x2"})
}

func TestCLI_Run_setErrAbsNameWithPath(t *testing.T) {
	testError(t, "ssmenv set --path /x1 /x2=v1", lib.ErrAbsNameWithPath{Path: "/x1", Name: "/x2"})
}

func TestCLI_Run_replaceErrAbsNameWithPath(t *testing.T) {
	testError(
		t,
		"ssmenv replace --path /x1 --recursive /x2=v1",
		lib.ErrAbsNameWithPath{Path: "/x1", Name: "/x2"},
	)
}

func TestCLI_Run_replaceErrSlashWithoutRecursive(t *testing.T) {
	testError(t, "ssmenv replace --path /x foo/bar=v1", lib.ErrSlashWithoutRecursive{Expr: "foo/bar=v1"})
}

func testError(t *testing.T, command string, want error) {
	_, err := _runOut(command)
	if err != want {
		t.Errorf("\n got: %v\nwant: %v", err, want)
	}
}

func TestCLI_Run_setErrUnmarshal(t *testing.T) {
	_, err := _runOut(`ssmenv set 'foo="'`)
	if _, ok := err.(lib.ErrUnmarshal); !ok {
		t.Errorf("got: %T (%v), want: ErrUnmarshal", err, err)
	}
}

func _run(command string) {
	panicIfError((CLI{}).Run(_parseCommand(command)))
}

func _runOut(command string) (string, error) {
	w := new(bytes.Buffer)
	if err := (CLI{output: w}).Run(_parseCommand(command)); err != nil {
		return "", err
	}
	return w.String(), nil
}

func _runIn(command string, stdin string) {
	errCh := make(chan error, 1)
	r, w := io.Pipe()
	go func() {
		errCh <- (CLI{input: r}).Run(_parseCommand(command))
	}()
	fmt.Fprintln(w, stdin)
	panicIfError(w.Close())
	panicIfError(<-errCh)
}

func _parseCommand(command string) []string {
	args, err := shellwords.Parse(command)
	panicIfError(err)

	if args[0] != "ssmenv" {
		panic("LogicFailure")
	}
	args = args[1:]

	if debug {
		args = append([]string{"--debug"}, args...)
	}
	return append([]string{"ssmenv", "--region", region}, args...)
}

func _get(path string) map[string]*ssm.Parameter {
	params, err := lib.GetParametersByPath(svc, path, true)
	panicIfError(err)

	paramsByName := make(map[string]*ssm.Parameter)
	for _, param := range params {
		paramsByName[*param.Name] = param
	}
	return paramsByName
}

func _reset(path string) {
	params, ok := initialParams[path]
	if !ok {
		panic("LogicFailure")
	}

	recursive := path != ""

	panicIfError(lib.ReplaceParameters(svc, path, recursive, params, nil))
}

func _p(name, _type, value string) *ssm.Parameter {
	return &ssm.Parameter{
		Name:  &name,
		Type:  &_type,
		Value: &value,
	}
}

// nolint: unparam
func _unorderdMatch(s1, s2 string) bool {
	lines1 := strings.Split(s1, "\n")
	lines2 := strings.Split(s2, "\n")
	sort.Strings(lines1)
	sort.Strings(lines2)
	return strings.Join(lines1, "\n") == strings.Join(lines2, "\n")
}

func _unsetEnviron() string {
	unset := ""
	for _, env := range os.Environ() {
		unset += " -u " + strings.Split(env, "=")[0]
	}
	return unset
}

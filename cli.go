package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/m4i/ssmenv/lib"
	"github.com/m4i/ssmenv/stderrlogger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Errors with a fixed message.
var (
	ErrPathAndPaths      = errors.New("--path and --paths can not be given at the same time")
	ErrRecursiveWithName = errors.New("--recursive can not be used with a name")
	ErrExportWithName    = errors.New("--export can not be used with a name")
	ErrTooManyArguments  = errors.New("too many arguments")
)

// A CLI is the ssmenv command line interface.
type CLI struct {
	input  io.Reader
	output io.Writer
}

// Run runs the ssmenv command.
func (c CLI) Run(args []string) error {
	cmd := c.newCmd()
	cmd.SetArgs(args[1:])
	if c.output != nil {
		cmd.SetOutput(c.output)
	}
	return cmd.Execute()
}

func (c CLI) in() io.Reader {
	if c.input == nil {
		return os.Stdin
	}
	return c.input
}

func (c CLI) out() io.Writer {
	if c.output == nil {
		return os.Stdout
	}
	return c.output
}

func (c CLI) newCmd() *cobra.Command {
	cmd := c.newRootCmd()
	cmd.AddCommand(c.newExecCmd())
	cmd.AddCommand(c.newGetCmd())
	cmd.AddCommand(c.newSetCmd())
	cmd.AddCommand(c.newReplaceCmd())
	return cmd
}

func (c CLI) newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssmenv",
		Short: "A tool to use Amazon EC2 Systems Manager (SSM) Parameter Store as environment variables",
		Long:  `ssmenv is a tool to use Amazon EC2 Systems Manager (SSM) Parameter Store as environment variables.`,
		RunE:  c.runRoot,
	}
	cmd.PersistentFlags().String("region", "", "The region to use. Overrides config/env settings.")
	cmd.PersistentFlags().String("path", "", "The hierarchy for the parameter.")
	cmd.PersistentFlags().Bool("debug", false, "debug mode")
	panicIfError(cmd.PersistentFlags().MarkHidden("debug"))
	cmd.Flags().Bool("version", false, "show version")
	return cmd
}

func (c CLI) newExecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec [flags] command ...",
		Short: "Exec the command with environment variables",
		Long:  `Exec the command with environment variables.`,
		RunE:  c.runExec,
	}
	cmd.Flags().StringSlice("paths", []string{}, "Comma separated multiple paths.")
	cmd.Flags().Bool("recursive", false, "Retrieve all parameters within a hierarchy.")
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func (c CLI) newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [flags] [name]",
		Short: "Print parameters",
		Long:  `Print parameters.`,
		RunE:  c.runGet,
	}
	cmd.Flags().Bool("recursive", false, "Retrieve all parameters within a hierarchy.")
	cmd.Flags().Bool("export", false, "Print export statements for shells.")
	return cmd
}

func (c CLI) newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [flags] name=value ...",
		Short: "Set parameters",
		Long:  `Set parameters.`,
		RunE:  c.runSet,
	}
	return cmd
}

func (c CLI) newReplaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "replace [flags] name=value ...",
		Short: "Replace all the parameters of the given path",
		Long:  `Replace all the parameters of the given path.`,
		RunE:  c.runReplace,
	}
	cmd.Flags().Bool("recursive", false, "Replace all parameters within a hierarchy.")
	return cmd
}

func (c CLI) runRoot(cmd *cobra.Command, _ []string) error {
	versionFlag, err := cmd.Flags().GetBool("version")
	if err != nil {
		return err
	}

	if versionFlag {
		fmt.Fprintln(c.out(), version)
		return nil
	}

	return pflag.ErrHelp
}

func (c CLI) runExec(cmd *cobra.Command, args []string) error {
	svc, path, err := getPersistentFlags(cmd)
	if err != nil {
		return err
	}

	paths, err := cmd.Flags().GetStringSlice("paths")
	if err != nil {
		return err
	}

	if path != "" {
		if len(paths) == 0 {
			paths = append(paths, path)
		} else {
			return ErrPathAndPaths
		}
	}

	recursive, err := cmd.Flags().GetBool("recursive")
	if err != nil {
		return err
	}

	cmd.SilenceUsage = true
	return lib.Exec(svc, paths, recursive, args)
}

func (c CLI) runGet(cmd *cobra.Command, args []string) error {
	svc, path, err := getPersistentFlags(cmd)
	if err != nil {
		return err
	}

	recursive, err := cmd.Flags().GetBool("recursive")
	if err != nil {
		return err
	}

	exportFlag, err := cmd.Flags().GetBool("export")
	if err != nil {
		return err
	}

	switch len(args) {
	case 0:
		cmd.SilenceUsage = true
		return lib.GetByPath(c.out(), svc, path, recursive, exportFlag)
	case 1:
		if recursive {
			return ErrRecursiveWithName
		}
		if exportFlag {
			return ErrExportWithName
		}
		cmd.SilenceUsage = true
		return lib.GetByName(c.out(), svc, path, args[0])
	default:
		return ErrTooManyArguments
	}
}

func (c CLI) runSet(cmd *cobra.Command, args []string) error {
	svc, path, err := getPersistentFlags(cmd)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		args, err = readExprs(c.in())
		if err != nil {
			return err
		}
	}

	cmd.SilenceUsage = true
	return lib.Set(c.out(), svc, path, args)
}

func (c CLI) runReplace(cmd *cobra.Command, args []string) error {
	svc, path, err := getPersistentFlags(cmd)
	if err != nil {
		return err
	}

	recursive, err := cmd.Flags().GetBool("recursive")
	if err != nil {
		return err
	}

	if len(args) == 0 {
		args, err = readExprs(c.in())
		if err != nil {
			return err
		}
	}

	cmd.SilenceUsage = true
	return lib.Replace(c.out(), svc, path, recursive, args)
}

func getPersistentFlags(cmd *cobra.Command) (*ssm.SSM, string, error) {
	region, err := cmd.Flags().GetString("region")
	if err != nil {
		return nil, "", err
	}

	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return nil, "", err
	}

	svc := ssm.New(newSession(region, debug))

	path, err := cmd.Flags().GetString("path")
	if err != nil {
		return nil, "", err
	}

	return svc, path, nil
}

func newSession(region string, debug bool) *session.Session {
	config := aws.NewConfig()

	if region != "" {
		config.WithRegion(region)
	}

	if debug {
		config.WithLogger(stderrlogger.New())
		config.WithLogLevel(aws.LogDebugWithHTTPBody | aws.LogDebugWithRequestErrors | aws.LogDebugWithRequestRetries)
	}

	return session.Must(session.NewSession(config))
}

func readExprs(r io.Reader) ([]string, error) {
	var exprs []string
	errCh := make(chan error)

	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			exprs = append(exprs, line)
		}
		errCh <- scanner.Err()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
		return exprs, nil
	case <-time.After(time.Second):
		return []string{}, nil
	}
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}

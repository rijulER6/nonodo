// Copyright (c) Gabriel de Quadros Ligneul
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

// This package contains the main function that executes the nonodo command.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/calindra/nonodo/internal/nonodo"
	"github.com/carlmjohnson/versioninfo"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var startupMessage = `
Http Rollups for development started at http://localhost:5004
GraphQL running at http://localhost:8080/graphql
Inspect running at http://localhost:8080/inspect/
Press Ctrl+C to stop the node
`
var cmd = &cobra.Command{
	Use:     "nonodo [flags] [-- application [args]...]",
	Short:   "nonodo is a development node for Cartesi Rollups",
	Run:     run,
	Version: versioninfo.Short(),
}

var addressBookCmd = &cobra.Command{
	Use:   "address-book",
	Short: "Show address book",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Read json and print address...")
		x := nonodo.NewNonodoOpts().InputBoxAddress
		fmt.Println("InputBoxAddress", x)
		y := opts.InputBoxAddress
		fmt.Println("InputBoxAddress", y)
	},
}

var (
	debug bool
	color bool
	opts  = nonodo.NewNonodoOpts()
)

func init() {
	// anvil-*
	cmd.Flags().StringVar(&opts.AnvilAddress, "anvil-address", opts.AnvilAddress,
		"HTTP address used by Anvil")
	cmd.Flags().IntVar(&opts.AnvilPort, "anvil-port", opts.AnvilPort,
		"HTTP port used by Anvil")
	cmd.Flags().BoolVar(&opts.AnvilVerbose, "anvil-verbose", opts.AnvilVerbose,
		"If set, prints Anvil's output")

	// contracts-*
	cmd.Flags().StringVar(&opts.ApplicationAddress, "contracts-application-address",
		opts.ApplicationAddress, "Application contract address")
	cmd.Flags().StringVar(&opts.InputBoxAddress, "contracts-input-box-address",
		opts.InputBoxAddress, "InputBox contract address")
	cmd.Flags().Uint64Var(&opts.InputBoxBlock, "contracts-input-box-block",
		opts.InputBoxBlock, "InputBox deployment block number")

	// enable-*
	cmd.Flags().BoolVarP(&debug, "enable-debug", "d", false, "If set, enable debug output")
	cmd.Flags().BoolVar(&color, "enable-color", true, "If set, enables logs color")
	cmd.Flags().BoolVar(&opts.EnableEcho, "enable-echo", opts.EnableEcho,
		"If set, nonodo starts a built-in echo application")

	// disable-*
	cmd.Flags().BoolVar(&opts.DisableDevnet, "disable-devnet", opts.DisableDevnet,
		"If set, nonodo won't start a local devnet")
	cmd.Flags().BoolVar(&opts.DisableAdvance, "disable-advance", opts.DisableAdvance,
		"If set, nonodo won't start the inputter to get inputs from the local chain")

	// http-*
	cmd.Flags().StringVar(&opts.HttpAddress, "http-address", opts.HttpAddress,
		"HTTP address used by nonodo to serve its APIs")
	cmd.Flags().IntVar(&opts.HttpPort, "http-port", opts.HttpPort,
		"HTTP port used by nonodo to serve its APIs")

	// rpc-url
	cmd.Flags().StringVar(&opts.RpcUrl, "rpc-url", opts.RpcUrl,
		"If set, nonodo connects to this url instead of setting up Anvil")
}

func run(cmd *cobra.Command, args []string) {
	startTime := time.Now()

	// setup log
	logOpts := new(tint.Options)
	if debug {
		logOpts.Level = slog.LevelDebug
	}
	logOpts.AddSource = debug
	logOpts.NoColor = !color || !isatty.IsTerminal(os.Stdout.Fd())
	logOpts.TimeFormat = "[15:04:05.000]"
	handler := tint.NewHandler(os.Stdout, logOpts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// check args
	checkEthAddress(cmd, "address-input-box")
	checkEthAddress(cmd, "address-application")
	if opts.AnvilPort == 0 {
		exitf("--anvil-port cannot be 0")
	}
	if cmd.Flags().Changed("rpc-url") && !cmd.Flags().Changed("contracts-input-box-block") {
		exitf("must set --contracts-input-box-block when setting --rpc-url")
	}
	if opts.EnableEcho && len(args) > 0 {
		exitf("can't use built-in echo with custom application")
	}
	opts.ApplicationArgs = args

	// handle signals with notify context
	ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// start nonodo
	ready := make(chan struct{}, 1)
	go func() {
		select {
		case <-ready:
			fmt.Println(startupMessage)
			slog.Info("nonodo: ready", "after", time.Since(startTime))
		case <-ctx.Done():
		}
	}()
	err := nonodo.NewSupervisor(opts).Start(ctx, ready)
	cobra.CheckErr(err)
}

func main() {
	cmd.AddCommand(addressBookCmd)
	cobra.CheckErr(cmd.Execute())
}

func exitf(format string, args ...any) {
	err := fmt.Sprintf(format, args...)
	slog.Error("configuration error", "error", err)
	os.Exit(1)
}

func checkEthAddress(cmd *cobra.Command, varName string) {
	if cmd.Flags().Changed(varName) {
		value, err := cmd.Flags().GetString(varName)
		cobra.CheckErr(err)
		bytes, err := hexutil.Decode(value)
		if err != nil {
			exitf("invalid address for --%v: %v", varName, err)
		}
		if len(bytes) != common.AddressLength {
			exitf("invalid address for --%v: wrong length", varName)
		}
	}
}

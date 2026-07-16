package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/risulongmemory/archive-center-go/internal/packageupdate"
)

type errorResult struct {
	ContractVersion string `json:"contract_version"`
	Action          string `json:"action"`
	Status          string `json:"status"`
	Code            string `json:"code"`
	Message         string `json:"message"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return writeFailure("", "usage", fmt.Errorf("command is required"))
	}
	action := args[0]
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", "", "Archive Center package root")
	if err := flags.Parse(args[1:]); err != nil {
		return writeFailure(action, "usage", err)
	}
	if flags.NArg() != 0 || *root == "" {
		return writeFailure(action, "usage", fmt.Errorf("--root is required and positional arguments are not accepted"))
	}
	var (
		result packageupdate.Result
		err    error
	)
	switch action {
	case "apply-pending":
		result, err = packageupdate.ApplyPending(*root)
	case "commit":
		result, err = packageupdate.Commit(*root)
	case "rollback":
		result, err = packageupdate.Rollback(*root)
	case "status":
		result, err = packageupdate.Status(*root)
	default:
		return writeFailure(action, "usage", fmt.Errorf("unsupported command %q", action))
	}
	if err != nil {
		code := "update_failed"
		var updateError *packageupdate.UpdateError
		if errors.As(err, &updateError) {
			code = updateError.Code
		}
		return writeFailure(action, code, err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return writeFailure(action, "output_failed", err)
	}
	return 0
}

func writeFailure(action, code string, err error) int {
	_ = json.NewEncoder(os.Stderr).Encode(errorResult{
		ContractVersion: packageupdate.ResultContract,
		Action:          action,
		Status:          "error",
		Code:            code,
		Message:         err.Error(),
	})
	return 1
}

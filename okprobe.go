package main

import (
	"fmt"
	"os"
	"os/exec"
)

/* Give the baseURL of the okex or okcatbox server, an OKProbe command, and file names containing the server credentials for read, read-trade, and read-withdraw, run a series of tests of the given OKProbe command.
 */
func testOKProbe(baseURL, command, queryString, read, trade, withdraw string) {

	test([]string{command, "--baseURL", baseURL, "--credentialsFile", read, "--makeErrorsCredentials", "--makeErrorsParams"})
	test([]string{command, "--baseURL", baseURL, "--credentialsFile", read, "--makeErrorsWrongCredentialsType"})
	test([]string{command, "--baseURL", baseURL, "--credentialsFile", trade, "--makeErrorsWrongCredentialsType"})
	test([]string{command, "--baseURL", baseURL, "--credentialsFile", withdraw, "--makeErrorsWrongCredentialsType"})
	test([]string{command, "--baseURL", baseURL, "--credentialsFile", read, "--queryString", queryString, "--forReal"})
	fmt.Printf("test okprobe %s success\n", command)
}

func test(args []string) {

	cmd := exec.Command("okprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("okprobe: out=%s, err=%v\nargs=%v\n", string(out), err, args)
		os.Exit(1)
	}
}

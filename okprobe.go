package main

import (
	"fmt"
	"os"
	"os/exec"
)

/* Give the baseURL of the okex or okcatbox server, an OKProbe command, and file names containing the server credentials for read, read-trade, and read-withdraw, run a series of tests of the given OKProbe command.
 */
func testOKProbe(baseURL, command, read, trade, withdraw string) {

	test([]string{command, "--baseURL", baseURL, "--credentialsFile", read, "--makeErrorsCredentials", "--makeErrorsParams"})
	test([]string{command, "--baseURL", baseURL, "--credentialsFile", read, "--makeErrorsWrongCredentialsType"})
	test([]string{command, "--baseURL", baseURL, "--credentialsFile", trade, "--makeErrorsWrongCredentialsType"})
	test([]string{command, "--baseURL", baseURL, "--credentialsFile", withdraw, "--makeErrorsWrongCredentialsType"})
	test([]string{command, "--baseURL", baseURL, "--credentialsFile", read, "--forReal"})
	fmt.Printf("test okprobe %s success\n", command)
}

func test(args []string) {

	_, err := exec.Command("okprobe", args...).Output()
	if err != nil {
		fmt.Printf("okprobe error %v\n", err)
		os.Exit(1)
	} else {
		//fmt.Printf("okprobe success %v\n", out)
	}
}

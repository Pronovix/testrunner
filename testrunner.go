// Copyright 2018 Pronovix
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	root    = flag.String("root", ".", "the directory where the tests are")
	command = flag.String("command", "", "command to run")
	pattern = flag.String("pattern", "Test.php$", "pattern to match test files")
	threads = flag.Int("threads", runtime.NumCPU(), "number of threads")
	timeout = flag.Int("timeout", 9, "after timeout minutes, a new line will be printed to stdout")
	verbose = flag.Bool("verbose", false, "enables verbose mode")

	split     = regexp.MustCompile(`\s+`)
	parts     []string
	success   = 0
	fail      = 0
	resultMtx sync.Mutex
)

// worker reads from the channel, runs the command and puts the output on the output channel.
func worker(path, output chan string, wg *sync.WaitGroup) {
	for filename := range path {
		maybeLog("Starting file " + filename)
		output <- run(filename, wg)
	}
}

// run runs the command with the file in path as an extra parameter.
func run(path string, wg *sync.WaitGroup) string {
	defer wg.Done()

	start := time.Now()
	cmdparts := append(parts[:], path)

	cmd := exec.Cmd{
		Path: parts[0],
		Args: cmdparts,
	}
	out, err := cmd.CombinedOutput()
	resultMtx.Lock()
	if err != nil {
		fail += 1
	} else {
		success += 1
	}
	resultMtx.Unlock()

	maybeLog("Finished file " + path)

	return fmt.Sprintf("Running %s\n\n%s\n\nElapsed: %s\n",
		strings.Join(cmdparts, " "),
		string(out),
		time.Since(start).String(),
	)
}

// printer prints out the messages coming the channel, and marks the job done in the wait group.
func printer(str chan string) {
	for {
		select {
		case output := <-str:
			if output = strings.TrimSpace(output); output != "" {
				fmt.Printf("\n%s\n\n", output)
			}
		case <-time.After(time.Duration(*timeout) * time.Minute):
			fmt.Println("")
		}
	}
}

func maybeLog(msg string) {
	if *verbose {
		fmt.Fprintln(os.Stderr, msg)
	}
}

// main funciton.
func main() {
	flag.Parse()

	if *command == "" {
		fmt.Println("no command is specified")
		os.Exit(1)
	}

	parts = split.Split(*command, -1)
	var wg sync.WaitGroup
	input := make(chan string, 128)
	output := make(chan string)

	for i := 0; i < *threads; i++ {
		go worker(input, output, &wg)
	}

	go printer(output)

	matcher, err := regexp.Compile(*pattern)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("Starting %d threads\n\n", *threads)

	start := time.Now()
	files := make(map[string]struct{})

	filepath.Walk(*root, func(path string, info os.FileInfo, err error) error {
		if matcher.MatchString(path) {
			if _, found := files[path]; !found {
				wg.Add(1)
				maybeLog("Adding file " + path)
				files[path] = struct{}{}
				input <- path
			}
		}

		return nil
	})

	wg.Wait()
	close(input)
	close(output)

	fmt.Printf("\n\nComplete runtime: %s | Success: %d Failure: %d\n\n", time.Since(start).String(), success, fail)

	if fail > 0 {
		os.Exit(1)
	}
}

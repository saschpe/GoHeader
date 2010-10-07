// Copyright 2010  The "goheader" Authors
//
// Use of this source code is governed by the Simplified BSD License
// that can be found in the LICENSE file.
//
// This software is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES
// OR CONDITIONS OF ANY KIND, either express or implied. See the License
// for more details.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
)


const COMMENT_LINE = "//!!! "

var exitCode = 0

var goBase = `// {cmd}
// MACHINE GENERATED.

package {pkg}

`

// Flags
var (
	system      = flag.String("s", "", "The operating system.")
	pkgName     = flag.String("p", "", "The name of the package.")
	listSystems = flag.Bool("l", false, "List of valid systems.")
	write       = flag.Bool("w", false,
		"If set, write its output to file.")
)


func usage() {
	fmt.Fprintf(os.Stderr, "Usage: goheader -s system -p package [path ...]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func processFile(filename string) os.Error {
	var rawOutput bytes.Buffer

	file, err := os.Open(filename, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := translateC(&rawOutput, file); err != nil {
		return err
	}

	if err := format(filename, &rawOutput); err != nil {
		return err
	}

	return nil
}

// === Main
// ===

func main() {
	validSystems := []string{"darwin", "freebsd", "linux", "windows"}
	var isSystem bool

	// === Parse the flags
	flag.Usage = usage
	flag.Parse()

	if *listSystems {
		fmt.Print("  = Systems\n\n  ")
		fmt.Println(validSystems)
		os.Exit(0)
	}
	if len(os.Args) == 1 || *system == "" || *pkgName == "" {
		usage()
	}

	*system = strings.ToLower(*system)

	for _, v := range validSystems {
		if v == *system {
			isSystem = true
			break
		}
	}
	if !isSystem {
		fmt.Fprintf(os.Stderr, "ERROR: System passed in flag 's' is invalid\n")
		os.Exit(2)
	}

	// === Update header
	cmd := strings.Join(os.Args, " ")
	goBase = strings.Replace(goBase, "{cmd}", cmd, 1)
	goBase = strings.Replace(goBase, "{pkg}", *pkgName, 1)
	// ===

	// Translate all headers passed in command line.
	for _, path := range flag.Args() {
		switch info, err := os.Stat(path); {
		case err != nil:
			reportError(err)
		case info.IsRegular():
			if err := processFile(path); err != nil {
				reportError(err)
			}
		case info.IsDirectory():
			walkDir(path)
		}
	}

	os.Exit(exitCode)
}


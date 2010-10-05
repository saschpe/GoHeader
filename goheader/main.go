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
	"bufio"
	//"bytes"
	"flag"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
)


const COMMENT_C_LINE = "//!!! "

var goHeader = `// {cmd}
// MACHINE GENERATED.

package {pkg}
`

var (
	fOS      = flag.String("s", "", "The operating system")
	fPackage = flag.String("p", "", "The name of the package")
	fListOS  = flag.Bool("ls", false, "List of valid systems")
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: goheader -s system -g package [defs.h...]\n")
	flag.PrintDefaults()
	os.Exit(1)
}


// Translates C type declaration into Go type declaration.
func translateC(fname string) os.Error {
	// === Regular expressions
	reSkip := regexp.MustCompile(`^(\n|//)`) // Empty lines and comments.

	reType := regexp.MustCompile(`^(typedef)[ \t]+(.+)[ \t]+(.+)[;](.+)?`)

	reStruct := regexp.MustCompile(`^(struct)[ \t]+(.+)[ \t]*{`)
	reStructField := regexp.MustCompile(`^[ \t]*(.+)[ \t]+(.+)[;](.+)?`)
	reStructFieldName := regexp.MustCompile(`^([^_]*_)?(.+)`)

	reDefine := regexp.MustCompile(`^[ \t]*#[ \t]*(define|DEFINE)[ \t]+([^ \t]+)[ \t]+(.+)`)
	reDefineMacro := regexp.MustCompile(`^.*(\(.*\))`)

	reSingleComment := regexp.MustCompile(`^(.+)?/\*[ \t]*(.+)[ \t]*\*/`)
	reStartMultipleComment := regexp.MustCompile(`^/\*(.+)?`)
	reMiddleMultipleComment := regexp.MustCompile(`^[ \t*]*(.+)`)
	reEndMultipleComment := regexp.MustCompile(`^(.+)?\*/`)

	// === File to read
	var isMultipleComment, isDefine, isStruct bool

	file, err := os.Open(fname, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	r := bufio.NewReader(file)

	fmt.Println(goHeader)

	for {
		line, err := r.ReadString('\n')
		if err == os.EOF {
			break
		}
		line = strings.TrimSpace(line) + "\n"
		isSingleComment := false

		// === Translate comment of single line.
		if !isMultipleComment {
			if sub := reSingleComment.FindStringSubmatch(line); sub != nil {
				isSingleComment = true
				line = "// " + sub[2] + "\n"

				if sub[1] != "" {
					line = sub[1] + line
				}
			}
		}
		if !isSingleComment && !isMultipleComment && strings.HasPrefix(line, "/*") {
			isMultipleComment = true
		}

		// === Translate comments of multiple line.
		if isMultipleComment {
			if sub := reStartMultipleComment.FindStringSubmatch(line); sub != nil {
				if sub[1] != "\n" {
					line = "// " + sub[1]
					fmt.Print(line)
				}
				continue
			}
			if sub := reEndMultipleComment.FindStringSubmatch(line); sub != nil {
				if sub[1] != "" {
					line = "// " + sub[1] + "\n"
					fmt.Print(line)
				}
				isMultipleComment = false
				continue
			}
			if sub := reMiddleMultipleComment.FindStringSubmatch(line); sub != nil {
				line = "// " + sub[1]
				fmt.Print(line)
				continue
			}
		}

		// === Translate type definitions.
		if sub := reType.FindStringSubmatch(line); sub != nil {
			gotype, ok := ctypeTogo(sub[2])
			line = fmt.Sprintf("type %s %s", sub[3], gotype)

			if sub[4] != "\n" {
				line += sub[4]
			} else {
				line += "\n"
			}
			if !ok {
				line = COMMENT_C_LINE + line
			}

			fmt.Print(line)
			continue
		}

		// === Translate defines.
		if sub := reDefine.FindStringSubmatch(line); sub != nil {
			if !isDefine {
				isDefine = true
				fmt.Print("const (\n")
			}
			line = fmt.Sprintf("%s = %s", sub[2], sub[3])

			// Removes comment (if any) to ckeck if it is a macro.
			lastField := strings.Split(sub[3], "//", -1)[0]
			if reDefineMacro.MatchString(lastField) {
				line = COMMENT_C_LINE + line
			}

			fmt.Print(line)
			continue
		}
		if isDefine && line == "\n" {
			fmt.Print(")\n\n")
			isDefine = false
			continue
		}

		// === Translate structs.
		if !isStruct {
			if sub := reStruct.FindStringSubmatch(line); sub != nil {
				isStruct = true

				if isDefine {
					fmt.Print(")\n")
					isDefine = false
				}

				fmt.Printf("type %s struct {\n", strings.Title(sub[2]))
				continue
			}
		} else {
			if sub := reStructField.FindStringSubmatch(line); sub != nil {
				// Translate the field type.
				gotype, ok := ctypeTogo(sub[1])

				// === Translate the field name.
				fieldName := reStructFieldName.FindStringSubmatch(sub[2])
				_fieldName := ""

				if fieldName[1] != "" {
					_fieldName = fieldName[2]
				} else {
					_fieldName = fieldName[0]
				}
				// ===

				line = fmt.Sprintf("%s %s %s",
					strings.Title(_fieldName), gotype, sub[3])

				// C type not found.
				if !ok {
					line = COMMENT_C_LINE + line
				}

				fmt.Print(line)
				continue
			}
			if strings.HasPrefix(line, "}") {
				fmt.Print(strings.Replace(line, ";", "", 1))
				isStruct = false
				continue
			}
		}

		// Comment another C lines.
		//if line != "\n" && !strings.HasPrefix(line, "//") {
		if !reSkip.MatchString(line) {
			line = COMMENT_C_LINE + line
		}

		fmt.Print(line)
	}

	return nil
}

// Turns a type's string from C to Go.
func ctypeTogo(ctype string) (gotype string, ok bool) {
	switch ctype {
	case "char", "signed char", "signed short int", "short int", "short":
		return "int8", true
	case "unsigned char", "unsigned short int", "unsigned short":
		return "uint8", true
	case "int", "signed int":
		return "int16", true
	case "unsigned int", "unsigned":
		return "uint16", true
	case "signed long int", "long int", "long":
		return "int32", true
	case "unsigned long int", "unsigned long":
		return "uint32", true
	case "float":
		return "float32", true
	case "double", "long double":
		return "float64", true
	}
	return ctype, false
}


func main() {
	var isValidOS bool
	validOS := []string{"darwin", "freebsd", "linux", "windows"}

	// === Parse the flags
	flag.Usage = usage
	flag.Parse()

	if *fListOS {
		fmt.Print("  = Systems\n\n  ")
		fmt.Println(validOS)
		os.Exit(0)
	}
	if len(os.Args) == 1 || *fOS == "" || *fPackage == "" {
		usage()
	}

	*fOS = strings.ToLower(*fOS)

	for _, v := range validOS {
		if v == *fOS {
			isValidOS = true
			break
		}
	}
	if !isValidOS {
		fmt.Fprintf(os.Stderr, "ERROR: System passed in flag 's' is invalid\n")
		os.Exit(1)
	}

	// === Update header
	cmd := strings.Join(os.Args, " ")
	goHeader = strings.Replace(goHeader, "{cmd}", path.Base(cmd), 1)
	goHeader = strings.Replace(goHeader, "{pkg}", *fPackage, 1)

File := "../test/header.h"

	if err := translateC(File); err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		os.Exit(1)
	}
}


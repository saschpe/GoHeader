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


const C_LINE_COMMENT = "//!!! "

var header = `// {cmd}
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


func turn(fname string) os.Error {
	reSkip := regexp.MustCompile(`^(\n|//)`) // Comments and empty lines.

	reStruct := regexp.MustCompile(`^(struct)[ \t]+(.+)[ \t]*{`)
	reStructField := regexp.MustCompile(`^[ \t]*(.+)[ \t]+(.+)[;](.+)?`)
	reStructField1 := regexp.MustCompile(`^[^_]*[_]?(.+)`)

	reDefine := regexp.MustCompile(`^[ \t]*#[ \t]*(define|DEFINE)[ \t]+([^ \t]+)[ \t]+(.+)`)
	reDefineMacro := regexp.MustCompile(`^.*(\(.*\))`)

	reSingleComment := regexp.MustCompile(`^(.+)?/\*[ \t]*(.+)[ \t]*\*/`)
	reStartMultipleComment := regexp.MustCompile(`^/\*(.+)?`)
	reMiddleMultipleComment := regexp.MustCompile(`^[ \t*]*(.+)`)
	reEndMultipleComment := regexp.MustCompile(`^(.+)?\*/`)

	file, err := os.Open(fname, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	r := bufio.NewReader(file)

	fmt.Println(header)
	var isMultipleComment, isDefine, isStruct bool

	for {
		line, err := r.ReadString('\n')
		if err == os.EOF {
			break
		}
		line = strings.TrimSpace(line) + "\n"
		isSingleComment := false

		// === Convert comment of single line.
		if !isMultipleComment {
			if fields := reSingleComment.FindStringSubmatch(line); fields != nil {
				isSingleComment = true
				line = "// " + fields[2] + "\n"

				if fields[1] != "" {
					line = fields[1] + line
				}
			}
		}

		if !isSingleComment && !isMultipleComment && strings.HasPrefix(line, "/*") {
			isMultipleComment = true
		}

		// === Convert comments of multiple line.
		if isMultipleComment {
			if fields := reStartMultipleComment.FindStringSubmatch(line); fields != nil {
				if fields[1] != "\n" {
					line = "// " + fields[1]
					fmt.Print(line)
				}
				continue
			}

			if fields := reEndMultipleComment.FindStringSubmatch(line); fields != nil {
				if fields[1] != "" {
					line = "// " + fields[1] + "\n"
					fmt.Print(line)
				}
				isMultipleComment = false
				continue
			}

			if fields := reMiddleMultipleComment.FindStringSubmatch(line); fields != nil {
				line = "// " + fields[1]
				fmt.Print(line)
				continue
			}
		}

		// === Turn defines.
		if fields := reDefine.FindStringSubmatch(line); fields != nil {
			if !isDefine {
				isDefine = true
				fmt.Print("const (\n")
			}
			line = fmt.Sprintf("%s = %s", fields[2], fields[3])

			// Removes comment (if any) to ckeck if it is a macro.
			lastField := strings.Split(fields[3], "//", -1)[0]
			if reDefineMacro.MatchString(lastField) {
				line = C_LINE_COMMENT + line
			}

			fmt.Print(line)
			continue
		}

		if isDefine && line == "\n" {
			fmt.Print(")\n\n")
			isDefine = false
			continue
		}

		// === Turn structs.
		if !isStruct {
			if fields := reStruct.FindStringSubmatch(line); fields != nil {
				isStruct = true

				if isDefine {
					fmt.Print(")\n")
					isDefine = false
				}

				fmt.Printf("type %s struct {\n", strings.Title(fields[2]))
				continue
			}
		} else {
			if fields := reStructField.FindStringSubmatch(line); fields != nil {
				// Convert the field type.
				gotype, ok := ctypeTogo(fields[1])

				// === Convert the field name.
				fieldName := reStructField1.FindStringSubmatch(fields[2])
				_fieldName := ""

				if fieldName[1] != "" {
					_fieldName = fieldName[1]
				} else {
					_fieldName = fieldName[0]
				}
				// ===

				line = fmt.Sprintf("%s %s %s",
					strings.Title(_fieldName), gotype, fields[3])

				// C type not found.
				if !ok {
					line = C_LINE_COMMENT + line
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
			line = C_LINE_COMMENT + line
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
	header = strings.Replace(header, "{cmd}", path.Base(cmd), 1)
	header = strings.Replace(header, "{pkg}", *fPackage, 1)

File := "/usr/include/asm-generic/ioctls.h"
//File := "/usr/include/asm-generic/termios.h"

	if err := turn(File); err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		os.Exit(1)
	}
}


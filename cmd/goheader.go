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
	"container/vector"
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

// Flags
var (
	system      = flag.String("s", "", "The operating system")
	pkgName     = flag.String("p", "", "The name of the package")
	listSystems = flag.Bool("l", false, "List of valid systems")
	write       = flag.Bool("w", false,
		"If set, write each input file with its output.")
)


func usage() {
	fmt.Fprintf(os.Stderr, "Usage: goheader -s system -g package [defs.h...]\n")
	flag.PrintDefaults()
	os.Exit(1)
}


// Translates C type declaration into Go type declaration.
func translateC(cHeader string) os.Error {
	// === Regular expressions
	reSkip := regexp.MustCompile(`^(\n|//)`) // Empty lines and comments.

	reType := regexp.MustCompile(`^(typedef)[ \t]+(.+)[ \t]+(.+)[;](.+)?`)

	reStruct := regexp.MustCompile(`^(struct)[ \t]+(.+)[ \t]*{`)
	reStructField := regexp.MustCompile(`^(.+)[ \t]+(.+)[;](.+)?`)
	reStructFieldName := regexp.MustCompile(`^([^_]*_)?(.+)`)

	reDefine := regexp.MustCompile(`^#[ \t]*(define|DEFINE)[ \t]+([^ \t]+)[ \t]+(.+)`)
	reDefineOnly := regexp.MustCompile(`^#[ \t]*(define|DEFINE)[ \t]+`)
	reDefineMacro := regexp.MustCompile(`^.*(\(.*\))`)

	reSingleComment := regexp.MustCompile(`^(.+)?/\*[ \t]*(.+)[ \t]*\*/`)
	reStartMultipleComment := regexp.MustCompile(`^/\*(.+)?`)
	reMiddleMultipleComment := regexp.MustCompile(`^[ \t*]*(.+)`)
	reEndMultipleComment := regexp.MustCompile(`^(.+)?\*/`)
	// ===

	// Check the extension.
	if path.Ext(cHeader) != ".h" {
		return os.NewError("error extension")
	}

	// === File to read
	var isMultipleComment, isDefineBlock, isStruct bool
	var extraType vector.StringVector // Types defined in C header.

	inFile, err := os.Open(cHeader, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer inFile.Close()

	inBuf := bufio.NewReader(inFile)

	// === File to write in the actual directory.
	var outBuf *bufio.Writer

	if *write {
		goHeader := strings.Split(path.Base(cHeader), ".h", 2)[0]
		goHeader = fmt.Sprintf("%s_%s.go", goHeader, *system)

		outFile, err := os.Open(goHeader, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer outFile.Close()

		outBuf = bufio.NewWriter(outFile)
	} else {
		outBuf = bufio.NewWriter(os.Stdout)
	}

	if _, err := outBuf.WriteString(goHeader); err != nil {
		return err
	}

	for {
		line, err := inBuf.ReadString('\n')
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
					if _, err := outBuf.WriteString(line); err != nil {
						return err
					}
				}
				continue
			}
			if sub := reEndMultipleComment.FindStringSubmatch(line); sub != nil {
				if sub[1] != "" {
					line = "// " + sub[1] + "\n"
					if _, err := outBuf.WriteString(line); err != nil {
						return err
					}
				}
				isMultipleComment = false
				continue
			}
			if sub := reMiddleMultipleComment.FindStringSubmatch(line); sub != nil {
				line = "// " + sub[1]
				if _, err := outBuf.WriteString(line); err != nil {
					return err
				}
				continue
			}
		}

		// === Translate type definitions.
		if sub := reType.FindStringSubmatch(line); sub != nil {
			// Add the new type.
			extraType.Push(sub[3])

			gotype, ok := ctypeTogo(sub[2], &extraType)
			line = fmt.Sprintf("type %s %s", sub[3], gotype)

			if sub[4] != "\n" {
				line += sub[4]
			} else {
				line += "\n"
			}
			if !ok {
				line = COMMENT_C_LINE + line
			}

			if _, err := outBuf.WriteString(line); err != nil {
				return err
			}
			continue
		}

		// === Translate defines.
		if sub := reDefine.FindStringSubmatch(line); sub != nil {
			line = fmt.Sprintf("%s = %s", sub[2], sub[3])

			if !isDefineBlock {
				// Get characters of next line.
				startNextLine, err := inBuf.Peek(10)
				if err != nil {
					return err
				}

				// Constant in single line.
				if startNextLine[0] == '\n' || !reDefineOnly.Match(startNextLine) {
					line = "const " + line
				} else {
					isDefineBlock = true
					line = "const (\n" + line
				}
			}

			// Removes comment (if any) to ckeck if it is a macro.
			lastField := strings.Split(sub[3], "//", -1)[0]
			if reDefineMacro.MatchString(lastField) {
				line = COMMENT_C_LINE + line
			}

			if _, err := outBuf.WriteString(line); err != nil {
				return err
			}
			continue
		}

		if isDefineBlock && line == "\n" {
			if _, err := outBuf.WriteString(")\n\n"); err != nil {
				return err
			}
			isDefineBlock = false
			continue
		}

		// === Translate structs.
		if !isStruct {
			if sub := reStruct.FindStringSubmatch(line); sub != nil {
				isStruct = true

				if isDefineBlock {
					if _, err := outBuf.WriteString(")\n"); err != nil {
						return err
					}
					isDefineBlock = false
				}

				if _, err := outBuf.WriteString(fmt.Sprintf(
					"type %s struct {\n", strings.Title(sub[2]))); err != nil {
					return err
				}
				continue
			}
		} else {
			if sub := reStructField.FindStringSubmatch(line); sub != nil {
				// Translate the field type.
				gotype, ok := ctypeTogo(sub[1], &extraType)

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

				if _, err := outBuf.WriteString(line); err != nil {
					return err
				}
				continue
			}
			if strings.HasPrefix(line, "}") {
				if _, err := outBuf.WriteString(strings.Replace(line, ";", "", 1)); err != nil {
					return err
				}
				isStruct = false
				continue
			}
		}

		// Comment another C lines.
		//if line != "\n" && !strings.HasPrefix(line, "//") {
		if !reSkip.MatchString(line) {
			line = COMMENT_C_LINE + line
		}

		if _, err := outBuf.WriteString(line); err != nil {
			return err
		}
	}

	if err := outBuf.Flush(); err != nil {
		return err
	}
	return nil
}

// Translates a C type definition into Go definition. The C header could have
// defined new types so they're checked in the firs place.
func ctypeTogo(ctype string, extraCtype *vector.StringVector) (gotype string, ok bool) {
	for _, v := range *extraCtype {
		if v == ctype {
			return ctype, true
		}
	}

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
		os.Exit(1)
	}

	// === Update header
	cmd := strings.Join(os.Args, " ")
	goHeader = strings.Replace(goHeader, "{cmd}", path.Base(cmd), 1)
	goHeader = strings.Replace(goHeader, "{pkg}", *pkgName, 1)

	// Translate all headers passed in command line.
	for _, file := range flag.Args() {
		if err := translateC(file); err != nil {
			fmt.Fprintf(os.Stderr, err.String())
			os.Exit(1)
		}
	}

//	File := "../test/header.h" //!!!
}


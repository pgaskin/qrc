// +build ignored

package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const QtVer = "5.13"
const QtSrc = "https://raw.githubusercontent.com/qt/qtbase/" + QtVer + "/"

func main() {
	if err := generate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: generate locale_generated.go: %v\n", err)
		os.Exit(1)
		return
	}
}

func generate() error {
	resp, err := http.Get(QtSrc + "src/corelib/tools/qlocale.h")
	if err != nil {
		return fmt.Errorf("get qlocale.h: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get qlocale.h: response status %s", resp.Status)
	}

	language, country, err := parse(resp.Body)
	if err != nil {
		return fmt.Errorf("parse qlocale.h: %w", err)
	}

	//

	f, err := ioutil.TempFile(".", "locale_generate")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer f.Close()
	defer os.Remove(f.Name())

	if _, err := fmt.Fprintf(f, "// Code generated by locale_generate.go from Qt %s. DO NOT EDIT.\npackage qrc\n", QtVer); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(f, "\n// Language is a language supported by Qt (note: multiple names can have the\n// same code).\n"); err != nil {
		return err
	}
	if err := language.GenerateGo(f, "Language", "uint16"); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(f, "\n// Country is a country supported by Qt (note: multiple names can have the same\n// code).\n"); err != nil {
		return err
	}
	if err := country.GenerateGo(f, "Country", "uint16"); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(f.Name(), "locale_generated.go"); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	//

	tf, err := ioutil.TempFile(".", "locale_generate")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer tf.Close()
	defer os.Remove(tf.Name())

	if _, err := fmt.Fprintf(tf, "// Code generated by locale_generate.go from Qt %s. DO NOT EDIT.\npackage qrc\n", QtVer); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(tf, "\nimport %#v\n", "testing"); err != nil {
		return err
	}

	if err := language.GenerateGoTest(tf, "Language"); err != nil {
		return err
	}

	if err := country.GenerateGoTest(tf, "Country"); err != nil {
		return err
	}

	if err := tf.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tf.Name(), "locale_generated_test.go"); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func parse(r io.Reader) (language *Enum, country *Enum, err error) {
	type State int

	const (
		StateDefault State = iota
		StateLanguage
		StateCountry
	)

	var p struct {
		State
		Line              string
		Language, Country *Enum
	}

	s := bufio.NewScanner(r)

	for s.Scan() {
		p.Line = strings.Join(strings.Fields(strings.TrimSpace(s.Text())), " ")
		switch p.State {
		case StateDefault:
			switch p.Line {
			case "enum Language {":
				if p.Language != nil {
					return nil, nil, fmt.Errorf("parse qlocale.h: parse languages: already seen")
				}
				p.Language = NewEnum()
				p.State = StateLanguage
			case "enum Country {":
				if p.Country != nil {
					return nil, nil, fmt.Errorf("parse qlocale.h: parse countries: already seen")
				}
				p.Country = NewEnum()
				p.State = StateCountry
			default:
				p.State = StateDefault
			}
		case StateLanguage:
			switch p.Line {
			case "":
				p.State = StateLanguage
			case "};":
				p.State = StateDefault
			default:
				if err := p.Language.ParseC(p.Line); err != nil {
					return nil, nil, fmt.Errorf("parse qlocale.h: parse languages: %w", err)
				}
				p.State = StateLanguage
			}
		case StateCountry:
			switch p.Line {
			case "":
				p.State = StateCountry
			case "};":
				p.State = StateDefault
			default:
				if err := p.Country.ParseC(p.Line); err != nil {
					return nil, nil, fmt.Errorf("parse qlocale.h: parse languages: %w", err)
				}
				p.State = StateCountry
			}
		}
	}

	if err := s.Err(); err != nil {
		return nil, nil, err
	}

	if p.State != StateDefault {
		return nil, nil, fmt.Errorf("parse qlocale.h: unexpected EOF at state %d (last line: %q)", p.State, p.Line)
	}

	if l, c := p.Language != nil, p.Country != nil; !l || !c {
		return nil, nil, fmt.Errorf("parse qlocale.h: missing language (found: %t) or country (found: %t)", l, c)
	}

	return p.Language, p.Country, nil
}

type Enum struct {
	Value map[string]int
	Name  map[int]string
}

func NewEnum() *Enum {
	return &Enum{
		Value: map[string]int{},
		Name:  map[int]string{},
	}
}

func (e *Enum) ParseC(line string) error {
	spl1 := strings.Split(line, " = ")
	if len(spl1) != 2 {
		return fmt.Errorf("line %q: expected one %q", line, " = ")
	}

	spl2 := strings.Split(spl1[1], ",")
	if len(spl2) > 2 {
		return fmt.Errorf("line %q: expected at most one %q", line, ",")
	}

	a, b := spl1[0], spl2[0]

	for _, c := range a {
		if !unicode.IsLetter(c) {
			return fmt.Errorf("line %q: bad language %q", line, a)
		}
	}

	if _, ok := e.Value[a]; ok {
		return fmt.Errorf("line %q: language %q already seen", line, a)
	}

	if v, err := strconv.Atoi(b); err == nil {
		e.Value[a] = v
		if _, ok := e.Name[v]; !ok {
			e.Name[v] = a
		}
	} else if v, ok := e.Value[b]; ok {
		e.Value[a] = v
	} else {
		return fmt.Errorf("line %q: invalid value %q", line, b)
	}

	return nil
}

func (e Enum) GenerateGo(w io.Writer, typeName, typeType string) error {
	var vw int
	var v []string
	for n := range e.Value {
		v = append(v, n)
		if len(n) > vw {
			vw = len(n)
		}
	}
	sort.Strings(v)

	var x []int
	for c := range e.Name {
		x = append(x, c)
	}
	sort.Ints(x)

	if _, err := fmt.Fprintf(w, "type %s %s\n", typeName, typeType); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "\nconst (\n"); err != nil {
		return err
	}
	for _, n := range v {
		if e.Name[e.Value[n]] == n {
			if _, err := fmt.Fprintf(w, "\t%s%*s %s = %#v\n", typeName, -1*vw, n, typeName, e.Value[n]); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "\t%s%*s %s = %s%s\n", typeName, -1*vw, n, typeName, typeName, e.Name[e.Value[n]]); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(w, ")\n"); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "\nfunc (%c %s) String() string {\n\tswitch %c {\n", strings.ToLower(typeName)[0], typeName, strings.ToLower(typeName)[0]); err != nil {
		return err
	}
	for _, c := range x {
		if _, err := fmt.Fprintf(w, "\tcase %#v:\n\t\treturn %#v", c, e.Name[c]); err != nil {
			return err
		}
		var tmp []string
		for _, n := range v {
			if e.Value[n] == c && e.Name[e.Value[n]] != n {
				tmp = append(tmp, n)
			}
		}
		if len(tmp) != 0 {
			if _, err := fmt.Fprintf(w, " // %s\n", strings.Join(tmp, ", ")); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "\n"); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(w, "\t}\n\tpanic(%#v)\n}\n", "no such "+strings.ToLower(typeName)); err != nil {
		return err
	}

	return nil
}

func (e Enum) GenerateGoTest(w io.Writer, typeName string) error {
	var x []struct {
		C int
		N string
		X string
	}
	for n, c := range e.Value { // not e.Name since we want to test that aliases map to the correct names
		x = append(x, struct {
			C int
			N string
			X string
		}{c, n, e.Name[c]})
	}
	sort.Slice(x, func(i, j int) bool {
		return x[i].C < x[j].C
	})

	if _, err := fmt.Fprintf(w, "\nfunc Test%s(t *testing.T) {\n", typeName); err != nil {
		return err
	}
	for _, c := range x {
		if _, err := fmt.Fprintf(w, "\tif %s%s != %#v || %s%s.String() != %#v {\n\t\tt.Errorf(%#v, %#v)\n\t}\n", typeName, c.N, c.C, typeName, c.N, c.X, "%q incorrect", c.N); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "}\n"); err != nil {
		return err
	}
	return nil
}

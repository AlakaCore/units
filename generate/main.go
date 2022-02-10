package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strings"
	"time"
)

func title(x string) string {
	tmp := strings.ReplaceAll(x, "-", " ")
	components := strings.Split(tmp, " ")
	result := ""
	for _, component := range components {
		result = result + strings.Title(component)
	}
	return result
}

func array(x []string, quote bool) string {
	result := ""
	for _, s := range x {
		format := "%s,"
		if quote {
			format = "\"%s\","
		}
		result += fmt.Sprintf(format, s)
	}
	return strings.Trim(result, ",")
}

func fn(structName, fnName, code, returns string, comment string, args ...string) string {
	codeString := ""
	for _, line := range strings.Split(code, "\n") {
		codeString += fmt.Sprintf("    %s\n", line)
	}

	arguments := array(args, false)
	return fmt.Sprintf(`// %s %s
func (x %s) %s(%s) %s {
%s}`, fnName, comment, structName, fnName, arguments, returns, codeString)
}

func getter(structName, fnName, accessor, returns string) string {
	return fn(structName, fnName, fmt.Sprintf("return x.%s", accessor), returns, fmt.Sprintf("gets the %s field", accessor))
}

func conversionComponents(converter string) []string {
	components := strings.Split(converter, "=>")
	for idx, c := range components {
		components[idx] = strings.TrimSpace(strings.ReplaceAll(c, ",", ""))
	}
	return components
}

type Unit struct {
	Name     string   `yaml:"name"`
	Symbol   string   `yaml:"symbol"`
	FromBase string   `yaml:"fromBase"`
	ToBase   string   `yaml:"toBase"`
	Matches  []string `yaml:"matches"`
}

func (u *Unit) StructName() string {
	return title(u.Name)
}

type Definition struct {
	Type     string   `yaml:"type"`
	BaseUnit string   `yaml:"baseUnit"`
	Matches  []string `yaml:"matches"`
	Units    []Unit   `yaml:"units"`
	Base     Unit
}

func (d *Definition) StructName() string {
	return title(d.Type)
}

type UnitsYaml struct {
	Version     string       `yaml:"version"`
	Definitions []Definition `yaml:"definitions"`
}

func appendText(spaces int, to, format string, args ...interface{}) string {
	next := fmt.Sprintf(format, args...)
	f := "%s"
	for i := 0; i < spaces; i++ {
		f += "\n"
	}
	f += "%s"
	return fmt.Sprintf(f, to, next)
}

func appends(to, format string, args ...interface{}) string {
	return appendText(2, to, format, args...)
}

func (u *Unit) MakeGoCode(def *Definition) string {
	block := ""
	name := u.StructName()

	longest := len(u.FromBase)
	if len(u.ToBase) > longest {
		longest = len(u.ToBase)
	}

	block = appendText(0, block, `// %s (Unit)
// UnitType     : %s
// UnitType.Base: %s
// Unit.FromBase: %-`+fmt.Sprintf("%d", longest)+`s = %s
// Unit.ToBase  : %-`+fmt.Sprintf("%d", longest)+`s = %s`,
		name, def.StructName(), def.Base.StructName(), u.FromBase, u.Symbol, u.ToBase, def.Base.Symbol)

	block = appendText(1, block, `type %s struct {
	name     string
	symbol   string
	fromBase string
	toBase   string
	matches  []string
	typeOf   %s
}`, name, def.StructName())

	block = appends(block, getter(u.StructName(), "Name", "name", "string"))
	block = appends(block, getter(u.StructName(), "Symbol", "symbol", "string"))
	fromComponents := conversionComponents(u.FromBase)
	toComponents := conversionComponents(u.ToBase)
	block = appends(block, fn(
		u.StructName(),
		"FromBase",
		fmt.Sprintf("return %s", fromComponents[1]),
		"float64",
		fmt.Sprintf("converts %s to %s", def.Base.Symbol, u.Symbol),
		fmt.Sprintf("%s float64", fromComponents[0]),
	))
	block = appends(block, fn(
		u.StructName(),
		"ToBase",
		fmt.Sprintf("return %s", toComponents[1]),
		"float64",
		fmt.Sprintf("converts %s to %s", def.Base.Symbol, u.Symbol),
		fmt.Sprintf("%s float64", toComponents[0]),
	))
	block = appends(block, getter(u.StructName(), "MatchList", "matches", "[]string"))
	block = appends(block, fn(name, "Matches", `for _, m := range x.matches {
	if m == check {
		return true
	}
}
return false`, "bool", `returns true if check matches our possible names.
// Helpful when a user is allowed to enter in unit types
// freehand, for example.`, "check string"))
	block = appends(block, getter(u.StructName(), "TypeOf", "typeOf", "UnitType"))
	block = appends(block, fn(
		u.StructName(),
		"Base",
		fmt.Sprintf("return %sUnit", def.Base.StructName()),
		"Unit",
		"returns the base unit"))

	block = appends(block, `var %sUnit = %s {
	name:     "%s",
	symbol:   "%s",
	fromBase: "%s",
	toBase:   "%s",
	matches:  []string{%s},
	typeOf:   %sUnitType,
}`,
		u.StructName(),
		u.StructName(),
		u.Name,
		u.Symbol,
		u.FromBase,
		u.ToBase,
		array(u.Matches, true),
		def.StructName())

	return block
}

func (d *Definition) MakeGoCode() string {
	block := ""
	name := d.StructName()

	block = appendText(0, block, `// %s (UnitType)
// Contains %d units:`, name, len(d.Units))

	var unitNames []string
	longestStruct := 0
	longestFrom := 0
	for _, u := range d.Units {
		if u.Name == d.BaseUnit {
			d.Base = u
		}
		if len(u.StructName()) > longestStruct {
			longestStruct = len(u.StructName())
		}
		if len(u.FromBase) > longestFrom {
			longestFrom = len(u.FromBase)
		}
	}

	for _, u := range d.Units {
		block = appendText(1,
			block,
			`//  - %-`+fmt.Sprintf("%d", longestStruct)+
				`s %-`+
				fmt.Sprintf("%d", longestFrom)+
				`s = %s`,
			u.StructName(),
			u.FromBase,
			u.Symbol,
		)
		unitNames = append(unitNames, u.StructName()+"Unit")
	}
	block = appendText(1, block, `// Base: %s`, d.Base.StructName())

	block = appendText(1, block, `type %s struct {
	name    string
	base    Unit
	matches []string
	units   []Unit
}`, name)

	block = appends(block, getter(name, "Name", "name", "string"))
	block = appends(block, getter(name, "Base", "base", "Unit"))
	block = appends(block, getter(name, "Units", "units", "[]Unit"))
	block = appends(block, fn(name, "UnitList", `var list []string
for _, u := range x.units {
	list = append(list, u.Name())
}
return list`, "[]string", "returns the list of units as strings"))
	block = appends(block, getter(name, "MatchList", "matches", "[]string"))
	block = appends(block, fn(name, "Matches", `for _, m := range x.matches {
	if m == check {
		return true
	}
}
return false`, "bool", `returns true if check matches our possible names.
// Helpful when a user is allowed to enter in unit types
// freehand, for example.`, "check string"))

	matches := array(d.Matches, true)
	units := array(unitNames, false)
	block = appends(block, `var %sUnitType = %s{
	name: "%s",
	base: %sUnit,
	matches: []string{%s},
	units: []Unit{%s},
}`, d.StructName(), d.StructName(), d.Type, d.Base.StructName(), matches, units)

	for _, u := range d.Units {
		block = appends(block, u.MakeGoCode(d))
	}

	return block
}

func (uy *UnitsYaml) MakeGoFile() []byte {
	file := "package units"

	file = appends(file, `// File autogenerated on %s.
// Do not edit directly`, time.Now().String())

	// Add the primary interfaces
	file = appends(file, `// Unit represents a scalar type of unit which can be converted to and from a base 
type Unit interface {
	Name() string
	Symbol() string
	FromBase(float64) float64
	ToBase(float64) float64
	MatchList() []string
	Matches(string) bool
	TypeOf() UnitType
	Base() Unit
}

// UnitType represents a collection of related units
type UnitType interface {
	Name() string
	Base() Unit
	Units() []Unit
	UnitList() []string
	MatchList() []string
	Matches(string) bool
}`)

	// Store the unit types in a map
	file = appends(file, `var UnitMap = make(map[string]Unit)
var TypeMap = make(map[string]UnitType)`)

	for _, d := range uy.Definitions {
		file = appends(file, d.MakeGoCode())
	}

	return []byte(file)
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	f, err := os.ReadFile(cwd + "/units.yaml")
	if err != nil {
		panic(err)
	}

	data := UnitsYaml{}
	err = yaml.Unmarshal(f, &data)
	if err != nil {
		panic(err)
	}

	goFile := data.MakeGoFile()

	if err := os.WriteFile(cwd+"/units.go", goFile, 0644); err != nil {
		log.Panic(err)
	}

	os.Exit(0)
}

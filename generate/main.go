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

func arraySep(x []string, quote bool, sep string) string {
	result := ""
	for idx, s := range x {
		format := "%s,"
		if quote {
			format = "\"%s\","
		}

		if idx < len(x)-1 {
			format += "%s"
			result += fmt.Sprintf(format, s, sep)
		} else {
			result += fmt.Sprintf(format, s)
		}
	}
	return result
}

func anonFn(fnName, code, returns string, comment string, args ...string) string {
	codeString := ""
	for _, line := range strings.Split(code, "\n") {
		codeString += fmt.Sprintf("    %s\n", line)
	}

	arguments := array(args, false)
	return fmt.Sprintf(`// %s %s
func %s(%s) %s {
%s}`, fnName, comment, fnName, arguments, returns, codeString)
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

func getter(structName, fnName, value, returns string, quote bool) string {
	if quote {
		value = fmt.Sprintf("\"%s\"", value)
	}
	return fn(structName, fnName, fmt.Sprintf("return %s", value), returns, fmt.Sprintf("always returns %s", value))
}

func conversionComponents(converter string) []string {
	components := strings.Split(converter, "=>")
	for idx, c := range components {
		components[idx] = strings.TrimSpace(strings.ReplaceAll(c, ",", ""))
	}
	return components
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

var matchCode = `check = SanitizeString(check)
for _, m := range x.MatchList() {
	if m == check || m == "*" {
		return true
	}
}
return false`

type Unit struct {
	Name     string   `yaml:"name"`
	Symbol   string   `yaml:"symbol"`
	FromBase string   `yaml:"fromBase"`
	ToBase   string   `yaml:"toBase"`
	Matches  []string `yaml:"matches"`
}

func (u *Unit) Title() string {
	return title(u.Name)
}

func (u *Unit) StructName(defName string) string {
	uname := u.Title()
	if uname == defName {
		uname = "_" + uname
	}
	uname += defName
	return uname
}

func (u *Unit) VarName(defName string) string {
	return u.StructName(defName) + "Unit"
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

func (d *Definition) VarName() string {
	return d.StructName() + "UnitType"
}

type UnitsYaml struct {
	Version     string       `yaml:"version"`
	Definitions []Definition `yaml:"definitions"`
}

func (u *Unit) MakeGoCode(def *Definition) string {
	block := ""
	name := u.StructName(def.StructName())

	longest := len(u.FromBase)
	if len(u.ToBase) > longest {
		longest = len(u.ToBase)
	}

	block = appendText(0, block, `// %s (Unit)
// UnitType     : %s
// UnitType.Base: %s
// Unit.FromBase: %-`+fmt.Sprintf("%d", longest)+`s = %s
// Unit.ToBase  : %-`+fmt.Sprintf("%d", longest)+`s = %s`,
		name, def.StructName(), def.Base.StructName(def.StructName()), u.FromBase, u.Symbol, u.ToBase, def.Base.Symbol)

	block = appendText(1, block, `type %s %s`, name, def.StructName())
	block = appends(block, getter(name, "Title", u.Title(), "string", true))
	block = appends(block, getter(name, "Name", u.Name, "string", true))
	block = appends(block, getter(name, "Symbol", u.Symbol, "string", true))
	fromComponents := conversionComponents(u.FromBase)
	toComponents := conversionComponents(u.ToBase)
	block = appends(block, fn(
		name,
		"FromBase",
		fmt.Sprintf("return %s", fromComponents[1]),
		"float64",
		fmt.Sprintf("converts %s to %s", def.Base.Symbol, u.Symbol),
		fmt.Sprintf("%s float64", fromComponents[0]),
	))
	block = appends(block, fn(
		name,
		"ToBase",
		fmt.Sprintf("return %s", toComponents[1]),
		"float64",
		fmt.Sprintf("converts %s to %s", u.Symbol, def.Base.Symbol),
		fmt.Sprintf("%s float64", toComponents[0]),
	))

	matches := array(u.Matches, true)
	block = appends(block, `// %sMatchList is effectively a constant
var %sMatchList = [...]string {%s}`, name, name, matches)
	block = appends(block, getter(name, "MatchList", fmt.Sprintf(`%sMatchList[:]`, name), "[]string", false))

	block = appends(block, fn(name, "Matches", matchCode, "bool", `returns true if check matches our possible names.
// Helpful when a user is allowed to enter in unit types
// freehand, for example.`, "check string"))

	block = appends(block, getter(name, "TypeOf", def.VarName(), "UnitType", false))
	block = appends(block, getter(name, "Base", def.Base.VarName(def.StructName()), "Unit", false))

	block = appends(block, `var %s %s = 0.0`, u.VarName(def.StructName()), name)
	return block
}

func (d *Definition) MakeGoCode() string {
	block := ""
	name := d.StructName()

	block = appendText(0, block, `// %s (UnitType)
// Contains %d units:`, name, len(d.Units))

	var unitNames []string
	var unitVars []string
	longestStruct := 0
	longestFrom := 0
	for _, u := range d.Units {
		if u.Name == d.BaseUnit {
			d.Base = u
		}
		if len(u.StructName(name)) > longestStruct {
			longestStruct = len(u.StructName(name))
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
			u.StructName(name),
			u.FromBase,
			u.Symbol,
		)
		unitNames = append(unitNames, u.Name)
		unitVars = append(unitVars, u.VarName(name))
	}
	block = appendText(1, block, `// Base: %s`, d.Base.StructName(name))

	block = appendText(1, block, `type %s float64`, name)

	matches := array(d.Matches, true)
	units := array(unitNames, true)
	uVars := array(unitVars, false)

	block = appends(block, getter(name, "Title", name, "string", true))
	block = appends(block, getter(name, "Name", d.Type, "string", true))
	block = appends(block, getter(name, "Base", d.Base.VarName(d.StructName()), "Unit", false))

	block = appends(block, `// %sUnits is effectively a constant
var %sUnits = [...]Unit {%s}`, name, name, uVars)
	block = appends(block, getter(name, "Units", fmt.Sprintf("%sUnits[:]", name), "[]Unit", false))

	block = appends(block, `// %sUnitList is effectively a constant
var %sUnitList = [...]string {%s}`, name, name, units)
	block = appends(block, getter(name, "UnitList", fmt.Sprintf(`%sUnitList[:]`, name), "[]string", false))

	block = appends(block, `// %sMatchList is effectively a constant
var %sMatchList = [...]string {%s}`, name, name, matches)
	block = appends(block, getter(name, "MatchList", fmt.Sprintf(`%sMatchList[:]`, name), "[]string", false))

	block = appends(block, fn(name, "Matches", matchCode, "bool", `returns true if check matches our possible names.
// Helpful when a user is allowed to enter in unit types
// freehand, for example.`, "check string"))

	block = appends(block, `var %s %s = 0.0`, d.VarName(), name)

	for _, u := range d.Units {
		block = appends(block, u.MakeGoCode(d))
	}

	return block
}

func (uy *UnitsYaml) MakeGoFile() []byte {
	file := `// Package units provides a standard way of working with unit for
// Alaka and Alakans alike. It's automatically generated via a
// .yaml file with a format that makes it really easy to add new
// units. Because we use code generation, we can provide functions
// that are super fast by using explicit values without the work
// of hand copying hundreds of methods across a bunch of permutations
// of the same thing.
//
// All the primary UnitTypes and Units of this package are built
// directly on the float64 construct. This allows go users to treat
// scalars as the Unit or UnitType that they actually represent,
// including the ability to use those type definitions as guards in
// functions that depend on a particular Unit or UnitType. Eg.:
//
// func AddPressure (p1, p2 PascalsPressure) PascalsPressure {
//     returns p1 + p2
// }
package units

import (
    "regexp"
    "strings"
)`

	file = appends(file, `// File autogenerated on %s.
// Do not edit directly`, time.Now().String())

	// Add the primary interfaces
	file = appends(file, `// Unit represents a scalar type of unit which can be converted to and from a base 
type Unit interface {
	// Title is used for code interfaces
	Title() string
	// Name is used for displays
	Name() string
	// Symbol is the symbol of the unit and can be displayed beside scalars
	Symbol() string
	// FromBase converts the given number of the unit type base to this unit
	FromBase(float64) float64
	// ToBase converts the given number of this unit type to the base unit
	ToBase(float64) float64
	// MatchList is a list of matching strings which should represent this unit in userland
	MatchList() []string
	// Matches compares a string to a switch of all possible matches
	Matches(string) bool
	// TypeOf returns the UnitType of this unit. You can access the BaseUnit from there
	TypeOf() UnitType
	// Base returns the base Unit of this UnitType directly
	Base() Unit
}

// UnitType represents a collection of related units
type UnitType interface {
	// Title is used for code interfaces
	Title() string
	// Name is used for displays
	Name() string
	// Base returns the primary unit of this unit type that is stored in Alaka.
	// Most of the time this is an SI unit, but not always (temperature is C,
	// not K, for example)
	Base() Unit
	// Units returns all the supported units of this unit type
	Units() []Unit
	// UnitList returns all the supported units of this unit type as strings
	UnitList() []string
	// MatchList is a list of matching strings which should represent this unit type in userland
	MatchList() []string
	// Matches compares a string to a switch of all possible matches
	Matches(string) bool
}`)

	// Utility functions
	file = appends(file, "var WhitespaceRegex = regexp.MustCompile(`\\s`)")
	file = appends(file, anonFn("SanitizeString", `out := strings.ToLower(input)
out = WhitespaceRegex.ReplaceAllString(out, "")
return out`, "string", "removes whitespace and lower cases the string", "input string"))

	file = appends(file, anonFn(
		"AlakaTitle",
		`return ut.Title() + "_" + u.Title()`,
		"string",
		"returns the Alaka string representing this particular unit and unit type combo",
		"ut UnitType, u Unit"))

	var allTypes []string
	var allUnits []string
	var allUnitTypes []string
	// Provide a function for getting a unit and/or unit type
	getTypeCode := `switch input {`
	getUnitCode := `search := typeOf.Title() + "->" + SanitizeString(input)
switch search {`
	getTypeUnitCode := `switch input {`

	numberName := ""
	numberUnitName := ""

	longestDName := 0
	for _, d := range uy.Definitions {
		if len(d.StructName()) > longestDName {
			longestDName = len(d.StructName())
		}
	}

	for _, d := range uy.Definitions {
		if d.Type == "Number" {
			numberName = d.VarName()
		}

		allTypes = append(allTypes, d.StructName())

		for _, match := range d.Matches {
			getTypeCode = appendText(1, getTypeCode, `case "%s":
  return %s`, match, d.VarName())
		}

		unitMapWhitespace := " "
		for i := 0; i < longestDName-len(d.StructName()); i++ {
			unitMapWhitespace += " "
		}

		unitMap := fmt.Sprintf(`    "%s":%s{`, d.StructName(), unitMapWhitespace)
		longest := 0
		var unitNames []string

		for _, u := range d.Units {
			if u.Name == "Number" {
				numberUnitName = u.VarName(d.StructName())
			}

			if len(u.Title()) > longest {
				longest = len(u.Title())
			}
			unitNames = append(unitNames, u.Title())
			allUnitTypes = append(allUnitTypes, d.StructName()+"_"+u.Title())

			for _, match := range u.Matches {
				getUnitCode = appendText(1, getUnitCode, `case "%s":
  return %s`, d.StructName()+"->"+match, u.VarName(d.StructName()))
			}

			getTypeUnitCode = appendText(1, getTypeUnitCode, `case "%s":
  return %s`, d.StructName()+"_"+u.Title(), fmt.Sprintf(`%s, %s`, d.VarName(), u.VarName(d.StructName())))
		}

		unitMap = fmt.Sprintf(`%s%s}`, unitMap, array(unitNames, true))
		allUnits = append(allUnits, unitMap)

	}
	getTypeCode = appendText(1, getTypeCode, `default:
  return %s
}`, numberName)
	getUnitCode = appendText(1, getUnitCode, `default:
  return %s
}`, numberUnitName)
	getTypeUnitCode = appendText(1, getTypeUnitCode, `default:
  return %s, %s
}`, numberName, numberUnitName)

	file = appends(file, `// AllTypes is a list of all available types below
var AllTypes = [...]string{
    %s
}`, arraySep(allTypes, true, "\n    "))
	file = appends(file, `// AllUnits is a map of unit type -> units
var AllUnits = map[string][]string{
%s
}`, arraySep(allUnits, false, "\n"))
	//file = appends(file, `// AllUnits is a list of all available units below
	//var AllTypes = [...]UnitType{%s}`, array(allTypes, false))
	file = appends(file, `// AllUnitTypes is a list of all available Unit and Type combos below
// AKA the list of all possible output combinations of AlakaTitle
var AllUnitTypes = [...]string{
    %s
}`, arraySep(allUnitTypes, true, "\n    "))

	file = appends(file, anonFn(
		"GetType",
		getTypeCode,
		"UnitType",
		fmt.Sprintf("returns the unit type which matches input or %s", numberName),
		"input string"))
	file = appends(file, anonFn(
		"GetUnit",
		getUnitCode,
		"Unit",
		fmt.Sprintf("returns the unit which matches input or %s", numberUnitName),
		"input string, typeOf UnitType"))
	file = appends(file, anonFn(
		"GetTypeUnit",
		getTypeUnitCode,
		"(UnitType, Unit)",
		fmt.Sprintf(`returns the unit type and unit which matches input or (%s, %s).
// Opposite of AlakaTitle`, numberName, numberUnitName),
		"input string"))

	for _, d := range uy.Definitions {
		file = appends(file, d.MakeGoCode())
	}

	// It's a pita to get % and use a lot of sprintf so we do that last replace here
	file = strings.ReplaceAll(file, "percentagesymbol", "%")

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

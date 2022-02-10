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

type Unit struct {
	Name     string   `yaml:"name"`
	Symbol   string   `yaml:"symbol"`
	FromBase string   `yaml:"fromBase"`
	ToBase   string   `yaml:"toBase"`
	Matches  []string `yaml:"matches"`
}

func (u *Unit) StructName(defName string) string {
	uname := title(u.Name)
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

	block = appendText(1, block, `type %s struct {
	name     string
	symbol   string
	fromBase string
	toBase   string
	matches  []string
	typeOf   %s
}`, name, def.StructName())

	block = appends(block, getter(name, "Name", "name", "string"))
	block = appends(block, getter(name, "Symbol", "symbol", "string"))
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
		fmt.Sprintf("converts %s to %s", def.Base.Symbol, u.Symbol),
		fmt.Sprintf("%s float64", toComponents[0]),
	))
	block = appends(block, getter(name, "MatchList", "matches", "[]string"))
	block = appends(block, fn(name, "Matches", `for _, m := range x.matches {
	if m == check || m == "*" {
		return true
	}
}
return false`, "bool", `returns true if check matches our possible names.
// Helpful when a user is allowed to enter in unit types
// freehand, for example.`, "check string"))
	block = appends(block, getter(name, "TypeOf", "typeOf", "UnitType"))
	block = appends(block, fn(
		name,
		"Base",
		fmt.Sprintf("return %sUnit", def.Base.StructName(def.StructName())),
		"Unit",
		"returns the base unit"))

	block = appends(block, `var %s = %s {
	name:     "%s",
	symbol:   "%s",
	fromBase: "%s",
	toBase:   "%s",
	matches:  []string{%s},
	typeOf:   %sUnitType,
}`,
		u.VarName(def.StructName()),
		name,
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
		unitNames = append(unitNames, u.VarName(name))
	}
	block = appendText(1, block, `// Base: %s`, d.Base.StructName(name))

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
	if m == check || m == "*" {
		return true
	}
}
return false`, "bool", `returns true if check matches our possible names.
// Helpful when a user is allowed to enter in unit types
// freehand, for example.`, "check string"))

	matches := array(d.Matches, true)
	units := array(unitNames, false)
	block = appends(block, `var %s = %s{
	name: "%s",
	base: %s,
	matches: []string{%s},
	units: []Unit{%s},
}`, d.VarName(), d.StructName(), d.Type, d.Base.VarName(name), matches, units)

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

	// Store the unit types in a map
	file = appends(file, `var UnitMap = make(map[string]Unit)
var TypeMap = make(map[string]UnitType)`)

	// Provide a function for getting a unit and/or unit type
	getTypeCode := `switch input {`
	getUnitCode := `search := typeOf.Title() + "->" + input

switch search {`
	getTypeUnitCode := `switch input {`

	numberName := ""
	numberUnitName := ""
	for _, d := range uy.Definitions {
		if d.Type == "Number" {
			numberName = d.VarName()
		}

		for _, match := range d.Matches {
			getTypeCode = appendText(1, getTypeCode, `case "%s":
  return %s`, match, d.VarName())
		}

		for _, u := range d.Units {
			if u.Name == "Number" {
				numberUnitName = u.VarName(d.StructName())
			}

			for _, match := range u.Matches {
				getUnitCode = appendText(1, getUnitCode, `case "%s":
  return %s`, d.StructName()+"->"+match, u.VarName(d.StructName()))
			}

			getTypeUnitCode = appendText(1, getTypeUnitCode, `case "%s":
  return %s`, strings.ReplaceAll(d.Type+"_"+u.Name, " ", ""), fmt.Sprintf(`%s, %s`, d.VarName(), u.VarName(d.StructName())))
		}

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
		fmt.Sprintf("returns the unit type and unit which matches input or (%s, %s)", numberName, numberUnitName),
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

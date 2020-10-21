package cli

import (
	"flag"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	boolType    = reflect.TypeOf((*bool)(nil))
	float64Type = reflect.TypeOf((*float64)(nil))
	intType     = reflect.TypeOf((*int)(nil))
	int64Type   = reflect.TypeOf((*int64)(nil))
	stringType  = reflect.TypeOf((*string)(nil))
	uintType    = reflect.TypeOf((*uint)(nil))
	uint64Type  = reflect.TypeOf((*uint64)(nil))
)

// ParseFlags parses flags into the given struct using a very simple
// mapping. It loops over exported fields in the struct that have a
// "flag" tag and applies the following rules:
//
// If the field's type implements flag.Value, flag.Var is called to
// parse it.
//
// If the field is of a kind corresponding to the various typed
// parsing functions in the flag package, such as float64, string, or
// int, the appropriate function is called to parse it.
//
// In either of these two cases, the arguments passed to the parsing
// function for that field are set via a comma separated list in the
// "flag" tag. There are a few special cases, however:
//
// If the first element of the comma-separated list in the tag is a
// number, that number is assumed to correspond to the index of an
// extra argument as returned by flag.Arg(n). An optional second
// element is used as a default value.
func ParseFlags(flags interface{}, usage func()) error {
	type argFlag struct {
		field reflect.StructField
		tag   string
		v     reflect.Value
		n     int
		parts []string
	}

	v := reflect.ValueOf(flags).Elem()
	t := v.Type()

	args := make([]argFlag, 0, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		tag, ok := field.Tag.Lookup("flag")
		if !ok {
			continue
		}

		fv := v.Field(i)

		parts := strings.SplitN(tag, ",", 2)
		if len(parts) == 0 {
			panic(fmt.Errorf("invalid tag on field %q: %q", field.Name, tag))
		}
		if n, err := strconv.ParseInt(parts[0], 10, 0); err == nil {
			args = append(args, argFlag{
				field: field,
				tag:   tag,
				v:     fv,
				n:     int(n),
				parts: parts,
			})
			continue
		}

		if val, ok := fv.Interface().(flag.Value); ok {
			flag.Var(val, parts[0], parts[1])
			continue
		}
		if val, ok := fv.Addr().Interface().(flag.Value); ok {
			flag.Var(val, parts[0], parts[1])
			continue
		}

		parts = strings.SplitN(tag, ",", 3)
		switch field.Type.Kind() {
		case reflect.Bool:
			d, err := strconv.ParseBool(parts[1])
			if err != nil {
				panic(fmt.Errorf("parse default from %q for %q: %w", tag, field.Name, err))
			}
			flag.BoolVar(fv.Addr().Convert(boolType).Interface().(*bool), parts[0], d, parts[2])

		case reflect.Float64:
			d, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				panic(fmt.Errorf("parse default from %q for %q: %w", tag, field.Name, err))
			}
			flag.Float64Var(fv.Addr().Convert(float64Type).Interface().(*float64), parts[0], d, parts[2])

		case reflect.Int:
			d, err := strconv.ParseInt(parts[1], 10, 0)
			if err != nil {
				panic(fmt.Errorf("parse default from %q for %q: %w", tag, field.Name, err))
			}
			flag.IntVar(fv.Addr().Convert(intType).Interface().(*int), parts[0], int(d), parts[2])

		case reflect.Int64:
			d, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				panic(fmt.Errorf("parse default from %q for %q: %w", tag, field.Name, err))
			}
			flag.Int64Var(fv.Addr().Convert(int64Type).Interface().(*int64), parts[0], d, parts[2])

		case reflect.String:
			flag.StringVar(fv.Addr().Convert(stringType).Interface().(*string), parts[0], parts[1], parts[2])

		case reflect.Uint:
			d, err := strconv.ParseUint(parts[1], 10, 0)
			if err != nil {
				panic(fmt.Errorf("parse default from %q for %q: %w", tag, field.Name, err))
			}
			flag.UintVar(fv.Addr().Convert(uintType).Interface().(*uint), parts[0], uint(d), parts[2])

		case reflect.Uint64:
			d, err := strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				panic(fmt.Errorf("parse default from %q for %q: %w", tag, field.Name, err))
			}
			flag.Uint64Var(fv.Addr().Convert(uint64Type).Interface().(*uint64), parts[0], d, parts[2])

		default:
			panic(fmt.Errorf("unsupported flag type for field %q: %v", field.Name, field.Type))
		}
	}

	flag.Parse()

	for _, arg := range args {
		raw := flag.Arg(arg.n)

		if val, ok := arg.v.Interface().(flag.Value); ok {
			err := val.Set(raw)
			if err != nil {
				return fmt.Errorf("set arg %q: %w", arg.field.Name, err)
			}
			continue
		}
		if val, ok := arg.v.Addr().Interface().(flag.Value); ok {
			err := val.Set(raw)
			if err != nil {
				return fmt.Errorf("set arg %q: %w", arg.field.Name, err)
			}
			continue
		}

		switch arg.field.Type.Kind() {
		case reflect.Bool:
			v, err := strconv.ParseBool(raw)
			if err == nil {
				arg.v.SetBool(v)
				continue
			}
			if len(arg.parts) < 2 {
				return fmt.Errorf("invalid value for arg %q: %q", arg.field.Name, raw)
			}

			d, err := strconv.ParseBool(arg.parts[1])
			if err != nil {
				panic(fmt.Errorf("parse default from %q for %q: %w", arg.tag, arg.field.Name, err))
			}
			arg.v.SetBool(d)

		case reflect.Float64:
			v, err := strconv.ParseFloat(raw, 64)
			if err == nil {
				arg.v.SetFloat(v)
				continue
			}
			if len(arg.parts) < 2 {
				return fmt.Errorf("invalid value for arg %q: %q", arg.field.Name, raw)
			}

			d, err := strconv.ParseFloat(arg.parts[1], 64)
			if err != nil {
				panic(fmt.Errorf("parse default from %q for %q: %w", arg.tag, arg.field.Name, err))
			}
			arg.v.SetFloat(d)

		case reflect.Int, reflect.Int64:
			v, err := strconv.ParseInt(raw, 10, 0)
			if err == nil {
				arg.v.SetInt(v)
				continue
			}
			if len(arg.parts) < 2 {
				return fmt.Errorf("invalid value for arg %q: %q", arg.field.Name, raw)
			}

			d, err := strconv.ParseInt(arg.parts[1], 10, 0)
			if err != nil {
				panic(fmt.Errorf("parse default from %q for %q: %w", arg.tag, arg.field.Name, err))
			}
			arg.v.SetInt(d)

		case reflect.String:
			if raw == "" {
				if len(arg.parts) < 2 {
					return fmt.Errorf("invalid value for arg %q: %q", arg.field.Name, raw)
				}
				raw = arg.parts[1]
			}
			arg.v.SetString(raw)

		case reflect.Uint, reflect.Uint64:
			v, err := strconv.ParseUint(raw, 10, 0)
			if err == nil {
				arg.v.SetUint(v)
				continue
			}
			if len(arg.parts) < 2 {
				return fmt.Errorf("invalid value for arg %q: %q", arg.field.Name, raw)
			}

			d, err := strconv.ParseUint(arg.parts[1], 10, 0)
			if err != nil {
				panic(fmt.Errorf("parse default from %q for %q: %w", arg.tag, arg.field.Name, err))
			}
			arg.v.SetUint(d)

		default:
			panic(fmt.Errorf("unsupported flag type for field %q: %v", arg.field.Name, arg.field.Type))
		}
	}

	return nil
}
